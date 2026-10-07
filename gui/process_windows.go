package main

import (
	"os/exec"
	"syscall"
)

func hideConsole(cmd *exec.Cmd) {
	// Suppress the console without imposing SW_HIDE on scrcpy's SDL window.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}
