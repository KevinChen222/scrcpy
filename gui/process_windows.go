package main

import (
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"
)

var processJob syscall.Handle

// Keep the non-inheritable handle open until Windows tears down this process.
// Children (including detached ADB servers) inherit job membership, not the
// handle, so even a forced GUI exit closes the last handle and kills the tree.
func setupProcessCleanup() error {
	if processJob != 0 {
		return nil
	}
	kernel := syscall.NewLazyDLL("kernel32.dll")
	job, _, err := kernel.NewProc("CreateJobObjectW").Call(0, 0)
	if job == 0 {
		return fmt.Errorf("创建后台进程组失败: %w", err)
	}
	info := struct {
		ProcessTime, JobTime     int64
		Flags                    uint32
		MinWorking, MaxWorking   uintptr
		ActiveProcesses          uint32
		Affinity                 uintptr
		Priority, Scheduling     uint32
		IO                       [6]uint64
		ProcessMemory, JobMemory uintptr
		PeakProcess, PeakJob     uintptr
	}{Flags: 0x2000} // JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	ok, _, err := kernel.NewProc("SetInformationJobObject").Call(job, 9, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info))
	if ok == 0 {
		syscall.CloseHandle(syscall.Handle(job))
		return fmt.Errorf("设置后台进程清理失败: %w", err)
	}
	current, err := syscall.GetCurrentProcess()
	if err != nil {
		syscall.CloseHandle(syscall.Handle(job))
		return fmt.Errorf("读取当前进程失败: %w", err)
	}
	ok, _, err = kernel.NewProc("AssignProcessToJobObject").Call(job, uintptr(current))
	if ok == 0 {
		syscall.CloseHandle(syscall.Handle(job))
		return fmt.Errorf("启用后台进程清理失败: %w", err)
	}
	processJob = syscall.Handle(job)
	return nil
}

func hideConsole(cmd *exec.Cmd) {
	// Suppress the console without imposing SW_HIDE on scrcpy's SDL window.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}
