package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// A real child process exercises argument boundaries and cancellation without
// requiring a phone or altering the user's ADB server.
func init() {
	if mode := os.Getenv("SCRCPY_GUI_TEST_CHILD"); mode != "" {
		if path := os.Getenv("SCRCPY_GUI_TEST_ARGS"); path != "" {
			data, _ := json.Marshal(os.Args[1:])
			if err := os.WriteFile(path, data, 0600); err != nil {
				os.Exit(2)
			}
		}
		if mode == "wait" {
			time.Sleep(30 * time.Second)
		}
		if mode == "echo" {
			fmt.Println(strings.Join(os.Args[1:], " "))
			os.Exit(0)
		}
		if mode == "fail" {
			fmt.Fprintln(os.Stderr, "ERROR: simulated capture failure")
			os.Exit(3)
		}
		fmt.Print("device\n")
		os.Exit(0)
	}
}

func TestDeviceDiscoveryMergesConnectedServices(t *testing.T) {
	mdns := parseMDNS(`List of discovered mdns services
adb-phone _adb-tls-connect._tcp. 192.168.1.10:37123
adb-phone-pair _adb-tls-pairing._tcp 192.168.1.10:40123
adb-old _adb._tcp 192.168.1.20:5555
printer _ipp._tcp 192.168.1.99:631
bad _adb._tcp invalid
`)
	connected := parseDevices(`List of devices attached
adb-phone._adb-tls-connect._tcp device product:panther model:Pixel_7 device:panther transport_id:1
192.168.1.20:5555 unauthorized transport_id:2
usb-123 offline model:Pixel_8
`)
	legacy := []device{{Address: "192.168.1.20:5555"}, {Address: "192.168.1.30:5555"}}
	devices := mergeDevices(legacy, mdns, connected)
	if len(devices) != 5 {
		t.Fatalf("expected one entry per transport (including pairing), got %+v", devices)
	}
	for _, d := range devices {
		if d.Name == "Pixel 7" && (d.Address != "192.168.1.10:37123" || d.State != "device") {
			t.Fatalf("lost mDNS address or connection state: %+v", d)
		}
		if d.Pairing && d.Address != "192.168.1.10:40123" {
			t.Fatalf("pairing transport confused with connect transport: %+v", d)
		}
	}
	if devices[2].Address != "192.168.1.20:5555" || devices[3].Address != "192.168.1.30:5555" {
		t.Fatalf("distinct legacy candidates were merged: %+v", devices)
	}
}

func TestAddressesAndBoundedScan(t *testing.T) {
	for value, want := range map[string]string{
		" 192.168.1.10 ": "192.168.1.10:5555",
		"10.0.0.2:37000": "10.0.0.2:37000",
		"[fd00::2]:1234": "[fd00::2]:1234",
	} {
		got, err := normalizeAddress(value, true)
		if err != nil || got != want {
			t.Errorf("%q: %q, %v", value, got, err)
		}
	}
	for _, value := range []string{"", "example.com:5555", "8.8.8.8:5555", "192.168.1.2:0", "192.168.1.2:65536", "192.168.1.2:5555;echo"} {
		if _, err := normalizeAddress(value, true); err == nil {
			t.Errorf("accepted invalid LAN address %q", value)
		}
	}
	hosts, err := subnetHosts("192.168.1.42/24")
	if err != nil || len(hosts) != 254 || hosts[0] != "192.168.1.1:5555" || hosts[253] != "192.168.1.254:5555" {
		t.Fatalf("incorrect scan bounds: %d hosts, %v", len(hosts), err)
	}
	if _, err := subnetHosts("10.0.0.0/8"); err == nil {
		t.Fatal("accepted unbounded subnet")
	}
}

