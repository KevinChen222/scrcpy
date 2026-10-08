package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

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
