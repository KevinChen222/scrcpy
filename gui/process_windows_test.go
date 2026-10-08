package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// Each owner is a separate process: assigning the test runner to a kill-on-close
// job would also kill the unrelated process used to check cleanup scope.
func init() {
	mode := os.Getenv("SCRCPY_GUI_PROCESS_TEST")
	if mode == "" {
		return
	}
	dir := os.Getenv("SCRCPY_GUI_PROCESS_DIR")
	exe, _ := os.Executable()
	fail := func(err error) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	switch mode {
	case "owner":
		if err := setupProcessCleanup(); err != nil {
			fail(err)
		}
		// The intermediate client exits while its detached server keeps running.
		cmd := exec.Command(exe)
		cmd.Env = append(os.Environ(), "SCRCPY_GUI_PROCESS_TEST=spawn")
		if out, err := cmd.CombinedOutput(); err != nil {
			fail(fmt.Errorf("spawn: %s: %w", out, err))
		}
		data, err := os.ReadFile(filepath.Join(dir, "descendant"))
		if err != nil {
			fail(err)
		}
		pid, err := strconv.Atoi(string(data))
		if err != nil {
			fail(err)
		}
		cmd = exec.Command(exe)
		cmd.Env = append(os.Environ(), "SCRCPY_GUI_PROCESS_TEST=wait")
		hideConsole(cmd)
		if err := cmd.Start(); err != nil {
			fail(err)
		}
		data, _ = json.Marshal([]int{cmd.Process.Pid, pid})
		if err := os.WriteFile(filepath.Join(dir, "ready"), data, 0600); err != nil {
			fail(err)
		}
		for {
			if _, err := os.Stat(filepath.Join(dir, "quit")); err == nil {
				os.Exit(0)
			}
			time.Sleep(10 * time.Millisecond)
		}
	case "spawn":
		cmd := exec.Command(exe)
		cmd.Env = append(os.Environ(), "SCRCPY_GUI_PROCESS_TEST=wait")
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000008} // DETACHED_PROCESS, like adb
		if err := cmd.Start(); err != nil {
			fail(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "descendant"), []byte(strconv.Itoa(cmd.Process.Pid)), 0600); err != nil {
			fail(err)
		}
	case "wait":
		time.Sleep(30 * time.Second)
	}
	os.Exit(0)
}

func TestWindowsProcessTreeCleanup(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, ending := range []string{"quit", "forced"} {
		t.Run(ending, func(t *testing.T) {
			dir := t.TempDir()
			start := func(mode string) *exec.Cmd {
				t.Helper()
				cmd := exec.Command(exe)
				cmd.Env = append(os.Environ(), "SCRCPY_GUI_PROCESS_TEST="+mode, "SCRCPY_GUI_PROCESS_DIR="+dir)
				hideConsole(cmd)
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
				return cmd
			}
			unrelated := start("wait")
			owner := start("owner")
			var data []byte
			deadline := time.Now().Add(5 * time.Second)
			for {
				data, err = os.ReadFile(filepath.Join(dir, "ready"))
				if err == nil {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("process tree startup timed out")
				}
				time.Sleep(10 * time.Millisecond)
			}
			var pids []int
			if err := json.Unmarshal(data, &pids); err != nil || len(pids) != 2 {
				t.Fatalf("invalid child PIDs: %s: %v", data, err)
			}
			handles := make([]syscall.Handle, len(pids))
			for i, pid := range pids {
				handle, err := syscall.OpenProcess(0x00100001, false, uint32(pid)) // SYNCHRONIZE | PROCESS_TERMINATE
				if err != nil {
					t.Fatal(err)
				}
				handles[i] = handle
				t.Cleanup(func() { syscall.TerminateProcess(handle, 1); syscall.CloseHandle(handle) })
				if state, err := syscall.WaitForSingleObject(handle, 0); err != nil || state != syscall.WAIT_TIMEOUT {
					t.Fatalf("child %d not running: %d: %v", pid, state, err)
				}
			}
			if ending == "forced" {
				if err := owner.Process.Kill(); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(filepath.Join(dir, "quit"), nil, 0600); err != nil {
				t.Fatal(err)
			}
			for i, handle := range handles {
				if state, err := syscall.WaitForSingleObject(handle, 5000); err != nil || state != syscall.WAIT_OBJECT_0 {
					t.Fatalf("child %d survived %s: %d: %v", pids[i], ending, state, err)
				}
			}
			handle, err := syscall.OpenProcess(0x00100000, false, uint32(unrelated.Process.Pid))
			if err != nil {
				t.Fatal(err)
			}
			defer syscall.CloseHandle(handle)
			if state, err := syscall.WaitForSingleObject(handle, 0); err != nil || state != syscall.WAIT_TIMEOUT {
				t.Fatalf("cleanup affected unrelated process: %d: %v", state, err)
			}
		})
	}
}
