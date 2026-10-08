package main

import (
	"context"
	"fmt"
	"image"
	"os"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// Opt-in integration check against the bundled scrcpy and an authorized phone.
// It changes only the window and stops the session it started.
func checkNativeMirrorControls() error {
	if proc := mirrorUser32.NewProc("SetThreadDpiAwarenessContext"); proc.Find() == nil {
		previous, _, _ := proc.Call(^uintptr(3))
		if previous != 0 {
			defer proc.Call(previous)
		}
	}
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
	playback.Frames = &mirrorFrames{}
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
	go func() { hookDone <- runMirrorWindow(ctx, cmd.Process.Pid, playback, serial, cancel) }()
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
	hostTitle, _ := syscall.UTF16PtrFromString("scrcpy LAN · " + serial + " · Alt+Z 缩放")
	findWindow := mirrorUser32.NewProc("FindWindowW")
	var window uintptr
	if !wait(func() bool {
		window, _, _ = findWindow.Call(0, uintptr(unsafe.Pointer(hostTitle)))
		child, _, _ := mirrorUser32.NewProc("FindWindowExW").Call(window, 0, 0, uintptr(unsafe.Pointer(title)))
		var pid uint32
		mirrorWindowPID.Call(child, uintptr(unsafe.Pointer(&pid)))
		var owner uint32
		mirrorWindowPID.Call(window, uintptr(unsafe.Pointer(&owner)))
		visible, _, _ := mirrorUser32.NewProc("IsWindowVisible").Call(window)
		return window != 0 && visible != 0 && owner == uint32(os.Getpid()) && pid == uint32(cmd.Process.Pid) && strings.Contains(output.String(), "INFO: Texture") && mirrorIsFullscreen(window) == playback.Fullscreen
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
	background := os.Getenv("SCRCPY_GUI_MIRROR_BACKGROUND_TEST") == "1"
	// A title-bar click grants foreground permission without tapping the phone.
	if playback.Fullscreen {
		mirrorUser32.NewProc("SendMessageW").Call(window, 0x8001, 1, 0)
	}
	if !background {
		var focusBounds mirrorRect
		mirrorWindowRect.Call(window, uintptr(unsafe.Pointer(&focusBounds)))
		cursor := struct{ X, Y int32 }{}
		mirrorUser32.NewProc("GetCursorPos").Call(uintptr(unsafe.Pointer(&cursor)))
		mirrorUser32.NewProc("SetCursorPos").Call(uintptr(focusBounds.Left+100), uintptr(focusBounds.Top+12))
		mouse := mirrorUser32.NewProc("mouse_event")
		mouse.Call(2, 0, 0, 0, 0)
		mouse.Call(4, 0, 0, 0, 0)
		mirrorUser32.NewProc("SetCursorPos").Call(uintptr(cursor.X), uintptr(cursor.Y))
	}
	focused := func() bool {
		current, _, _ := mirrorForeground.Call()
		root, _, _ := mirrorUser32.NewProc("GetAncestor").Call(current, 2)
		return current == window || root == window
	}
	if !background && !wait(focused) {
		current, _, _ := mirrorForeground.Call()
		var bounds mirrorRect
		mirrorWindowRect.Call(window, uintptr(unsafe.Pointer(&bounds)))
		visible, _, _ := mirrorUser32.NewProc("IsWindowVisible").Call(window)
		enabled, _, _ := mirrorUser32.NewProc("IsWindowEnabled").Call(window)
		root, _, _ := mirrorUser32.NewProc("GetAncestor").Call(current, 2)
		parent, _, _ := mirrorUser32.NewProc("GetParent").Call(current)
		currentStyle, _, _ := mirrorWindowStyle.Call(current, ^uintptr(15))
		var currentPID uint32
		mirrorWindowPID.Call(current, uintptr(unsafe.Pointer(&currentPID)))
		name := make([]uint16, 256)
		mirrorUser32.NewProc("GetWindowTextW").Call(current, uintptr(unsafe.Pointer(&name[0])), 256)
		return fmt.Errorf("无法聚焦投屏，host=%x foreground=%x root=%x parent=%x style=%x pid=%d (%s) visible=%d enabled=%d rect=%+v", window, current, root, parent, currentStyle, currentPID, syscall.UTF16ToString(name), visible, enabled, bounds)
	}
	toggle := func() {
		if background {
			mirrorUser32.NewProc("SendMessageW").Call(window, 0x8001, 1, 0)
			return
		}
		key := mirrorUser32.NewProc("keybd_event")
		key.Call(0x7a, 0x57, 0, 0)
		key.Call(0x7a, 0x57, 2, 0)
	}
	toggle()
	if !wait(func() bool { return mirrorIsFullscreen(window) }) {
		return fmt.Errorf("F11 未进入全屏")
	}
	if _, found := mirrorViews.Load(window); !found {
		return fmt.Errorf("投屏快捷键未安装")
	}
	key := mirrorUser32.NewProc("keybd_event")
	if !background && !focused() {
		return fmt.Errorf("投屏失去焦点，未发送测试按键")
	}
	if background {
		mirrorUser32.NewProc("SendMessageW").Call(window, 0x8001, 3, 0)
	} else {
		key.Call(0x1b, 1, 0, 0)
		time.Sleep(150 * time.Millisecond)
		key.Call(0x1b, 1, 0, 0) // Holding Esc must not toggle back into fullscreen.
		key.Call(0x1b, 1, 2, 0)
	}
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
	child, _, _ := mirrorUser32.NewProc("FindWindowExW").Call(window, 0, 0, uintptr(unsafe.Pointer(title)))
	for i, mode := range scaleModes {
		mirrorUser32.NewProc("SendMessageW").Call(window, 0x111, uintptr(100+i), 0)
		crop := image.Rectangle{}
		frameW, frameH := playback.Frames.size()
		if !wait(func() bool {
			var viewport, rendered, bounds mirrorRect
			mirrorClientRect.Call(window, uintptr(unsafe.Pointer(&viewport)))
			mirrorClientRect.Call(child, uintptr(unsafe.Pointer(&rendered)))
			mirrorWindowRect.Call(child, uintptr(unsafe.Pointer(&bounds)))
			mirrorUser32.NewProc("MapWindowPoints").Call(0, window, uintptr(unsafe.Pointer(&bounds)), 2)
			want := scaledMirrorRect(int(viewport.Right), int(viewport.Bottom), frameW, frameH, crop, scaleMode(mode))
			return int(rendered.Right) == want.W && int(rendered.Bottom) == want.H && int(bounds.Left) == want.X && int(bounds.Top) == want.Y
		}) {
			var viewport, rendered, bounds mirrorRect
			mirrorClientRect.Call(window, uintptr(unsafe.Pointer(&viewport)))
			mirrorClientRect.Call(child, uintptr(unsafe.Pointer(&rendered)))
			mirrorWindowRect.Call(child, uintptr(unsafe.Pointer(&bounds)))
			mirrorUser32.NewProc("MapWindowPoints").Call(0, window, uintptr(unsafe.Pointer(&bounds)), 2)
			childStyle, _, _ := mirrorWindowStyle.Call(child, ^uintptr(15))
			return fmt.Errorf("缩放模式 %s: viewport=%+v rendered=%+v bounds=%+v style=%x want=%+v", mode, viewport, rendered, bounds, childStyle, scaledMirrorRect(int(viewport.Right), int(viewport.Bottom), frameW, frameH, crop, scaleMode(mode)))
		}
	}
	controls := "physical Esc/F11"
	if background {
		controls = "window messages (foreground keyboard skipped)"
	}
	if err := checkNativeSelection(window, child, playback.Frames); err != nil {
		return err
	}
	fmt.Printf("custom 1280px / 90fps / 8Mbps, buffer=%d fullscreen=%v: decoded; %s passed; three scale modes and manual selection geometry passed\n", playback.BufferMS, playback.Fullscreen, controls)
	return nil
}

func checkNativeSelection(window, child uintptr, frames *mirrorFrames) error {
	send := mirrorUser32.NewProc("SendMessageW")
	send.Call(window, 0x8001, 1, 0) // fullscreen
	send.Call(window, 0x111, 200, 0)
	overlay, _, _ := mirrorUser32.NewProc("FindWindowExW").Call(window, 0, 0, uintptr(unsafe.Pointer(mirrorText("选择显示区域"))))
	if overlay == 0 {
		return fmt.Errorf("全屏选区遮罩未创建")
	}
	var viewport mirrorRect
	mirrorClientRect.Call(window, uintptr(unsafe.Pointer(&viewport)))
	w, h := frames.size()
	r := scaledMirrorRect(int(viewport.Right), int(viewport.Bottom), w, h, image.Rectangle{}, scaleFit)
	selection := image.Rect(r.X+r.W/5, r.Y+r.H/5, r.X+r.W*3/4, r.Y+r.H*4/5)
	point := func(p image.Point) uintptr { return uintptr(uint16(p.X)) | uintptr(uint16(p.Y))<<16 }
	if os.Getenv("SCRCPY_GUI_MIRROR_BACKGROUND_TEST") == "1" {
		send.Call(overlay, 0x201, 1, point(selection.Min))
		send.Call(overlay, 0x200, 1, point(selection.Max))
		send.Call(overlay, 0x202, 0, point(selection.Max))
	} else {
		start := struct{ X, Y int32 }{int32(selection.Min.X), int32(selection.Min.Y)}
		end := struct{ X, Y int32 }{int32(selection.Max.X), int32(selection.Max.Y)}
		mirrorUser32.NewProc("ClientToScreen").Call(overlay, uintptr(unsafe.Pointer(&start)))
		mirrorUser32.NewProc("ClientToScreen").Call(overlay, uintptr(unsafe.Pointer(&end)))
		hit, _, _ := mirrorUser32.NewProc("WindowFromPoint").Call(uintptr(uint32(start.X)) | uintptr(uint32(start.Y))<<32)
		if hit != overlay {
			return fmt.Errorf("选区未拦截鼠标，overlay=%x hit=%x", overlay, hit)
		}
		cursor := struct{ X, Y int32 }{}
		mirrorUser32.NewProc("GetCursorPos").Call(uintptr(unsafe.Pointer(&cursor)))
		defer mirrorUser32.NewProc("SetCursorPos").Call(uintptr(cursor.X), uintptr(cursor.Y))
		mouse := mirrorUser32.NewProc("mouse_event")
		mirrorUser32.NewProc("SetCursorPos").Call(uintptr(start.X), uintptr(start.Y))
		mouse.Call(2, 0, 0, 0, 0)
		time.Sleep(50 * time.Millisecond)
		mirrorUser32.NewProc("SetCursorPos").Call(uintptr(end.X), uintptr(end.Y))
		mouse.Call(4, 0, 0, 0, 0)
		time.Sleep(100 * time.Millisecond)
	}
	crop := selectedMirrorCrop(selection, r, w, h)
	want := scaledMirrorRect(int(viewport.Right), int(viewport.Bottom), w, h, crop, scaleManual)
	var bounds mirrorRect
	mirrorWindowRect.Call(child, uintptr(unsafe.Pointer(&bounds)))
	mirrorUser32.NewProc("MapWindowPoints").Call(0, window, uintptr(unsafe.Pointer(&bounds)), 2)
	if int(bounds.Left) != want.X || int(bounds.Top) != want.Y || int(bounds.Right-bounds.Left) != want.W || int(bounds.Bottom-bounds.Top) != want.H {
		return fmt.Errorf("手动选区坐标不符: got=%+v want=%+v", bounds, want)
	}
	send.Call(window, 0x111, 200, 0) // reselect; Escape keeps existing crop
	send.Call(window, 0x8001, 3, 0)
	if !mirrorIsFullscreen(window) {
		return fmt.Errorf("取消选区意外退出全屏")
	}
	var restored mirrorRect
	mirrorWindowRect.Call(child, uintptr(unsafe.Pointer(&restored)))
	mirrorUser32.NewProc("MapWindowPoints").Call(0, window, uintptr(unsafe.Pointer(&restored)), 2)
	if restored != bounds {
		return fmt.Errorf("取消重选未保留原区域")
	}
	send.Call(window, 0x111, 201, 0) // reset
	mirrorWindowRect.Call(child, uintptr(unsafe.Pointer(&restored)))
	mirrorUser32.NewProc("MapWindowPoints").Call(0, window, uintptr(unsafe.Pointer(&restored)), 2)
	if int(restored.Left) != r.X || int(restored.Top) != r.Y || int(restored.Right-restored.Left) != r.W || int(restored.Bottom-restored.Top) != r.H {
		return fmt.Errorf("选区重置未恢复完整画面")
	}
	send.Call(window, 0x8001, 3, 0)
	return nil
}
