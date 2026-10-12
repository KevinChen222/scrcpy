package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestLinuxCancellationOwnsProcessGroup(t *testing.T) {
	unrelated := exec.Command("sleep", "30")
	if err := unrelated.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { unrelated.Process.Kill(); unrelated.Wait() }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	file := filepath.Join(t.TempDir(), "child.pid")
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", `sleep 30 & echo $! > "$1"; wait`, "group-test", file)
	hideConsole(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	var pid int
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(file)
		if err == nil {
			pid, _ = strconv.Atoi(strings.TrimSpace(string(data)))
			if pid != 0 {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pid == 0 {
		cancel()
		cmd.Wait()
		t.Fatal("descendant did not start")
	}
	if group, err := syscall.Getpgid(pid); err != nil || group != cmd.Process.Pid {
		cancel()
		cmd.Wait()
		t.Fatalf("child is outside session process group: %d %v", group, err)
	}
	cancel()
	cmd.Wait()
	deadline = time.Now().Add(3 * time.Second)
	for {
		data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
		// A zombie no longer executes; its adopted parent owns the remaining reap.
		fields := strings.Fields(string(data))
		if errors.Is(err, os.ErrNotExist) || (len(fields) > 2 && fields[2] == "Z") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("cancellation left a descendant running")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := unrelated.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("cancellation disturbed an unrelated process: %v", err)
	}
}

func TestLinuxRuntimeEnvironment(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	client := filepath.Join(dir, "scrcpy")
	if err := os.Link(exe, client); err != nil {
		t.Fatal(err)
	}
	server := filepath.Join(dir, "scrcpy-server")
	t.Setenv("SCRCPY_GUI_TEST_CHILD", "exit")
	for _, test := range []struct {
		name, override string
		portable       bool
	}{
		{"system defaults", "", false}, {"portable server", "", true}, {"explicit override", "/custom/server", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("SCRCPY_SERVER_PATH", test.override)
			if test.portable {
				if err := os.WriteFile(server, []byte("test server"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			b := &backend{adb: "/chosen/adb", scrcpy: client}
			cmd, _, err := b.start(context.Background(), "phone", presets[0], playbackOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Wait(); err != nil {
				t.Fatal(err)
			}
			env := map[string]string{}
			for _, entry := range cmd.Env {
				key, value, ok := strings.Cut(entry, "=")
				if ok {
					env[key] = value
				}
			}
			expected := test.override
			if expected == "" && test.portable {
				expected = server
			}
			if env["SCRCPY_SERVER_PATH"] != expected || env["ADB"] != "/chosen/adb" || env["SDL_VIDEODRIVER"] != "x11" {
				t.Fatalf("incorrect runtime handoff: ADB=%q server=%q driver=%q", env["ADB"], env["SCRCPY_SERVER_PATH"], env["SDL_VIDEODRIVER"])
			}
		})
	}
}

func TestLinuxMirrorRequiresRealtimeStdout(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SCRCPY_GUI_TEST_CHILD", "exit")
	b := &backend{scrcpy: exe, adb: "/chosen/adb"}
	for _, playback := range []playbackOptions{{}, {AudioOnly: true}, {Frames: &mirrorFrames{}}} {
		cmd, _, err := b.start(context.Background(), "phone", presets[0], playback)
		if err != nil {
			t.Fatal(err)
		}
		if err := cmd.Wait(); err != nil {
			t.Fatal(err)
		}
		if playback.Frames != nil {
			if filepath.Base(cmd.Path) != "stdbuf" || len(cmd.Args) < 4 || cmd.Args[1] != "-oL" || cmd.Args[2] != exe {
				t.Fatalf("embedded video stdout is still buffered: %v", cmd.Args)
			}
		} else if cmd.Path != exe {
			t.Fatalf("changed standalone/audio launch: %v", cmd.Args)
		}
	}
	// Report a missing dependency before starting the video client, rather than
	// letting its visible window be killed by the attachment timeout later.
	t.Setenv("PATH", t.TempDir())
	if _, _, err := b.start(context.Background(), "phone", presets[0], playbackOptions{Frames: &mirrorFrames{}}); err == nil || !strings.Contains(err.Error(), "stdbuf") {
		t.Fatalf("missing line-buffering tool did not fail at startup: %v", err)
	}
}
