package main

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

var version = "dev"

type windowControls interface {
	ToggleFullScreen()
	SetFullScreen(bool)
	IsFullScreen() bool
}

type application struct {
	ctx             context.Context
	b               *backend
	updates         chan func()
	workers         sync.WaitGroup
	networks        []string
	network         string
	devices         []device
	selected        string
	preset          int
	custom          [2]customPreset
	audioPreset     int
	customAudio     string
	customBuffer    string
	useCustomBuffer bool
	wired           bool
	playback        playbackOptions
	window          windowControls
	logWindow       *mygo.Window
	logs            *sessionLog
	address         string
	pairAddress     string
	pairCode        string
	showPair        bool
	busy            bool
	cancelJob       context.CancelFunc
	session         *exec.Cmd
	stopSession     context.CancelFunc
	stopping        bool
	status          string
	detail          string
	errorText       string
}

func newApplication(ctx context.Context, b *backend) *application {
	a := &application{ctx: ctx, b: b, updates: make(chan func(), 16), preset: 1,
		custom:      [2]customPreset{{"1600", "60", "6"}, {"0", "120", "40"}},
		audioPreset: 1, customAudio: "128", customBuffer: "2", logs: &sessionLog{},
		playback: playbackOptions{BufferMS: 2000, KeyboardUHID: true, AudioCodec: "opus", ScaleMode: string(scaleFit)}, networks: localNetworks(), status: "准备就绪"}
	if b != nil {
		b.logs = a.logs
	}
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
	a.logs.printf("INFO %s", message)
	a.workers.Add(1)
	go func() {
		defer a.workers.Done()
		defer cancel()
		update, err := job(ctx)
		if err != nil {
			a.logs.printf("ERROR %v", err)
		}
		a.post(func() {
			a.busy, a.cancelJob = false, nil
			if err != nil {
				a.status, a.errorText = "操作未完成", err.Error()
			} else if update != nil {
				update()
				a.logs.printf("INFO %s %s", a.status, a.detail)
			}
		})
	}()
}

func (a *application) setDevices(devices []device) {
	a.devices = devices
	for _, d := range devices {
		if d.key() == a.selected && d.usb() == a.wired {
			return
		}
	}
	a.selected = ""
	for _, d := range devices {
		if !d.Pairing && d.usb() == a.wired {
			a.selected = d.key()
			break
		}
	}
}

func (a *application) setMode(wired bool) {
	a.wired = wired
	a.preset = 1
	if wired {
		a.preset = len(presets) - 1
	}
	a.showPair, a.pairCode = false, ""
	a.setDevices(a.devices)
}

func (a *application) visibleDevices() []device {
	var devices []device
	for _, d := range a.devices {
		if d.usb() == a.wired {
			devices = append(devices, d)
		}
	}
	return devices
}

func (a *application) customSettings() *customPreset {
	if a.wired {
		return &a.custom[1]
	}
	return &a.custom[0]
}