func TestConnectRequiresReadyDevice(t *testing.T) {
	var calls [][]string
	b := &backend{run: func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, args)
		if args[0] == "connect" {
			return "already connected to 192.168.1.10:5555\n", nil
		}
		return "device\r\n", nil
	}}
	serial, err := b.connect(context.Background(), "192.168.1.10")
	if err != nil || serial != "192.168.1.10:5555" || len(calls) != 2 {
		t.Fatalf("connect: %q, %v, calls %v", serial, err, calls)
	}
	b.run = func(_ context.Context, args ...string) (string, error) { return "failed to connect: refused", nil }
	if _, err := b.connect(context.Background(), serial); err == nil {
		t.Fatal("accepted adb failure with zero exit status")
	}
	b.run = func(_ context.Context, args ...string) (string, error) {
		if args[0] == "connect" {
			return "connected to " + serial, nil
		}
		return "unauthorized", nil
	}
	if _, err := b.connect(context.Background(), serial); err == nil {
		t.Fatal("accepted unauthorized transport")
	}
}

func TestPairValidationAndFailure(t *testing.T) {
	var calls [][]string
	b := &backend{run: func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, args)
		return "Successfully paired to 192.168.1.10:40000", nil
	}}
	for _, code := range []string{"", "12345", "1234567", "12x456"} {
		if err := b.pair(context.Background(), "192.168.1.10:40000", code); err == nil {
			t.Fatalf("accepted invalid code %q", code)
		}
	}
	if len(calls) != 0 {
		t.Fatal("invalid input launched adb")
	}
	if err := b.pair(context.Background(), "192.168.1.10:40000", "012345"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls[0], []string{"pair", "192.168.1.10:40000", "012345"}) {
		t.Fatalf("pair args: %v", calls)
	}
	b.run = func(_ context.Context, _ ...string) (string, error) { return "Failed: wrong password", nil }
	if err := b.pair(context.Background(), "192.168.1.10:40000", "123456"); err == nil {
		t.Fatal("accepted rejected pairing")
	}
}

func TestChildProcessArgumentsAndStop(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "args.json")
	t.Setenv("SCRCPY_GUI_TEST_CHILD", "exit")
	t.Setenv("SCRCPY_GUI_TEST_ARGS", file)
	b := &backend{adb: exe, scrcpy: exe}
	playback := playbackOptions{BufferMS: 3000, Fullscreen: true, ScaleMode: string(scaleStretch), TurnScreenOff: true, KeyboardUHID: true}
	out, err := b.runADB(context.Background(), "-s", "192.168.1.10:5555", "get-state")
	if err != nil || out != "device\n" {
		t.Fatalf("adb subprocess: %q %v", out, err)
	}
	cmd, _, err := b.start(context.Background(), "phone with spaces", presets[1], playback)
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var args []string
	if err := json.Unmarshal(data, &args); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(args, presets[1].args("phone with spaces", playback)) {
		t.Fatalf("argument boundaries changed: %v", args)
	}
	for _, arg := range []string{"--max-size=1600", "--max-fps=60", "--video-bit-rate=6M", "--video-codec=h264", "--video-buffer=3000", "--audio-buffer=3000", "--fullscreen", "--render-fit=stretched", "--turn-screen-off", "--keyboard=uhid"} {
		if !strings.Contains(string(data), arg) {
			t.Errorf("balanced preset missing %s", arg)
		}
	}
	t.Setenv("SCRCPY_GUI_TEST_CHILD", "wait")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd, _, err = b.start(ctx, "192.168.1.10:5555", presets[0], playback)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled mirror exited successfully")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("mirror did not stop after cancellation")
	}
}

