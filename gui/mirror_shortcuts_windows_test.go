package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// Opt-in integration check against the bundled scrcpy and an authorized phone.
// It changes only the window and stops the session it started.
func checkNativeMirrorControls() error {
	serial := os.Getenv("SCRCPY_GUI_NATIVE_SERIAL")
	if serial == "" {
		return fmt.Errorf("set SCRCPY_GUI_NATIVE_SERIAL to an authorized device")
	}
	for _, playback := range []playbackOptions{{}, {BufferMS: 2000, Fullscreen: true}, {BufferMS: 3000, Fullscreen: true}} {
		if err := checkNativeMirrorSession(serial, playback); err != nil {
			return fmt.Errorf("buffer=%d fullscreen=%v: %w", playback.BufferMS, playback.Fullscreen, err)
		}
	}
	return nil
}

func checkNativeMirrorSession(serial string, playback playbackOptions) error {
	b, err := newBackend()
	if err != nil {
		return err
	}
	b.logs = &sessionLog{}
	b.logs.setEnabled(true)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	p, err := (customPreset{"1280", "90", "8"}).preset()
	if err != nil {
		return err
	}
	cmd, output, err := b.start(ctx, serial, p, playback)
	if err != nil {
		return err
	}
	hookDone := make(chan error, 1)
	go func() { hookDone <- runMirrorShortcuts(ctx, cmd.Process.Pid) }()
	defer func() {
		cancel()
		cmd.Wait()
		<-hookDone
	}()
	wait := func(check func() bool) bool {
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) && ctx.Err() == nil {
			if check() {
				return true
			}
			time.Sleep(20 * time.Millisecond)
		}
		return false
	}
	title, _ := syscall.UTF16PtrFromString("scrcpy LAN · " + serial)
	findWindow := mirrorUser32.NewProc("FindWindowW")
	var window uintptr
	if !wait(func() bool {
		window, _, _ = findWindow.Call(0, uintptr(unsafe.Pointer(title)))
		var pid uint32
		mirrorWindowPID.Call(window, uintptr(unsafe.Pointer(&pid)))
		return window != 0 && pid == uint32(cmd.Process.Pid) && strings.Contains(output.String(), "INFO: Texture")
	}) {
		return fmt.Errorf("投屏未显示并解码: %s", output.String())
	}
	visible, _, _ := mirrorUser32.NewProc("IsWindowVisible").Call(window)
	if visible == 0 {
		return fmt.Errorf("投屏窗口被启动参数隐藏")
	}
	style, _, _ := mirrorWindowStyle.Call(window, ^uintptr(15))
	if mirrorIsFullscreen(window) != playback.Fullscreen || !playback.Fullscreen && style&0x00c00000 == 0 {
		return fmt.Errorf("投屏启动模式不符: fullscreen=%v", playback.Fullscreen)
	}
	mirrorUser32.NewProc("SetForegroundWindow").Call(window)
	if !wait(func() bool { current, _, _ := mirrorForeground.Call(); return current == window }) {
		return fmt.Errorf("无法聚焦本次测试投屏窗口，请在解锁桌面运行原生检查")
	}
	toggle := func() {
		mirrorPostMessage.Call(window, 0x100, 0x7a, 0x00570001)
		mirrorPostMessage.Call(window, 0x101, 0x7a, 0xc0570001)
	}
	if !playback.Fullscreen {
		toggle()
	}
	if !wait(func() bool { return mirrorIsFullscreen(window) }) {
		return fmt.Errorf("F11 未进入全屏")
	}
	if !wait(func() bool { state := mirrorKeys.Load(); return state != nil && state.pid == uint32(cmd.Process.Pid) }) {
		return fmt.Errorf("投屏快捷键未安装")
	}
	key := mirrorUser32.NewProc("keybd_event")
	current, _, _ := mirrorForeground.Call()
	if current != window {
		return fmt.Errorf("投屏失去焦点，未发送测试按键")
	}
	key.Call(0x1b, 1, 0, 0)
	time.Sleep(150 * time.Millisecond)
	key.Call(0x1b, 1, 0, 0) // Holding Esc must not toggle back into fullscreen.
	key.Call(0x1b, 1, 2, 0)
	if !wait(func() bool { return !mirrorIsFullscreen(window) }) {
		return fmt.Errorf("Esc 未退出全屏")
	}
	style, _, _ = mirrorWindowStyle.Call(window, ^uintptr(15))
	if style&0x00c00000 == 0 {
		return fmt.Errorf("Esc 后未恢复普通窗口标题栏")
	}
	toggle()
	if !wait(func() bool { return mirrorIsFullscreen(window) }) {
		return fmt.Errorf("Esc 后 F11 未能重新进入全屏")
	}
	toggle()
	if !wait(func() bool { return !mirrorIsFullscreen(window) }) {
		return fmt.Errorf("F11 未能还原普通窗口")
	}
	fmt.Printf("custom 1280px / 90fps / 8Mbps, buffer=%d fullscreen=%v: decoded; Esc restored title bar; F11 round trip passed\n", playback.BufferMS, playback.Fullscreen)
	return nil
}
