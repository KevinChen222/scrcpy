package main

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func setupProcessCleanup() error { return nil }

// Own a separate process group; cancellation must not signal the GUI, a shared
// ADB server, or mirrors started by another application. The parent-death
// signal also prevents the direct child surviving a forced GUI termination.
func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	if cmd.Cancel != nil {
		cmd.Cancel = func() error {
			err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			if errors.Is(err, syscall.ESRCH) {
				return os.ErrProcessDone
			}
			return err
		}
	}
}
