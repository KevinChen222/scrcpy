package main

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMultiDeviceSessionsStopAndQuit(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SCRCPY_GUI_TEST_CHILD", "wait")
	ctx, cancel := context.WithCancel(context.Background())
	muted := make(chan string, 3)
	adbStopped := false
	b := &backend{adb: exe, scrcpy: exe, run: func(ctx context.Context, args ...string) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if len(args) == 1 && args[0] == "kill-server" {
			if len(muted) != 1 {
				t.Fatal("ADB stopped before session cleanup finished")
			}
			adbStopped = true
			return "", nil
		}
		if len(args) > 4 && args[3] == "cmd" {
			muted <- args[1]
			return "[V] volume is 0 in range [0..15]", nil
		}
		if len(args) > 4 && args[3] == "getprop" {
			return args[1], nil
		}
		if len(args) > 4 && args[3] == "settings" {
			return "null\n", nil
		}
		return "device\n", nil
	}}
	a := newApplication(ctx, b)
	a.wired = true
	a.setDevices([]device{{Serial: "phone-a", State: "device"}, {Serial: "phone-b", State: "device"}, {Serial: "phone-c", State: "device"}})
	t.Cleanup(func() { cancel(); a.workers.Wait() })
	applyUpdate := func() {
		t.Helper()
		select {
		case update := <-a.updates:
			update()
		case <-time.After(3 * time.Second):
			t.Fatal("missing session update")
		}
	}
	for i, d := range a.devices {
		a.selected = d.key()
		a.playback.AudioOnly = i == 1
		a.playback.MuteOnStop = i != 2
		a.start()
		applyUpdate()
		if len(a.sessions) != i+1 {
			t.Fatalf("session %d not added: %s", i, a.errorText)
		}
	}
	if a.sessions[0].playback.AudioOnly || !a.sessions[1].playback.AudioOnly || a.sessions[0].playback.Frames == a.sessions[2].playback.Frames {
		t.Fatal("independent playback modes or frame metadata were shared")
	}
	a.start()
	if a.busy || len(a.sessions) != 3 {
		t.Fatal("same device was started twice")
	}
	first := a.sessions[0]
	a.stop(first)
	applyUpdate()
	if len(a.sessions) != 2 || a.sessions[0].serial != "phone-b" || a.sessions[0].stopping || a.sessions[0].cmd.ProcessState != nil || a.errorText != "" {
		t.Fatalf("stopping one device disturbed another: %s", a.errorText)
	}
	if serial := <-muted; serial != "phone-a" {
		t.Fatalf("wrong device muted: %s", serial)
	}
	a.shutdown(cancel)
	if !adbStopped {
		t.Fatal("GUI exit left the ADB server running")
	}
	if serial := <-muted; serial != "phone-b" {
		t.Fatalf("quit muted wrong device: %s", serial)
	}
	select {
	case serial := <-muted:
		t.Fatalf("opted-out device was muted: %s", serial)
	default:
	}
	for _, session := range a.sessions {
		if session.cmd.ProcessState == nil {
			t.Fatal("GUI exit did not reap all processes")
		}
	}
}

func TestMediaMuteChecksOutputAndTarget(t *testing.T) {
	for _, output := range []string{"[V] volume is 0 in range [0..15]", "[V] volume is 8 in range [0..15]", "Error: Permission denied", ""} {
		b := &backend{run: func(ctx context.Context, args ...string) (string, error) {
			if ctx.Err() != nil || !reflect.DeepEqual(args, []string{"-s", "own-phone", "shell", "cmd", "media_session", "volume", "--stream", "3", "--set", "0", "--get"}) {
				t.Fatalf("wrong media mute context/arguments: %v", args)
			}
			return output, nil
		}}
		if err := b.muteMedia("own-phone"); (err == nil) != strings.Contains(output, "volume is 0 in range") {
			t.Fatalf("output %q: %v", output, err)
		}
	}
	b := &backend{run: func(context.Context, ...string) (string, error) { return "", fmt.Errorf("offline") }}
	if err := b.muteMedia("own-phone"); err == nil {
		t.Fatal("mute command failure was ignored")
	}
	if !newApplication(context.Background(), nil).playback.MuteOnStop {
		t.Fatal("mute-on-stop must be enabled by default")
	}
}
