package main

import (
	"context"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

func TestCustomBufferValidation(t *testing.T) {
	for value, want := range map[string]int{"0": 0, "0.5": 500, " 1.735 ": 1735, "2": 2000, "60": 60000} {
		got, err := customBufferMS(value)
		if err != nil || got != want || bufferSeconds(got) != strings.TrimSpace(value) {
			t.Errorf("%q: %d, %v", value, got, err)
		}
	}
	for _, value := range []string{"", "-1", "60.001", "0.0005", "NaN", "Inf", "1s", "1;echo"} {
		if _, err := customBufferMS(value); err == nil {
			t.Errorf("accepted invalid buffer %q", value)
		}
	}
	a := newApplication(context.Background(), &backend{})
	a.wired, a.useCustomBuffer, a.customBuffer = true, true, "invalid"
	a.setDevices([]device{{Serial: "usb-phone", State: "device"}})
	a.start()
	if a.busy || len(a.sessions) > 0 || a.errorText == "" {
		t.Fatal("invalid custom buffer launched a session")
	}
}

func TestAudioCodecArguments(t *testing.T) {
	for _, codec := range []string{"opus", "aac", "flac", "raw"} {
		playback := playbackOptions{AudioOnly: true, AudioCodec: codec, AudioBitrateKbps: 9000, BufferMS: 500}
		args := strings.Join(presets[0].args("phone", playback), " ")
		if !strings.Contains(args, "--audio-codec="+codec) || strings.Contains(args, "--audio-bit-rate=") == playback.losslessAudio() {
			t.Fatalf("incorrect audio codec arguments: %s", args)
		}
	}
}

func TestLogCollectionOnlyWhileEnabled(t *testing.T) {
	l := &sessionLog{}
	l.printf("hidden")
	if l.String() != "" || len(l.data) != 0 {
		t.Fatal("closed log window retained diagnostics")
	}
	l.setEnabled(true)
	var workers sync.WaitGroup
	for range 4 {
		workers.Go(func() {
			for range 100 {
				l.printf("INFO 中文日志")
			}
		})
	}
	workers.Wait()
	if !strings.Contains(l.String(), "INFO 中文日志") {
		t.Fatal("enabled log lost concurrent output")
	}
	l.Write([]byte(strings.Repeat("中文", 20000)))
	if len(l.data) > 65536 || !utf8.Valid(l.data) {
		t.Fatal("log limit corrupted UTF-8 or grew past its bound")
	}
	l.setEnabled(false)
	l.Write([]byte("ERROR after close"))
	if l.String() != "" || l.data != nil {
		t.Fatal("closing the log did not clear and stop it")
	}
	l.setEnabled(true)
	l.printf("new window")
	if strings.Contains(l.String(), "after close") {
		t.Fatal("reopening showed historical diagnostics")
	}
}

func TestADBLogsHidePairingCode(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SCRCPY_GUI_TEST_CHILD", "echo")
	l := &sessionLog{}
	l.setEnabled(true)
	b := &backend{adb: exe, logs: l}
	if _, err := b.runADB(context.Background(), "pair", "192.168.1.10:40000", "012345"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(l.String(), "012345") || !strings.Contains(l.String(), "[配对码已隐藏]") {
		t.Fatalf("pairing code leaked: %s", l.String())
	}
}

func TestScreenRestoreForEverySessionExit(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, audioOnly := range []bool{false, true} {
		for _, ending := range []string{"stop", "quit", "exit", "fail"} {
			t.Run(strings.Join([]string{map[bool]string{false: "video", true: "audio"}[audioOnly], ending}, "/"), func(t *testing.T) {
				mode := ending
				if ending == "stop" || ending == "quit" {
					mode = "wait"
				}
				t.Setenv("SCRCPY_GUI_TEST_CHILD", mode)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				restored := make(chan []string, 1)
				b := &backend{adb: exe, scrcpy: exe, run: func(ctx context.Context, args ...string) (string, error) {
					if len(args) > 4 && args[3] == "cmd" && args[4] == "media_session" {
						return "[V] volume is 0 in range [0..15]", nil
					}
					if len(args) > 3 && args[3] == "input" {
						if ctx.Err() != nil {
							return "", ctx.Err()
						}
						restored <- args
					}
					return "device\n", nil
				}}
				a := newApplication(ctx, b)
				a.wired = true
				a.playback.AudioOnly, a.playback.TurnScreenOff = audioOnly, true
				a.useCustomBuffer, a.customBuffer = true, "0.5"
				a.setDevices([]device{{Serial: "usb-phone", State: "device"}})
				a.start()
				select {
				case update := <-a.updates:
					update()
				case <-time.After(3 * time.Second):
					t.Fatal("session did not start")
				}
				if !strings.Contains(a.detail, "缓存 0.5 秒") {
					t.Fatalf("fractional cache display: %s", a.detail)
				}
				if ending == "stop" {
					a.stop(a.selectedSession())
				}
				if ending == "quit" {
					cancel()
				}
				select {
				case args := <-restored:
					if !reflect.DeepEqual(args, []string{"-s", "usb-phone", "shell", "input", "keyevent", "KEYCODE_WAKEUP"}) {
						t.Fatalf("restore args: %v", args)
					}
				case <-time.After(3 * time.Second):
					t.Fatal("session ended without waking its own device")
				}
				a.workers.Wait()
				if ending != "quit" {
					select {
					case update := <-a.updates:
						update()
					case <-time.After(time.Second):
						t.Fatal("missing end update")
					}
					if len(a.sessions) > 0 || !strings.Contains(a.detail, "恢复设备亮屏") {
						t.Fatalf("incomplete cleanup: %s", a.detail)
					}
					if !strings.Contains(a.detail, "媒体声音已静音") {
						t.Fatal("session exit did not report media mute")
					}
					if ending == "fail" && a.errorText == "" {
						t.Fatal("capture error was not shown with logging disabled")
					}
				}
			})
		}
	}
}

func TestScreenRestoreReappliesPowerOnlyWhenStillOff(t *testing.T) {
	for _, state := range []string{"ON", "OFF", "0"} {
		t.Run(state, func(t *testing.T) {
			var calls [][]string
			b := &backend{run: func(_ context.Context, args ...string) (string, error) {
				calls = append(calls, args)
				return "powerMode=" + state, nil
			}}
			if err := b.restoreScreen("own-phone"); err != nil {
				t.Fatal(err)
			}
			wantCalls := 2
			if state != "ON" {
				wantCalls = 3
			}
			if len(calls) != wantCalls {
				t.Fatalf("unexpected recovery calls: %v", calls)
			}
			if state != "ON" && !reflect.DeepEqual(calls[2], []string{"-s", "own-phone", "shell", "input", "keyevent", "KEYCODE_SLEEP", "KEYCODE_WAKEUP"}) {
				t.Fatalf("still-off display was not recovered: %v", calls)
			}
		})
	}
}
