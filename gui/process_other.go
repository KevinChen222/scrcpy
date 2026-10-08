//go:build !windows

package main

import "os/exec"

func setupProcessCleanup() error { return nil }

func hideConsole(cmd *exec.Cmd) {}
