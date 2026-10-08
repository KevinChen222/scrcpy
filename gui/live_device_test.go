package main

import (
	"context"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestLiveDeviceListAndRefresh(t *testing.T) {
	serial := os.Getenv("SCRCPY_GUI_DEVICE_LIST_TEST_SERIAL")
	if serial == "" {
		t.Skip("set SCRCPY_GUI_DEVICE_LIST_TEST_SERIAL for device identity/refresh checks")
	}
	b, err := newBackend()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := newApplication(ctx, b)
	a.logs.setEnabled(true)
	a.playback.MuteOnStop = false
	t.Cleanup(func() { cancel(); a.workers.Wait() })
	devices, _, err := b.discover(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	a.setDevices(devices)
	var chosen device
	for _, d := range devices {
		if d.hasConnection(serial) {
			chosen = d
			a.selected = d.key()
		}
	}
	if chosen.Identity == "" || chosen.State != "device" {
		t.Fatalf("authorized physical phone not identified: %+v", chosen)
	}
	out, err := b.run(ctx, "devices", "-l")
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range parseDevices(out) {
		if raw.State != "device" {
			continue
		}
		identity := b.readPhoneInfo(ctx, raw.Serial).Identity
		matches := 0
		for _, d := range devices {
			if d.Identity == identity && d.hasConnection(raw.Serial) {
				matches++
			}
		}
		if identity == "" || matches != 1 {
			t.Fatalf("real transport did not resolve to exactly one physical phone: %s", raw.Serial)
		}
	}
	if path := os.Getenv("SCRCPY_GUI_DEVICE_LIST_SCREENSHOT"); path != "" {
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(file, ui.NewTester(a.view, 1080, 880).Image())
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	a.start()
	select {
	case update := <-a.updates:
		update()
	case <-time.After(15 * time.Second):
		t.Fatal("real session startup timed out")
	}
	session := a.selectedSession()
	if session == nil {
		t.Fatalf("real session failed to start: %s", a.errorText)
	}
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(a.logs.String(), "INFO: Texture") {
		if time.Now().After(deadline) {
			t.Fatalf("no decoded real video: %s", a.logs.String())
		}
		time.Sleep(100 * time.Millisecond)
	}
	devices, _, err = b.discover(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	a.setDevices(devices)
	if a.selected != chosen.key() || a.selectedSession() != session || session.cmd.ProcessState != nil {
		t.Fatal("real refresh disturbed running session or selection")
	}
	a.start()
	if a.busy || len(a.sessions) != 1 {
		t.Fatal("refreshed phone could start a duplicate session")
	}
	a.stop(session)
	select {
	case update := <-a.updates:
		update()
	case <-time.After(5 * time.Second):
		t.Fatal("real session did not stop")
	}
	a.workers.Wait()
	if len(a.sessions) != 0 || a.errorText != "" {
		t.Fatalf("real stop failed: %s", a.errorText)
	}
	t.Logf("%d phone/service cards; name %q; all real ADB aliases resolve once; video decoded; refresh/duplicate guard/stop passed", len(devices), chosen.displayName())
}

// Opt in only with an authorized phone. This exercises the GUI session worker,
// including cancellation on exit, instead of just launching scrcpy directly.
func TestLiveDeviceScreenRestoreAndAudio(t *testing.T) {
	serial := os.Getenv("SCRCPY_GUI_DEVICE_TEST_SERIAL")
	if serial == "" {
		t.Skip("set SCRCPY_GUI_DEVICE_TEST_SERIAL for physical-device checks")
	}
	for _, test := range []struct {
		codec, ending string
		bitrate       int
	}{
		{"video", "stop", 0}, {"video", "quit", 0}, {"video", "kill", 0},
		{"opus", "stop", 128}, {"aac", "stop", 128}, {"flac", "stop", 0}, {"raw", "quit", 0}, {"opus", "stop", 9000},
	} {
		t.Run(fmt.Sprintf("%s-%d-%s", test.codec, test.bitrate, test.ending), func(t *testing.T) {
			b, err := newBackend()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			a := newApplication(ctx, b)
			a.logs.setEnabled(true)
			a.playback.AudioOnly, a.playback.AudioCodec = test.codec != "video", test.codec
			a.playback.TurnScreenOff = true
			a.audioPreset, a.customAudio = len(audioPresets), fmt.Sprint(test.bitrate)
			a.useCustomBuffer, a.customBuffer = true, "0.5"
			a.setDevices([]device{{Serial: serial, Address: serial, State: "device"}})
			t.Cleanup(func() { cancel(); a.workers.Wait() })
			a.start()
			select {
			case update := <-a.updates:
				update()
			case <-time.After(15 * time.Second):
				t.Fatal("session start timed out")
			}
			if len(a.sessions) == 0 {
				t.Fatalf("session did not start: %s", a.errorText)
			}
			wait := func(check func() bool) bool {
				deadline := time.Now().Add(10 * time.Second)
				for time.Now().Before(deadline) {
					select {
					case update := <-a.updates:
						update()
					default:
					}
					if check() {
						return true
					}
					time.Sleep(100 * time.Millisecond)
				}
				return false
			}
			decoded := "DEBUG: Decoder 'audio':"
			if test.codec == "video" {
				decoded = "INFO: Texture"
			}
			if !wait(func() bool { return strings.Contains(a.logs.String(), decoded) || len(a.sessions) == 0 }) {
				t.Fatalf("no decoded stream: %s", a.logs.String())
			}
			if len(a.sessions) == 0 {
				if test.bitrate == 9000 && a.errorText != "" {
					t.Logf("9000 Kbps rejected by encoder as expected; GUI reported: %s", a.errorText)
					return
				}
				t.Fatalf("stream exited: %s", a.errorText)
			}
			power := func(state string) bool {
				queryCtx, queryCancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer queryCancel()
				cmd := exec.CommandContext(queryCtx, b.adb, "-s", serial, "shell", "dumpsys", "SurfaceFlinger")
				hideConsole(cmd)
				out, err := cmd.Output()
				return err == nil && strings.Contains(string(out), "powerMode="+state)
			}
			if !wait(func() bool { return power("OFF") }) {
				t.Fatalf("physical display did not turn off: %s", a.logs.String())
			}
			switch test.ending {
			case "quit":
				cancel()
			case "kill":
				if err := a.selectedSession().cmd.Process.Kill(); err != nil {
					t.Fatal(err)
				}
			default:
				a.stop(a.selectedSession())
			}
			a.workers.Wait()
			volumeCtx, volumeCancel := context.WithTimeout(context.Background(), 3*time.Second)
			volume, volumeErr := b.run(volumeCtx, "-s", serial, "shell", "cmd", "media_session", "volume", "--stream", "3", "--get")
			volumeCancel()
			if volumeErr != nil || !strings.Contains(volume, "volume is 0 in range") {
				t.Fatalf("physical media volume was not muted: %s %v", volume, volumeErr)
			}
			if !wait(func() bool { return power("ON") }) {
				t.Fatalf("physical display did not turn on: %s", a.logs.String())
			}
			t.Logf("%s decoded; physical display OFF → ON and media volume 0 after %s; 0.5-second buffer", test.codec, test.ending)
		})
	}
}
