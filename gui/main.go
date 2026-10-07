package main

import (
	"context"
	"fmt"
	"log"
	"os/exec"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

var version = "dev"

type application struct {
	ctx         context.Context
	b           *backend
	updates     chan func()
	networks    []string
	network     string
	devices     []device
	selected    string
	preset      int
	address     string
	pairAddress string
	pairCode    string
	showPair    bool
	busy        bool
	cancelJob   context.CancelFunc
	session     *exec.Cmd
	stopSession context.CancelFunc
	stopping    bool
	status      string
	detail      string
	errorText   string
}

func newApplication(ctx context.Context, b *backend) *application {
	a := &application{ctx: ctx, b: b, updates: make(chan func(), 16), preset: 1, networks: localNetworks(), status: "准备就绪"}
	if len(a.networks) > 0 {
		a.network = a.networks[0]
	}
	return a
}

func (a *application) post(update func()) {
	select {
	case a.updates <- update:
	case <-a.ctx.Done():
	}
}

// Workers return changes to apply during a UI frame; only the UI thread owns
// application state. Network operations and child processes never block it.
func (a *application) work(message string, job func(context.Context) (func(), error)) {
	if a.busy || a.b == nil {
		return
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.cancelJob, a.busy, a.status, a.errorText = cancel, true, message, ""
	go func() {
		defer cancel()
		update, err := job(ctx)
		a.post(func() {
			a.busy, a.cancelJob = false, nil
			if err != nil {
				a.status, a.errorText = "操作未完成", err.Error()
			} else if update != nil {
				update()
			}
		})
	}()
}

func (a *application) setDevices(devices []device) {
	a.devices = devices
	for _, d := range devices {
		if d.key() == a.selected {
			return
		}
	}
	a.selected = ""
	for _, d := range devices {
		if !d.Pairing {
			a.selected = d.key()
			break
		}
	}
}

func (a *application) discover() {
	network := a.network
	a.work("正在发现局域网设备…", func(ctx context.Context) (func(), error) {
		devices, note, err := a.b.discover(ctx, network)
		return func() {
			a.setDevices(devices)
			a.status, a.detail = "扫描完成", note
			if len(devices) == 0 {
				a.detail = "未发现设备。请开启手机无线调试，或用 IP:端口手动连接。" + note
			}
		}, err
	})
}

func (a *application) connect() {
	address := a.address
	a.work("正在连接设备…", func(ctx context.Context) (func(), error) {
		serial, err := a.b.connect(ctx, address)
		if err != nil {
			return nil, err
		}
		out, err := a.b.run(ctx, "devices", "-l")
		return func() {
			a.setDevices(mergeDevices(a.devices, parseDevices(out)))
			a.selected, a.status, a.detail = serial, "设备已连接", "选择档位后点击开始投屏。"
		}, err
	})
}

func (a *application) pair() {
	address, code := a.pairAddress, a.pairCode
	a.pairCode, a.showPair = "", false
	a.work("正在配对…", func(ctx context.Context) (func(), error) {
		if err := a.b.pair(ctx, address, code); err != nil {
			return nil, err
		}
		devices, note, err := a.b.discover(ctx, "")
		return func() {
			a.setDevices(devices)
			a.status = "配对成功"
			a.detail = "选择无线调试连接设备后投屏；也可填写手机无线调试主页的 IP:端口。" + note
		}, err
	})
}

func (a *application) start() {
	if a.session != nil {
		return
	}
	var chosen device
	for _, d := range a.devices {
		if d.key() == a.selected {
			chosen = d
			break
		}
	}
	if chosen.key() == "" || chosen.Pairing {
		return
	}
	p := presets[a.preset]
	a.work("正在启动投屏…", func(ctx context.Context) (func(), error) {
		serial := chosen.Serial
		if chosen.Address != "" {
			var err error
			serial, err = a.b.connect(ctx, chosen.Address)
			if err != nil {
				return nil, err
			}
		} else {
			out, err := a.b.run(ctx, "-s", serial, "get-state")
			if err != nil || out != "device\n" && out != "device\r\n" {
				return nil, fmt.Errorf("设备 %s 尚未授权或离线，请解锁手机并允许 USB / 无线调试", serial)
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		sessionCtx, cancel := context.WithCancel(a.ctx)
		cmd, output, err := a.b.start(sessionCtx, serial, p)
		if err != nil {
			cancel()
			return nil, err
		}
		return func() {
			a.session, a.stopSession, a.stopping = cmd, cancel, false
			a.status, a.detail = "投屏运行中 · "+p.Name, "投屏显示在独立窗口，可用键盘和鼠标控制手机。"
			go func() {
				err := cmd.Wait()
				cancel()
				a.post(func() {
					a.session, a.stopSession = nil, nil
					a.status, a.detail = "投屏已结束", ""
					if err != nil && !a.stopping {
						a.errorText = output.String()
						if a.errorText == "" {
							a.errorText = err.Error()
						}
					}
					a.stopping = false
				})
			}()
		}, nil
	})
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b, err := newBackend()
	a := newApplication(ctx, b)
	if err != nil {
		a.status, a.errorText = "运行文件不完整", err.Error()
	}
	mygo.App.SetName("scrcpy LAN")
	mygo.Theme.SetSource(mygo.ThemeLight)
	mygo.App.OnBeforeQuit(func(_ *mygo.QuitEvent) { cancel() })
	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title: "scrcpy LAN · " + version, Width: 1080, Height: 880,
			MinWidth: 940, MinHeight: 720, Content: ui.View(a.view),
		})
		win.OnClosed(cancel)
		if b != nil {
			a.discover()
			win.Invalidate()
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