func (a *application) discover() {
	if a.wired {
		a.work("正在查找 USB 设备…", func(ctx context.Context) (func(), error) {
			devices, err := a.b.discoverUSB(ctx)
			return func() {
				a.setDevices(devices)
				a.status, a.detail = "USB 刷新完成", "选择 USB 设备后开始投屏。"
				if len(devices) == 0 {
					a.detail = "未发现 USB 设备。请使用数据线连接，开启 USB 调试并在手机上允许授权。"
				}
			}, err
		})
		return
	}
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
	if chosen.key() == "" || chosen.Pairing || chosen.usb() != a.wired {
		return
	}
	var p preset
	playback := a.playback
	if a.useCustomBuffer {
		var err error
		playback.BufferMS, err = customBufferMS(a.customBuffer)
		if err != nil {
			a.status, a.errorText = "请检查自定义缓存", err.Error()
			a.logs.printf("ERROR %v", err)
			return
		}
	}
	if playback.AudioOnly {
		if playback.losslessAudio() {
			p.Name = "仅音频 · " + strings.ToUpper(playback.audioCodec()) + " 无损"
		} else if a.audioPreset == len(audioPresets) {
			var err error
			playback.AudioBitrateKbps, err = customAudioBitrate(a.customAudio)
			if err != nil {
				a.status, a.errorText = "请检查自定义音频码率", err.Error()
				a.logs.printf("ERROR %v", err)
				return
			}
		} else {
			playback.AudioBitrateKbps = audioPresets[a.audioPreset].BitrateKbps
		}
		if !playback.losslessAudio() {
			p.Name = fmt.Sprintf("仅音频 · %s · %d Kbps", strings.ToUpper(playback.audioCodec()), playback.AudioBitrateKbps)
		}
	} else if a.preset == len(presets) {
		var err error
		p, err = a.customSettings().preset()
		if err != nil {
			a.status, a.errorText = "请检查自定义档位", err.Error()
			a.logs.printf("ERROR %v", err)
			return
		}
	} else {
		p = presets[a.preset]
	}
	if p.USBOnly && !chosen.usb() {
		return
	}
	message := "正在启动投屏…"
	if playback.AudioOnly {
		message = "正在启动音频…"
	}
	if !playback.AudioOnly {
		playback.Frames = &mirrorFrames{}
	}
	a.work(message, func(ctx context.Context) (func(), error) {
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
		cmd, _, err := a.b.start(sessionCtx, serial, p, playback)
		if err != nil {
			cancel()
			return nil, err
		}
		type result struct{ err, restoreErr error }
		done := make(chan result, 1)
		shortcutsDone := make(chan struct{})
		a.workers.Add(1)
		go func() {
			defer a.workers.Done()
			defer close(shortcutsDone)
			if playback.AudioOnly {
				return
			}
			if err := runMirrorWindow(sessionCtx, cmd.Process.Pid, playback, a.b, serial, cancel); err != nil {
				a.logs.printf("ERROR 投屏缩放窗口: %v", err)
				cancel()
			}
		}()
		a.workers.Add(1)
		go func() {
			defer a.workers.Done()
			err := cmd.Wait()
			cancel()
			<-shortcutsDone
			var restoreErr error
			if playback.TurnScreenOff {
				restoreErr = a.b.restoreScreen(serial)
				if restoreErr != nil {
					a.logs.printf("ERROR %v", restoreErr)
				} else {
					a.logs.printf("INFO 已请求恢复设备亮屏")
				}
			}
			a.logs.printf("INFO scrcpy 已退出: %v", err)
			done <- result{err, restoreErr}
		}()
		return func() {
			a.session, a.stopSession, a.stopping = cmd, cancel, false
			buffer := "额外音视频缓存已关闭"
			if playback.BufferMS > 0 {
				buffer = fmt.Sprintf("音视频缓存 %s 秒", bufferSeconds(playback.BufferMS))
			}
			a.status, a.detail = "投屏运行中 · "+p.Name, buffer+"；投屏窗口 Alt+Z 调节画面缩放，F11 切换全屏，Esc 退出全屏，Alt+Q 关闭。"
			if playback.AudioOnly {
				buffer = "额外音频缓存已关闭"
				if playback.BufferMS > 0 {
					buffer = fmt.Sprintf("音频缓存 %s 秒", bufferSeconds(playback.BufferMS))
				}
				a.status, a.detail = "音频运行中 · "+p.Name, buffer+"；仅转发设备声音，不打开投屏窗口。点击停止音频结束。"
			}
			if playback.TurnScreenOff {
				a.detail += " 已请求关闭设备屏幕电源。"
			}
			go func() {
				result := <-done
				a.post(func() {
					a.session, a.stopSession = nil, nil
					a.status, a.detail = "投屏已结束", ""
					if playback.AudioOnly {
						a.status = "音频已结束"
					}
					if result.err != nil && !a.stopping {
						a.errorText = result.err.Error()
						a.logs.printf("ERROR 投屏异常退出: %v", result.err)
					}
					if playback.TurnScreenOff {
						a.detail = "已请求恢复设备亮屏。"
						if result.restoreErr != nil {
							a.errorText += "\n" + result.restoreErr.Error()
							a.detail = ""
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
	log.SetOutput(a.logs)
	defer func() {
		cancel()
		a.workers.Wait()
	}()
	if err != nil {
		a.status, a.errorText = "运行文件不完整", err.Error()
	}
	mygo.App.SetName("scrcpy LAN")
	mygo.Theme.SetSource(mygo.ThemeLight)
	mygo.App.OnBeforeQuit(func(_ *mygo.QuitEvent) { cancel() })
	mygo.App.WhenReady(func() {
		workArea := mygo.Screen.PrimaryDisplay().WorkArea
		width, height := min(1080, workArea.Width-40), min(880, workArea.Height-40)
		win := mygo.NewWindow(mygo.WindowOptions{
			Title: "scrcpy LAN · " + version, Width: width, Height: height,
			MinWidth: min(840, width), MinHeight: min(600, height),
			TitleBarStyle: mygo.TitleBarDefault, Content: ui.View(a.view),
		})
		a.window = win
		win.OnEnterFullScreen(win.Invalidate)
		win.OnLeaveFullScreen(win.Invalidate)
		win.OnMaximize(win.Invalidate)
		win.OnUnmaximize(win.Invalidate)
		win.OnClosed(func() { cancel(); mygo.App.Quit() })
		if b != nil {
			a.discover()
			win.Invalidate()
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