func TestAudioOnlyArgumentsAndScreenPower(t *testing.T) {
	for _, p := range audioPresets {
		for _, ms := range []int{0, 500, 1000, 2000, 1735} {
			for _, screenOff := range []bool{false, true} {
				args := strings.Join(presets[4].args("phone", playbackOptions{
					AudioOnly: true, AudioBitrateKbps: p.BitrateKbps, BufferMS: ms,
					TurnScreenOff: screenOff, KeyboardUHID: true, Fullscreen: true, ScaleMode: string(scaleStretch),
				}), " ")
				for _, flag := range []string{"--no-video", "--no-window", "--require-audio", "--audio-codec=opus", fmt.Sprintf("--audio-bit-rate=%dK", p.BitrateKbps)} {
					if !strings.Contains(args, flag) {
						t.Errorf("audio-only session missing %s: %s", flag, args)
					}
				}
				for _, flag := range []string{"--video-", "--max-size", "--max-fps", "--fullscreen", "--render-fit", "--window-title", "--keyboard"} {
					if strings.Contains(args, flag) {
						t.Errorf("audio-only session contains video/input option %s: %s", flag, args)
					}
				}
				if strings.Contains(args, "--turn-screen-off") != screenOff || strings.Contains(args, "--no-control") == screenOff {
					t.Fatalf("screen power control disabled or unexpectedly enabled: %s", args)
				}
				if strings.Contains(args, "--audio-buffer=") != (ms > 0) || ms > 0 && !strings.Contains(args, fmt.Sprintf("--audio-buffer=%d", ms)) {
					t.Fatalf("incorrect audio buffer: %s", args)
				}
			}
		}
	}
	args := strings.Join(presets[1].args("phone", playbackOptions{}), " ")
	if strings.Contains(args, "--turn-screen-off") || strings.Contains(args, "--keyboard=uhid") || strings.Contains(args, "--no-video") {
		t.Fatalf("disabled screen/keyboard/audio-only options leaked into mirroring: %s", args)
	}
}

func TestCustomAudioBitrateValidation(t *testing.T) {
	for value, want := range map[string]int{"6": 6, " 173 ": 173, "510": 510, "511": 511, "9000": 9000} {
		got, err := customAudioBitrate(value)
		if err != nil || got != want {
			t.Errorf("%q: %d, %v", value, got, err)
		}
	}
	for _, value := range []string{"", "5", "9001", "-1", "128.5", "128K", "128;echo", "99999999999999999999"} {
		if _, err := customAudioBitrate(value); err == nil {
			t.Errorf("accepted invalid audio bitrate %q", value)
		}
	}
	args := strings.Join(presets[0].args("phone", playbackOptions{AudioOnly: true, AudioBitrateKbps: 9000}), " ")
	if !strings.Contains(args, "--audio-bit-rate=9000K") {
		t.Fatalf("9000 Kbps request was changed: %s", args)
	}
}

func TestAudioOnlySessionValidationAndStop(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "audio-args.json")
	t.Setenv("SCRCPY_GUI_TEST_CHILD", "wait")
	t.Setenv("SCRCPY_GUI_TEST_ARGS", file)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := &backend{adb: exe, scrcpy: exe, run: func(_ context.Context, _ ...string) (string, error) { return "device\n", nil }}
	a := newApplication(ctx, b)
	a.wired, a.playback.AudioOnly = true, true
	a.preset, a.custom[1].FPS = len(presets), "invalid"
	a.audioPreset, a.customAudio = len(audioPresets), "invalid"
	a.setDevices([]device{{Serial: "usb-phone", State: "device"}})
	a.start()
	if a.busy || a.session != nil || a.errorText == "" {
		t.Fatal("invalid audio settings launched a job or failed to show an error")
	}
	a.customAudio = "173"
	a.start()
	select {
	case update := <-a.updates:
		update()
	case <-time.After(3 * time.Second):
		t.Fatal("audio session did not start")
	}
	if a.session == nil || !strings.Contains(a.status, "音频运行中") || !strings.Contains(a.detail, "不打开投屏窗口") {
		t.Fatalf("audio session did not ignore unused video settings or report its mode: %s %s", a.status, a.errorText)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		if data, err := os.ReadFile(file); err == nil && len(data) > 0 {
			if !strings.Contains(string(data), "--audio-bit-rate=173K") {
				t.Fatalf("custom audio bitrate not passed to child: %s", data)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("audio child did not receive arguments")
		}
		time.Sleep(10 * time.Millisecond)
	}
	a.stopping = true
	a.stopSession()
	select {
	case update := <-a.updates:
		update()
	case <-time.After(3 * time.Second):
		t.Fatal("audio session did not stop")
	}
	a.workers.Wait()
	if a.session != nil || a.stopSession != nil || a.stopping || a.status != "音频已结束" || a.errorText != "" {
		t.Fatalf("audio stop did not clean up session: %s %s", a.status, a.errorText)
	}
}

