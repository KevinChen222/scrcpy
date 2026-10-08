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

type deviceSession struct {
	key, serial, name string
	device            device
	label             string
	playback          playbackOptions
	cmd               *exec.Cmd
	cancel            context.CancelFunc
	stopping          bool
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
	sessions        []*deviceSession
	status          string
	detail          string
	errorText       string
}

func newApplication(ctx context.Context, b *backend) *application {
	a := &application{ctx: ctx, b: b, updates: make(chan func(), 16), preset: 1,
		custom:      [2]customPreset{{"1600", "60", "6"}, {"0", "120", "40"}},
		audioPreset: 1, customAudio: "128", customBuffer: "2", logs: &sessionLog{},
		playback: playbackOptions{BufferMS: 2000, KeyboardUHID: true, MuteOnStop: true, AudioCodec: "opus", ScaleMode: string(scaleFit)}, networks: localNetworks(), status: "准备就绪"}
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
	var selected device
	for _, old := range a.devices {
		if old.key() == a.selected || old.hasConnection(a.selected) {
			selected = old
		}
	}
	devices = mergeDevices(devices)
	for i, d := range devices {
		for _, old := range a.devices {
			if sameDevice(old, d) {
				// Keep aliases across refreshes without treating disappeared
				// connections as ready. Current discovery owns their state.
				d = combineDevice(old.stale(), d)
			}
		}
		active := ""
		for _, session := range a.sessions {
			if session.matches(d) {
				// Preserve the session's confirmed identity and original ADB
				// serial when a refresh only reports an unverified alias.
				d = combineDevice(session.device.stale(), d)
				session.device = d
				session.key, session.name = d.key(), d.displayName()
				active = session.serial
			}
		}
		devices[i] = d.withConnection(preferredConnection(d.connections(), active))
	}
	a.devices = devices
	for _, d := range a.visibleDevices() {
		if d.key() == a.selected || d.hasConnection(a.selected) || sameDevice(selected, d) {
			a.selected = d.key()
			return
		}
	}
	a.selected = ""
	for _, d := range a.visibleDevices() {
		if !d.Pairing {
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
		active := ""
		for _, session := range a.sessions {
			if session.matches(d) {
				active = session.serial
				break
			}
		}
		if visible, ok := d.forMode(a.wired, active); ok {
			devices = append(devices, visible)
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
		if err != nil {
			return nil, err
		}
		devices := a.b.resolveDevices(ctx, parseDevices(out))
		return func() {
			a.setDevices(mergeDevices(a.devices, devices))
			for _, d := range a.devices {
				if d.hasConnection(serial) {
					a.selected = d.key()
					break
				}
			}
			a.status, a.detail = "设备已连接", "选择档位后点击开始投屏。"
		}, ctx.Err()
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
	if a.busy || a.b == nil || a.selectedSession() != nil {
		return
	}
	var chosen device
	for _, d := range a.visibleDevices() {
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
	activeDevices := make([]device, 0, len(a.sessions))
	for _, session := range a.sessions {
		d := session.device
		d.Serial, d.State = session.serial, "device"
		activeDevices = append(activeDevices, d)
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
		ready := a.b.identifyDevices(ctx, []device{{Serial: serial, Address: chosen.Address,
			Name: chosen.Name, DeviceName: chosen.DeviceName, State: "device", Source: chosen.Source}})[0]
		ready = combineDevice(chosen, ready)
		for i, active := range activeDevices {
			if active.Identity == "" {
				active = a.b.identifyDevices(ctx, []device{active})[0]
				activeDevices[i] = active
			}
			if sameDevice(active, ready) {
				return func() {
					a.setDevices(mergeDevices(a.devices, activeDevices, []device{ready}))
					a.status = "设备已有运行中的会话"
					a.errorText = "请先停止该设备，再更改投屏或音频设置。"
				}, nil
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
		type result struct{ err, restoreErr, muteErr error }
		done := make(chan result, 1)
		shortcutsDone := make(chan struct{})
		a.workers.Add(1)
		go func() {
			defer a.workers.Done()
			defer close(shortcutsDone)
			if playback.AudioOnly {
				return
			}
			if err := runMirrorWindow(sessionCtx, cmd.Process.Pid, playback, serial, cancel); err != nil {
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
			var muteErr error
			if playback.MuteOnStop {
				muteErr = a.b.muteMedia(serial)
				if muteErr != nil {
					a.logs.printf("ERROR %v", muteErr)
				} else {
					a.logs.printf("INFO %s 媒体音量已设为 0", serial)
				}
			}
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
			done <- result{err, restoreErr, muteErr}
		}()
		return func() {
			session := &deviceSession{key: ready.key(), serial: serial, name: ready.displayName(), device: ready,
				label: p.Name, playback: playback, cmd: cmd, cancel: cancel}
			a.sessions = append(a.sessions, session)
			a.setDevices(mergeDevices(a.devices, []device{ready}))
			buffer := "额外音视频缓存已关闭"
			if playback.BufferMS > 0 {
				buffer = fmt.Sprintf("音视频缓存 %s 秒", bufferSeconds(playback.BufferMS))
			}
			a.status, a.detail = "投屏运行中 · "+p.Name, buffer+"；F11 全屏后按 Alt+Z 选择区域，Alt+Q 关闭。可继续选择其他设备启动。"
			if playback.AudioOnly {
				buffer = "额外音频缓存已关闭"
				if playback.BufferMS > 0 {
					buffer = fmt.Sprintf("音频缓存 %s 秒", bufferSeconds(playback.BufferMS))
				}
				a.status, a.detail = "音频运行中 · "+p.Name, buffer+"；不打开投屏窗口。可继续选择其他设备启动，每台设备可单独停止。"
			}
			if playback.TurnScreenOff {
				a.detail += " 已请求关闭设备屏幕电源。"
			}
			if playback.MuteOnStop {
				a.detail += " 结束后自动静音该设备。"
			}
			go func() {
				result := <-done
				a.post(func() {
					a.finishSession(session, result.err, result.restoreErr, result.muteErr)
				})
			}()
		}, nil
	})
}

func (a *application) selectedSession() *deviceSession {
	for _, session := range a.sessions {
		if a.selected == session.key || a.selected == session.serial {
			return session
		}
		for _, d := range a.devices {
			if d.key() == a.selected && session.matches(d) {
				return session
			}
		}
	}
	return nil
}

func (s *deviceSession) matches(d device) bool {
	if s.device.Identity != "" && d.Identity != "" {
		return s.device.Identity == d.Identity
	}
	return s.key != "" && s.key == d.key() || d.hasConnection(s.serial) || sameDevice(s.device, d)
}

func (a *application) stop(session *deviceSession) {
	if !session.stopping {
		session.stopping = true
		session.cancel()
	}
}

func (a *application) finishSession(session *deviceSession, err, restoreErr, muteErr error) {
	for i, active := range a.sessions {
		if active == session {
			a.sessions = append(a.sessions[:i], a.sessions[i+1:]...)
			break
		}
	}
	a.setDevices(a.devices)
	a.status, a.detail = session.serial+" 会话已结束", ""
	if len(a.sessions) > 0 {
		a.detail = fmt.Sprintf("其他 %d 台设备继续运行。", len(a.sessions))
	}
	if err != nil && !session.stopping {
		a.errorText = session.serial + ": " + err.Error()
		a.logs.printf("ERROR %s 会话异常退出: %v", session.serial, err)
	}
	if session.playback.TurnScreenOff {
		if restoreErr != nil {
			a.errorText = strings.TrimSpace(a.errorText + "\n" + restoreErr.Error())
		} else {
			a.detail += " 已请求恢复设备亮屏。"
		}
	}
	if session.playback.MuteOnStop {
		if muteErr != nil {
			a.errorText = strings.TrimSpace(a.errorText + "\n" + muteErr.Error())
		} else {
			a.detail += " 设备媒体声音已静音。"
		}
	}
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
