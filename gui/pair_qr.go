package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/skip2/go-qrcode"
)

const qrPairTimeout = 2 * time.Minute

type qrCredentials struct {
	service, password string
}

func newQRCredentials() (qrCredentials, error) {
	// Hex avoids delimiters and escapes in Android's WIFI payload and mDNS name.
	var random [24]byte
	if _, err := rand.Read(random[:]); err != nil {
		return qrCredentials{}, fmt.Errorf("生成二维码凭据失败: %w", err)
	}
	return qrCredentials{"studio-" + hex.EncodeToString(random[:8]), hex.EncodeToString(random[8:])}, nil
}

func (q qrCredentials) payload() string {
	return "WIFI:T:ADB;S:" + q.service + ";P:" + q.password + ";;"
}

func (q qrCredentials) bitmap() (*ui.Bitmap, error) {
	code, err := qrcode.New(q.payload(), qrcode.Medium)
	if err != nil {
		// Encoder errors may contain the payload; never expose credentials.
		return nil, fmt.Errorf("生成配对二维码失败")
	}
	// Integer module scaling preserves crisp edges and the white quiet zone.
	return ui.NewBitmap(code.Image(-6)), nil
}

func (b *backend) pairQR(ctx context.Context, q qrCredentials, interval time.Duration) error {
	if _, err := b.run(ctx, "start-server"); err != nil {
		return err
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		out, err := b.run(ctx, "mdns", "services")
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("mDNS 发现失败，请检查网络或改用配对码: %w", err)
		}
		for _, d := range parseMDNS(out) {
			if d.Pairing && strings.TrimSuffix(d.Name, ".") == q.service {
				// A matching connect service or another phone's pairing service
				// must never receive this session's password.
				if err := ctx.Err(); err != nil {
					return err
				}
				err := b.pairPassword(ctx, d.Address, q.password)
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return err
			}
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// Owned by the UI thread. Credentials are captured only by the worker; the
// application retains the displayed bitmap until completion or cancellation.
type qrPairState struct {
	bitmap  *ui.Bitmap
	running bool
	message string
}

func (a *application) startQRPair() {
	if a.busy || a.b == nil {
		return
	}
	q, err := newQRCredentials()
	var bitmap *ui.Bitmap
	if err == nil {
		bitmap, err = q.bitmap()
	}
	if err != nil {
		a.pairQR = &qrPairState{message: err.Error()}
		return
	}
	a.pairCode = ""
	session := &qrPairState{bitmap: bitmap, running: true, message: "等待扫码，二维码 2 分钟内有效…"}
	a.pairQR = session
	a.work("正在等待手机扫码配对…", func(ctx context.Context) (func(), error) {
		pairCtx, cancel := context.WithTimeout(ctx, qrPairTimeout)
		err := a.b.pairQR(pairCtx, q, time.Second)
		cancel()
		// Pairing has succeeded even if the subsequent connection refresh fails.
		var devices []device
		var note string
		if err == nil {
			var refreshErr error
			devices, note, refreshErr = a.b.discover(ctx, "")
			if refreshErr != nil {
				note = "设备刷新失败，请重新扫描或手动连接。"
			}
		}
		return func() {
			// A dismissed dialog must not be reopened by a queued completion.
			if a.pairQR != session {
				return
			}
			session.running, session.bitmap = false, nil
			if err != nil {
				switch {
				case errors.Is(err, context.DeadlineExceeded):
					session.message = "二维码已过期，请重新生成。若扫码后一直等待，请检查同一 Wi-Fi、VPN 和路由器的 mDNS 限制。"
				case errors.Is(err, context.Canceled):
					session.message = "二维码配对已取消，可重新生成。"
				default:
					session.message = err.Error()
				}
				a.status = "二维码配对未完成"
				a.logs.printf("INFO %s", session.message)
				return
			}
			a.setDevices(mergeDevices(a.devices, devices))
			a.status = "配对成功"
			a.detail = "选择无线调试连接设备后投屏；也可填写手机无线调试主页的 IP:端口。" + note
			a.closePair()
		}, nil
	})
}

func (a *application) cancelQRPair() {
	if a.pairQR != nil && a.pairQR.running && a.cancelJob != nil {
		a.cancelJob()
		a.status = "二维码配对已取消"
	}
	a.pairQR = nil
}

func (a *application) closePair() {
	a.cancelQRPair()
	a.showPair, a.pairCode, a.pairQRMode = false, "", false
}
