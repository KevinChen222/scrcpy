package main

import (
	"context"
	"errors"
	"fmt"
	"image/png"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestQRCredentials(t *testing.T) {
	q, err := newQRCredentials()
	if err != nil {
		t.Fatal(err)
	}
	other, err := newQRCredentials()
	if err != nil || q.service == other.service || q.password == other.password {
		t.Fatal("QR credentials were reused")
	}
	if !strings.HasPrefix(q.service, "studio-") || len(q.password) != 32 || strings.Trim(q.password, "0123456789abcdef") != "" {
		t.Fatal("unsafe QR credentials")
	}
	if q.payload() != "WIFI:T:ADB;S:"+q.service+";P:"+q.password+";;" {
		t.Fatal("invalid Android QR payload")
	}
	bitmap, err := q.bitmap()
	if err != nil {
		t.Fatal(err)
	}
	if w, h := bitmap.Size(); w < 200 || w != h {
		t.Fatal("invalid QR image size")
	}
}

func TestQRPairMatchesOnlyScannedService(t *testing.T) {
	q := qrCredentials{"studio-test", "0123456789abcdef0123456789abcdef"}
	var paired []string
	polls := 0
	b := &backend{run: func(ctx context.Context, args ...string) (string, error) {
		switch args[0] {
		case "start-server":
			return "", nil
		case "mdns":
			polls++
			out := "studio-other _adb-tls-pairing._tcp 192.168.1.2:40000\n" +
				"studio-test _adb-tls-connect._tcp 192.168.1.3:30000\n" +
				"studio-test-extra _adb-tls-pairing._tcp 192.168.1.4:40001\n"
			if polls > 1 {
				out += "studio-test. _adb-tls-pairing._tcp. 192.168.1.3:40002\n"
			}
			return out, nil
		case "pair":
			paired = append([]string(nil), args...)
			return "Successfully paired to " + args[1], nil
		}
		t.Fatalf("unexpected command %v", args)
		return "", nil
	}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := b.pairQR(ctx, q, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(paired, []string{"pair", "192.168.1.3:40002", q.password}) || polls != 2 {
		t.Fatal("paired a mismatched service or used the connect port")
	}
}

func TestQRPairCancellationAndFailures(t *testing.T) {
	for _, mode := range []string{"cancel", "timeout", "mdns", "reject", "pair-error", "start-server"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			q := qrCredentials{"studio-test", "private-qr-password"}
			pairs := 0
			b := &backend{run: func(ctx context.Context, args ...string) (string, error) {
				if args[0] == "start-server" {
					if mode == "start-server" {
						return "", errors.New("start failed")
					}
					return "", nil
				}
				if args[0] == "mdns" {
					switch mode {
					case "mdns":
						return "", errors.New("discovery failed")
					case "cancel":
						cancel()
					case "timeout":
						return "", nil
					}
					return "studio-test _adb-tls-pairing._tcp 192.168.1.3:40000", nil
				}
				pairs++
				if mode == "pair-error" {
					return "", fmt.Errorf("rejected %s", q.password)
				}
				return "rejected " + q.password, nil
			}}
			err := b.pairQR(ctx, q, time.Millisecond)
			if err == nil || strings.Contains(err.Error(), q.password) {
				t.Fatal("accepted a failure or exposed QR password")
			}
			if mode == "cancel" && !errors.Is(err, context.Canceled) || mode == "timeout" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("lost cancellation cause: %v", err)
			}
			if mode != "reject" && mode != "pair-error" && pairs != 0 {
				t.Fatal("paired after cancellation or discovery failure")
			}
		})
	}
}

func TestQRDialogDismissal(t *testing.T) {
	for _, action := range []string{"cancel", "escape", "outside", "code", "mode", "shutdown"} {
		t.Run(action, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			started := make(chan struct{})
			b := &backend{run: func(ctx context.Context, args ...string) (string, error) {
				if args[0] == "kill-server" {
					return "", nil
				}
				close(started)
				<-ctx.Done()
				return "", ctx.Err()
			}}
			a := newApplication(ctx, b)
			a.showPair = true
			tt := ui.NewTester(a.view, 840, 600)
			if err := tt.Click("二维码配对"); err != nil {
				t.Fatal(err)
			}
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("QR pairing worker did not start")
			}
			if !a.busy || a.pairQR == nil || a.pairQR.bitmap == nil {
				t.Fatal("QR was not shown while waiting")
			}
			for _, label := range []string{"取消", "配对码配对", "重新生成二维码"} {
				r, ok := tt.Find(label)
				if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > 840 || r.Y+r.H > 600 {
					t.Fatalf("QR dialog control %q clipped: %+v", label, r)
				}
			}
			if path := os.Getenv("SCRCPY_GUI_QR_SCREENSHOT"); path != "" && action == "cancel" {
				file, err := os.Create(path)
				if err != nil {
					t.Fatal(err)
				}
				defer file.Close()
				if err := png.Encode(file, tt.Image()); err != nil {
					t.Fatal(err)
				}
			}
			switch action {
			case "cancel":
				if err := tt.Click("取消"); err != nil {
					t.Fatal(err)
				}
			case "escape":
				tt.Key(0, ui.KeyEscape)
			case "outside":
				tt.Press(5, 5)
				tt.Release(5, 5)
			case "code":
				if err := tt.Click("配对码配对"); err != nil {
					t.Fatal(err)
				}
			case "mode":
				a.setMode(true)
			case "shutdown":
				a.shutdown(cancel)
			}
			tt.Frame()
			if action != "shutdown" {
				done := make(chan struct{})
				go func() { a.workers.Wait(); close(done) }()
				select {
				case <-done:
				case <-time.After(time.Second):
					t.Fatal("dismissal did not cancel worker")
				}
				tt.Frame()
				if a.pairQR != nil || a.busy || a.showPair != (action == "code") {
					t.Fatal("stale QR result restored dismissed state")
				}
			}
			cancel()
			a.workers.Wait()
		})
	}
}

func TestQRPasswordHiddenInADBLog(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SCRCPY_GUI_TEST_CHILD", "echo")
	logs := &sessionLog{}
	logs.setEnabled(true)
	b := &backend{adb: exe, logs: logs}
	b.run = b.runADB
	password := "0123456789abcdef0123456789abcdef"
	if err := b.pairPassword(context.Background(), "192.168.1.3:40000", password); err == nil || strings.Contains(err.Error(), password) {
		t.Fatal("failed pairing response exposed QR password")
	}
	if strings.Contains(logs.String(), password) || !strings.Contains(logs.String(), "[配对码已隐藏]") {
		t.Fatal("ADB log exposed QR password")
	}
}

func TestQRApplicationFailureAndRetry(t *testing.T) {
	a := newApplication(context.Background(), &backend{run: func(ctx context.Context, args ...string) (string, error) {
		if args[0] == "start-server" {
			return "", nil
		}
		return "", errors.New("discovery unavailable")
	}})
	a.showPair, a.pairQRMode = true, true
	var previous *qrPairState
	for range 2 {
		a.startQRPair()
		if a.pairQR == nil || a.pairQR == previous {
			t.Fatal("retry reused the previous QR session")
		}
		previous = a.pairQR
		a.workers.Wait()
		select {
		case update := <-a.updates:
			update()
		default:
			t.Fatal("missing QR failure update")
		}
		if a.busy || a.pairQR.running || a.pairQR.bitmap != nil || !strings.Contains(a.pairQR.message, "mDNS 发现失败") {
			t.Fatal("failure did not clear QR image or preserve a useful retry message")
		}
	}
}