func TestUSBDiscoveryExcludesNetworkAndEmulator(t *testing.T) {
	var calls [][]string
	b := &backend{run: func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, args)
		return `List of devices attached
usb-phone device model:Pixel_9
usb-locked unauthorized
192.168.1.10:5555 device model:Wireless
adb-phone._adb-tls-connect._tcp device model:Wireless
emulator-5554 device model:Emulator
`, nil
	}}
	devices, err := b.discoverUSB(context.Background())
	if err != nil || len(devices) != 2 || devices[0].Serial != "usb-phone" || devices[1].State != "unauthorized" {
		t.Fatalf("incorrect USB discovery: %+v, %v", devices, err)
	}
	if !reflect.DeepEqual(calls, [][]string{{"start-server"}, {"devices", "-l"}}) {
		t.Fatalf("USB discovery attempted network scanning: %v", calls)
	}
}

func TestAllPresetsOptionalAudioAndVideoBuffer(t *testing.T) {
	for _, p := range presets {
		for _, ms := range []int{0, 500, 1000, 2000, 1735} {
			args := p.args("usb-phone", playbackOptions{BufferMS: ms})
			for _, flag := range []string{"--video-buffer=", "--audio-buffer="} {
				found := strings.Contains(strings.Join(args, " "), flag)
				if found != (ms > 0) || ms > 0 && !strings.Contains(strings.Join(args, " "), fmt.Sprintf("%s%d", flag, ms)) {
					t.Errorf("%s buffer %d: incorrect %s in %v", p.Name, ms, flag, args)
				}
			}
			for _, arg := range args {
				if arg == "--fullscreen" || arg == "--render-fit=stretched" || arg == "--window-borderless" {
					t.Errorf("windowed proportional playback contains %s", arg)
				}
			}
		}
	}
	p := presets[len(presets)-1]
	if !p.USBOnly || p.Size != 0 || p.FPS != 120 || p.Bitrate != "40M" {
		t.Fatalf("incorrect wired specification: %+v", p)
	}
}

func TestCustomPresetValidationAndArguments(t *testing.T) {
	for _, input := range []customPreset{{"1280", "90", "8"}, {" 0 ", "120", "40"}} {
		p, err := input.preset()
		if err != nil {
			t.Fatal(err)
		}
		args := strings.Join(p.args("phone", playbackOptions{}), " ")
		for _, flag := range []string{"--max-size=" + strings.TrimSpace(input.Size), "--max-fps=" + input.FPS, "--video-bit-rate=" + input.BitrateMbps + "M"} {
			if !strings.Contains(args, flag) {
				t.Errorf("custom arguments missing %s: %s", flag, args)
			}
		}
		if strings.Contains(args, "-buffer=") || strings.Contains(args, "--fullscreen") {
			t.Fatalf("unbuffered windowed custom session contains extra options: %s", args)
		}
	}
	for _, input := range []customPreset{
		{"", "60", "6"}, {"-1", "60", "6"}, {"65536", "60", "6"},
		{"1600", "0", "6"}, {"1600", "oops", "6"}, {"1600", "65536", "6"},
		{"1600", "60", "0"}, {"1600", "60", "2148"}, {"1600", "60", "6M;echo"},
	} {
		if _, err := input.preset(); err == nil {
			t.Errorf("accepted invalid custom preset: %+v", input)
		}
	}
	a := newApplication(context.Background(), &backend{})
	a.wired, a.preset = true, len(presets)
	a.setDevices([]device{{Serial: "usb-phone", State: "device"}})
	a.customSettings().FPS = "invalid"
	a.start()
	if a.busy || a.session != nil || a.errorText == "" {
		t.Fatal("invalid custom settings launched a job or failed to show an error")
	}
}

func TestExitReapsSessionBeforeUIUpdate(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "started.json")
	t.Setenv("SCRCPY_GUI_TEST_CHILD", "wait")
	t.Setenv("SCRCPY_GUI_TEST_ARGS", file)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := &backend{adb: exe, scrcpy: exe, run: func(_ context.Context, _ ...string) (string, error) { return "device\n", nil }}
	a := newApplication(ctx, b)
	a.wired = true
	a.setDevices([]device{{Serial: "usb-phone", State: "device"}})
	a.start()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Stat(file); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("session did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	done := make(chan struct{})
	go func() { a.workers.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("exit did not reap the child without another UI frame")
	}
}
