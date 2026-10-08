package main

import (
	"context"
	"fmt"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestNativeViewSelectionAndPairing(t *testing.T) {
	a := newApplication(context.Background(), &backend{})
	a.networks, a.network = []string{"192.168.1.0/24"}, "192.168.1.0/24"
	a.devices = []device{
		{Name: "Pixel 9", Address: "192.168.1.10:37123", State: "device", Source: "无线调试"},
		{Name: "Pixel 配对服务", Address: "192.168.1.10:40123", Pairing: true, State: "待配对", Source: "无线调试"},
	}
	a.selected = a.devices[0].key()
	tt := ui.NewTester(a.view, 1080, 880)
	if err := tt.Click("高清 · 60 fps"); err != nil {
		t.Fatal(err)
	}
	if a.preset != 2 {
		t.Fatal("preset selection did not change")
	}
	if err := tt.Click("Pixel 配对服务"); err != nil {
		t.Fatal(err)
	}
	if a.pairAddress != "192.168.1.10:40123" {
		t.Fatal("pairing address was not populated")
	}
	if !a.showPair || !tt.HasText("配对 Android 设备") {
		t.Fatal("pairing dialog did not open")
	}
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	tt.Scroll(900, 500, 0, 500)
	if err := tt.Click("开始投屏"); err != nil {
		t.Fatal(err)
	}
	if a.busy {
		t.Fatal("pairing transport was offered for mirroring")
	}
	tt.Scroll(900, 500, 0, -500)
	if err := tt.Click("Pixel 9"); err != nil {
		t.Fatal(err)
	}
	if a.selected != "192.168.1.10:37123" {
		t.Fatal("device selection did not change")
	}
	if path := os.Getenv("SCRCPY_GUI_SCREENSHOT"); path != "" {
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := png.Encode(file, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNativeViewAtMinimumSize(t *testing.T) {
	a := newApplication(context.Background(), &backend{})
	a.networks, a.network = nil, ""
	tt := ui.NewTester(a.view, 840, 600)
	for _, text := range []string{"附近的设备", "流畅 · 30 fps", "原画 · 60 fps", "自定义", "开始投屏", "准备就绪"} {
		if !tt.HasText(text) {
			t.Errorf("missing UI text %q", text)
		}
	}
	tt.Scroll(700, 300, 0, 300)
	if path := os.Getenv("SCRCPY_GUI_MIN_SCREENSHOT"); path != "" {
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := png.Encode(file, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	if err := tt.Click("1 秒缓存"); err != nil {
		t.Fatal(err)
	}
	if a.playback.BufferMS != 1000 {
		t.Fatal("buffer settings were not reachable by scrolling at minimum size")
	}
	tt.Scroll(700, 300, 0, 500)
	if err := tt.Click("画面缩放模式"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(string(scaleFill)); err != nil || a.playback.ScaleMode != string(scaleFill) {
		t.Fatal("scale selection was not reachable at minimum size")
	}
	if path := os.Getenv("SCRCPY_GUI_MIN_SCREENSHOT"); path != "" {
		file, err := os.Create(path + "-scale.png")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := png.Encode(file, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNativeViewCompactLayout(t *testing.T) {
	for _, size := range [][2]int{{1080, 760}, {1536, 790}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			a := newApplication(context.Background(), &backend{})
			a.networks, a.network = []string{"192.168.1.0/24"}, "192.168.1.0/24"
			a.devices = []device{{Name: "Pixel 9", Address: "192.168.1.10:37123", State: "device", Source: "无线调试"}}
			a.selected = a.devices[0].key()
			tt := ui.NewTester(a.view, size[0], size[1])
			deviceRect, ok := tt.Find("Pixel 9")
			if !ok {
				t.Fatal("device missing")
			}
			preset, ok := tt.Find("流畅 · 30 fps")
			if !ok || preset.X > float32(size[0])*0.4 {
				t.Fatalf("device sidebar leaves too little room for settings: %+v", preset)
			}
			action, _ := tt.Find("开始投屏")
			for _, label := range []string{"流畅 · 30 fps", "原画 · 60 fps", "自定义", "1 秒缓存", "启动后熄灭设备屏幕（关闭屏幕电源）", "电脑键盘输入（UHID，支持手机输入法）", "投屏启动时全屏（覆盖任务栏）", "画面缩放模式"} {
				r, ok := tt.Find(label)
				if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > float32(size[0]) || r.Y+r.H >= action.Y {
					t.Errorf("setting %q is not fully visible above the action bar: %+v", label, r)
				}
			}
			if err := tt.Click("画面缩放模式"); err != nil {
				t.Fatal("window setting is not clickable without scrolling")
			}
			if err := tt.Click(string(scaleFill)); err != nil || a.playback.ScaleMode != string(scaleFill) {
				t.Fatal("fill scale selection failed")
			}
			if path := os.Getenv("SCRCPY_GUI_LAYOUT_SCREENSHOT"); path != "" {
				file, err := os.Create(fmt.Sprintf("%s-%dx%d.png", path, size[0], size[1]))
				if err != nil {
					t.Fatal(err)
				}
				defer file.Close()
				if err := png.Encode(file, tt.Image()); err != nil {
					t.Fatal(err)
				}
			}
			for i := range 20 {
				a.devices = append(a.devices, device{Name: fmt.Sprintf("Phone %d", i), Address: fmt.Sprintf("192.168.1.%d:5555", i+20), State: "device", Source: "局域网"})
			}
			tt.Frame()
			tt.Scroll(deviceRect.X, deviceRect.Y, 0, 500)
			if after, _ := tt.Find("Pixel 9"); after.Y >= deviceRect.Y {
				t.Fatal("long device list did not scroll")
			}
			if after, _ := tt.Find("流畅 · 30 fps"); after != preset {
				t.Fatalf("scrolling devices moved settings: before %+v, after %+v", preset, after)
			}
		})
	}
}

type testWindow struct {
	fullscreen bool
}

func (w *testWindow) ToggleFullScreen()    { w.fullscreen = !w.fullscreen }
func (w *testWindow) SetFullScreen(v bool) { w.fullscreen = v }
func (w *testWindow) IsFullScreen() bool   { return w.fullscreen }

func TestWindowControlsAndShortcuts(t *testing.T) {
	a := newApplication(context.Background(), nil)
	w := &testWindow{}
	a.window = w
	tt := ui.NewTester(a.view, 840, 600)
	for _, label := range []string{"最小化", "最大化", "还原窗口", "全屏", "退出全屏", "退出"} {
		if _, ok := tt.Find(label); ok {
			t.Fatalf("redundant window button %q remains", label)
		}
	}
	tt.Key(0, ui.KeyF11)
	if !w.fullscreen {
		t.Fatal("F11 did not enter fullscreen")
	}
	tt.Key(0, ui.KeyEscape)
	if w.fullscreen {
		t.Fatal("Escape did not leave fullscreen")
	}
	tt.Key(0, ui.KeyF11)
	if !w.fullscreen {
		t.Fatal("F11 did not enter fullscreen")
	}
	tt.Key(0, ui.KeyF11)
	if w.fullscreen {
		t.Fatal("F11 did not return to windowed mode")
	}
}

func TestWiredModeAndPlaybackSettings(t *testing.T) {
	a := newApplication(context.Background(), &backend{})
	a.setDevices(parseDevices("usb-phone device model:USB_Phone\n192.168.1.10:5555 device model:Wireless_Phone\n"))
	tt := ui.NewTester(a.view, 1080, 880)
	if tt.HasText("USB Phone") || tt.HasText("有线高规格") {
		t.Fatal("USB transport or preset appeared in wireless mode")
	}
	if a.playback.BufferMS != 2000 || a.playback.Fullscreen {
		t.Fatal("buffered windowed playback is not the default")
	}
	if err := tt.Click("USB 有线"); err != nil {
		t.Fatal(err)
	}
	if a.selected != "usb-phone" || !presets[a.preset].USBOnly || !tt.HasText("USB Phone") || tt.HasText("Wireless Phone") || tt.HasText("手动连接") {
		t.Fatal("wired mode did not select only USB transports and its high specification")
	}
	tt.Scroll(900, 500, 0, 500)
	for _, label := range []string{"关闭缓存", "1 秒缓存", "投屏启动时全屏（覆盖任务栏）", "画面缩放模式"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	if err := tt.Click(string(scaleStretch)); err != nil {
		t.Fatal(err)
	}
	if a.playback.BufferMS != 1000 || !a.playback.Fullscreen || a.playback.ScaleMode != string(scaleStretch) {
		t.Fatal("playback settings did not change")
	}
	a.sessions = []*deviceSession{{key: a.selected, playback: a.playback}}
	tt.Frame()
	for _, label := range []string{"无线 / 局域网", "关闭缓存", "2 秒缓存", "投屏启动时全屏（覆盖任务栏）"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	if a.wired || a.playback.BufferMS != 2000 || a.playback.Fullscreen {
		t.Fatal("running device blocked settings for the next session")
	}
	if a.sessions[0].playback.BufferMS != 1000 || !a.sessions[0].playback.Fullscreen {
		t.Fatal("next-session settings changed the running device snapshot")
	}
	a.sessions = nil
	tt.Frame()
	if err := tt.Click("无线 / 局域网"); err != nil {
		t.Fatal(err)
	}
	if a.wired || a.selected != "192.168.1.10:5555" || a.preset != 1 || a.playback.BufferMS != 2000 {
		t.Fatal("wireless mode did not restore selection or preserve buffer settings")
	}
}

func TestCustomPresetInputsAndModeIsolation(t *testing.T) {
	a := newApplication(context.Background(), &backend{})
	tt := ui.NewTester(a.view, 1080, 880)
	if err := tt.Click("自定义"); err != nil {
		t.Fatal(err)
	}
	tt.Scroll(900, 500, 0, 300)
	for label, value := range map[string]string{
		"分辨率长边（px，0 为原生）": "1280",
		"帧率上限（fps）":       "90",
		"视频码率（Mbps）":      "8",
	} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Key(ui.Ctrl, ui.KeyA)
		tt.Type(value)
	}
	if a.preset != len(presets) || *a.customSettings() != (customPreset{"1280", "90", "8"}) {
		t.Fatalf("custom inputs did not update: %+v", a.customSettings())
	}
	a.sessions = []*deviceSession{{key: a.selected, playback: a.playback}}
	tt.Frame()
	if err := tt.Click("帧率上限（fps）"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Ctrl, ui.KeyA)
	tt.Type("30")
	if a.customSettings().FPS != "30" {
		t.Fatal("running device blocked custom settings for the next session")
	}
	a.sessions = nil
	a.setMode(true)
	tt.Frame()
	tt.Scroll(900, 500, 0, -500)
	if err := tt.Click("自定义"); err != nil {
		t.Fatal(err)
	}
	if a.preset != len(presets) || *a.customSettings() != (customPreset{"0", "120", "40"}) {
		t.Fatal("USB mode did not offer its own custom settings")
	}
	a.setMode(false)
	if *a.customSettings() != (customPreset{"1280", "30", "8"}) {
		t.Fatal("mode switch lost wireless custom settings")
	}
}

func TestAudioModeInputsAndDisabledSettings(t *testing.T) {
	a := newApplication(context.Background(), &backend{})
	tt := ui.NewTester(a.view, 1080, 880)
	if a.playback.AudioOnly || a.playback.TurnScreenOff || !a.playback.KeyboardUHID || a.audioPreset != 1 {
		t.Fatal("incorrect screen power, keyboard or audio defaults")
	}
	if err := tt.Click("仅音频"); err != nil {
		t.Fatal(err)
	}
	if !a.playback.AudioOnly || !tt.HasText("开始音频") || tt.HasText("投屏档位") || tt.HasText("投屏启动时全屏（覆盖任务栏）") {
		t.Fatal("audio mode did not replace video settings and action")
	}
	if path := os.Getenv("SCRCPY_GUI_AUDIO_SCREENSHOT"); path != "" {
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := png.Encode(file, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	if err := tt.Click("高品质 · 256 Kbps"); err != nil {
		t.Fatal(err)
	}
	if a.audioPreset != 3 {
		t.Fatal("audio preset selection did not change")
	}
	if err := tt.Click("自定义音频码率"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("音频码率（Kbps，6–9000）"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Ctrl, ui.KeyA)
	tt.Type("173")
	tt.Scroll(900, 500, 0, 500)
	if err := tt.Click("启动后熄灭设备屏幕（关闭屏幕电源）"); err != nil {
		t.Fatal(err)
	}
	if a.customAudio != "173" || !a.playback.TurnScreenOff {
		t.Fatal("audio custom bitrate or screen power setting did not change")
	}
	a.sessions = []*deviceSession{{key: a.selected, playback: a.playback}}
	tt.Frame()
	if !tt.HasText("停止音频") {
		t.Fatal("running audio session has no stop action")
	}
	if err := tt.Click("启动后熄灭设备屏幕（关闭屏幕电源）"); err != nil {
		t.Fatal(err)
	}
	tt.Scroll(900, 500, 0, -1000)
	if err := tt.Click("画面和声音"); err != nil {
		t.Fatal(err)
	}
	if a.playback.AudioOnly || a.playback.TurnScreenOff || !a.sessions[0].playback.AudioOnly || !a.sessions[0].playback.TurnScreenOff || !tt.HasText("停止音频") {
		t.Fatal("next-session mode did not remain independent from running audio")
	}
	a.sessions = nil
	tt.Frame()
	if err := tt.Click("画面和声音"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("开始投屏") || a.preset != 1 || !a.playback.KeyboardUHID {
		t.Fatal("returning to video mode lost settings or the video start action")
	}
}

func TestMultiDeviceSessionActions(t *testing.T) {
	a := newApplication(context.Background(), &backend{})
	a.setDevices([]device{{Name: "Phone A", Address: "192.168.1.10:5555"}, {Name: "Phone B", Address: "192.168.1.11:5555"}, {Name: "Phone C", Address: "192.168.1.12:5555"}})
	stopped := [2]bool{}
	a.sessions = []*deviceSession{
		{key: a.devices[0].key(), serial: a.devices[0].key(), name: "Phone A", label: "均衡 · 60 fps", cancel: func() { stopped[0] = true }},
		{key: a.devices[1].key(), serial: a.devices[1].key(), name: "Phone B", label: "仅音频 · OPUS · 128 Kbps", playback: playbackOptions{AudioOnly: true}, cancel: func() { stopped[1] = true }},
	}
	a.selected = a.devices[2].key()
	tt := ui.NewTester(a.view, 1080, 880)
	if !tt.HasText("开始投屏") || !tt.HasText("运行中的设备 · 2") || !tt.HasText("仅音频 · OPUS · 128 Kbps") {
		t.Fatal("running devices hid the next-device action or session mode")
	}
	a.selected = a.devices[1].key()
	tt.Frame()
	if err := tt.Click("停止音频"); err != nil {
		t.Fatal(err)
	}
	if stopped != [2]bool{false, true} {
		t.Fatalf("stop affected the wrong device: %v", stopped)
	}
	if err := tt.Click("停止全部"); err != nil {
		t.Fatal(err)
	}
	if stopped != [2]bool{true, true} {
		t.Fatal("stop-all did not stop both devices")
	}
	if path := os.Getenv("SCRCPY_GUI_MULTI_SCREENSHOT"); path != "" {
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := png.Encode(file, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestScreenPowerAndKeyboardOptionsAtMinimumSize(t *testing.T) {
	a := newApplication(context.Background(), &backend{})
	tt := ui.NewTester(a.view, 840, 600)
	tt.Scroll(700, 300, 0, 400)
	for _, label := range []string{"启动后熄灭设备屏幕（关闭屏幕电源）", "电脑键盘输入（UHID，支持手机输入法）"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Scroll(700, 300, 0, 100)
	}
	if !a.playback.TurnScreenOff || a.playback.KeyboardUHID {
		t.Fatalf("screen power and keyboard fallback were not reachable at minimum size: screenOff=%v keyboardUHID=%v", a.playback.TurnScreenOff, a.playback.KeyboardUHID)
	}
	if path := os.Getenv("SCRCPY_GUI_OPTIONS_SCREENSHOT"); path != "" {
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := png.Encode(file, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCustomBufferAndAudioCodecs(t *testing.T) {
	a := newApplication(context.Background(), &backend{})
	tt := ui.NewTester(a.view, 1080, 880)
	if !tt.HasText("打开日志窗口") {
		t.Fatal("log window action is missing")
	}
	for _, label := range []string{"0.5 秒缓存", "1 秒缓存", "2 秒缓存", "自定义缓存"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	if err := tt.Click("缓存时间（秒，0–60）"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Ctrl, ui.KeyA)
	tt.Type("1.735")
	for _, label := range []string{"仅音频", "USB 有线"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	if !a.useCustomBuffer || a.customBuffer != "1.735" {
		t.Fatal("mode switch lost custom buffer")
	}
	for _, codec := range []string{"flac", "raw", "aac", "opus"} {
		if err := tt.Click("音频编码格式"); err != nil {
			t.Fatal(err)
		}
		if err := tt.Click(codec); err != nil {
			t.Fatal(err)
		}
		if a.playback.AudioCodec != codec || tt.HasText("音频码率") == a.playback.losslessAudio() {
			t.Fatalf("incorrect codec controls for %s", codec)
		}
	}
	a.sessions = []*deviceSession{{key: a.selected, playback: a.playback}}
	tt.Frame()
	if err := tt.Click("缓存时间（秒，0–60）"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Ctrl, ui.KeyA)
	tt.Type("4")
	if a.customBuffer != "4" {
		t.Fatal("running device blocked the next session's buffer settings")
	}
}
