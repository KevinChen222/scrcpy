package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"math"
	"os/exec"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

var (
	mirrorCreateWindow      = mirrorUser32.NewProc("CreateWindowExW")
	mirrorDestroyWindow     = mirrorUser32.NewProc("DestroyWindow")
	mirrorDefaultProc       = mirrorUser32.NewProc("DefWindowProcW")
	mirrorSetStyle          = mirrorUser32.NewProc("SetWindowLongPtrW")
	mirrorSetPosition       = mirrorUser32.NewProc("SetWindowPos")
	mirrorClientRect        = mirrorUser32.NewProc("GetClientRect")
	mirrorSetMenu           = mirrorUser32.NewProc("SetMenu")
	mirrorActiveView        atomic.Pointer[mirrorView]
	mirrorViewCallback      = syscall.NewCallback(mirrorViewProc)
	mirrorFindChildCallback = syscall.NewCallback(func(window, _ uintptr) uintptr {
		v := mirrorActiveView.Load()
		if v == nil {
			return 0
		}
		var owner uint32
		mirrorWindowPID.Call(window, uintptr(unsafe.Pointer(&owner)))
		visible, _, _ := mirrorUser32.NewProc("IsWindowVisible").Call(window)
		if owner == v.pid && visible != 0 {
			v.pendingChild = window
			return 0
		}
		return 1
	})
)

type mirrorCropResult struct {
	crop image.Rectangle
	w, h int
	err  error
}

// All window state belongs to the message-loop thread. Screenshot workers
// return immutable results through a channel; they never mutate window state.
type mirrorView struct {
	window, child, menu, popup uintptr
	pendingChild               uintptr
	pid                        uint32
	mode                       scaleMode
	frames                     *mirrorFrames
	frameW, frameH             int
	crop, candidate            image.Rectangle
	cropResults                chan mirrorCropResult
	scanning                   bool
	nextScan                   time.Time
	fullscreen                 bool
	menuOpen                   bool
	windowed                   mirrorRect
	cancel                     context.CancelFunc
}

func mirrorText(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func mirrorViewProc(window uintptr, message uint32, wParam, lParam uintptr) uintptr {
	v := mirrorActiveView.Load()
	if v != nil {
		switch message {
		case 0x5: // WM_SIZE
			v.layout(window)
		case 0x7: // WM_SETFOCUS
			mirrorUser32.NewProc("SetFocus").Call(v.child)
		case 0x211: // WM_ENTERMENULOOP
			v.menuOpen = true
		case 0x212: // WM_EXITMENULOOP
			v.menuOpen = false
		case 0x111: // WM_COMMAND
			id := int(wParam & 0xffff)
			if id >= 100 && id < 100+len(scaleModes) {
				v.mode = scaleMode(scaleModes[id-100])
				v.crop, v.candidate = image.Rectangle{}, image.Rectangle{}
				v.nextScan = time.Time{}
				v.layout(window)
			}
		case 0x8001: // Shortcut commands from our low-level hook.
			switch wParam {
			case 1:
				v.toggleFullscreen()
			case 2:
				v.showScaleMenu()
			case 3:
				if v.fullscreen {
					v.toggleFullscreen()
				}
			}
		case 0x10: // WM_CLOSE
			v.cancel()
			return 0
		}
	}
	r, _, _ := mirrorDefaultProc.Call(window, uintptr(message), wParam, lParam)
	return r
}

func (v *mirrorView) layout(window uintptr) {
	if v.child == 0 {
		return
	}
	var client mirrorRect
	mirrorClientRect.Call(window, uintptr(unsafe.Pointer(&client)))
	r := scaledMirrorRect(int(client.Right), int(client.Bottom), v.frameW, v.frameH, v.crop, v.mode)
	mirrorSetPosition.Call(v.child, 0, uintptr(r.X), uintptr(r.Y), uintptr(r.W), uintptr(r.H), 0x34) // frame changed, no activate/z-order
	for i, mode := range scaleModes {
		flags := uintptr(0)
		if scaleMode(mode) == v.mode {
			flags = 8
		} // MF_CHECKED
		mirrorUser32.NewProc("CheckMenuItem").Call(v.popup, uintptr(100+i), flags)
	}
	// Repaint uncovered borders after changing from fill/stretch to fit.
	mirrorUser32.NewProc("InvalidateRect").Call(window, 0, 1)
}

func (v *mirrorView) keepLayout() {
	// SDL may apply its initial size/position or a rotation resize after we
	// attach. Reconcile that asynchronous update with the host's viewport.
	var client, child mirrorRect
	mirrorClientRect.Call(v.window, uintptr(unsafe.Pointer(&client)))
	mirrorWindowRect.Call(v.child, uintptr(unsafe.Pointer(&child)))
	mirrorUser32.NewProc("MapWindowPoints").Call(0, v.window, uintptr(unsafe.Pointer(&child)), 2)
	want := scaledMirrorRect(int(client.Right), int(client.Bottom), v.frameW, v.frameH, v.crop, v.mode)
	if int(child.Left) != want.X || int(child.Top) != want.Y || int(child.Right-child.Left) != want.W || int(child.Bottom-child.Top) != want.H {
		v.layout(v.window)
	}
}

func (v *mirrorView) showScaleMenu() {
	var bounds mirrorRect
	mirrorWindowRect.Call(v.window, uintptr(unsafe.Pointer(&bounds)))
	// TPM_RETURNCMD: keep the command in this thread even while SDL has focus.
	id, _, _ := mirrorUser32.NewProc("TrackPopupMenu").Call(v.popup, 0x100, uintptr(bounds.Left+16), uintptr(bounds.Top+32), 0, v.window, 0)
	if id != 0 {
		mirrorViewProc(v.window, 0x111, id, 0)
	}
	mirrorUser32.NewProc("SetFocus").Call(v.child)
}

func (v *mirrorView) toggleFullscreen() {
	if !v.fullscreen {
		mirrorWindowRect.Call(v.window, uintptr(unsafe.Pointer(&v.windowed)))
		monitor, _, _ := mirrorMonitor.Call(v.window, 2)
		info := struct {
			Size          uint32
			Monitor, Work mirrorRect
			Flags         uint32
		}{}
		info.Size = uint32(unsafe.Sizeof(info))
		ok, _, _ := mirrorMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info)))
		if ok == 0 {
			return
		}
		v.fullscreen = true
		mirrorSetMenu.Call(v.window, 0)
		mirrorSetStyle.Call(v.window, ^uintptr(15), 0x96000000) // popup + visible + clip children
		b := info.Monitor
		mirrorSetPosition.Call(v.window, 0, uintptr(b.Left), uintptr(b.Top), uintptr(b.Right-b.Left), uintptr(b.Bottom-b.Top), 0x34)
	} else {
		v.fullscreen = false
		mirrorSetStyle.Call(v.window, ^uintptr(15), 0x16cf0000) // overlapped + visible + clip children
		mirrorSetMenu.Call(v.window, v.menu)
		b := v.windowed
		mirrorSetPosition.Call(v.window, 0, uintptr(b.Left), uintptr(b.Top), uintptr(b.Right-b.Left), uintptr(b.Bottom-b.Top), 0x34)
	}
	v.layout(v.window)
}

func (v *mirrorView) attach(child uintptr, fullscreen bool) error {
	module, _, _ := mirrorModule.Call(0)
	className := mirrorText("ScrcpyLANViewport")
	class := struct {
		Size, Style                        uint32
		Proc                               uintptr
		ClassExtra, WindowExtra            int32
		Instance, Icon, Cursor, Background uintptr
		Menu, Name                         *uint16
		SmallIcon                          uintptr
	}{Proc: mirrorViewCallback, Instance: module, Name: className}
	class.Size = uint32(unsafe.Sizeof(class))
	class.Cursor, _, _ = mirrorUser32.NewProc("LoadCursorW").Call(0, 32512)
	class.Background, _, _ = syscall.NewLazyDLL("gdi32.dll").NewProc("GetStockObject").Call(4) // BLACK_BRUSH
	registered, _, err := mirrorUser32.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&class)))
	if registered == 0 && err != syscall.Errno(1410) {
		return fmt.Errorf("注册投屏窗口: %w", err)
	}
	var bounds mirrorRect
	mirrorWindowRect.Call(child, uintptr(unsafe.Pointer(&bounds)))
	v.menu, _, _ = mirrorUser32.NewProc("CreateMenu").Call()
	v.popup, _, _ = mirrorUser32.NewProc("CreatePopupMenu").Call()
	for i, mode := range scaleModes {
		mirrorUser32.NewProc("AppendMenuW").Call(v.popup, 0, uintptr(100+i), uintptr(unsafe.Pointer(mirrorText(mode))))
	}
	mirrorUser32.NewProc("AppendMenuW").Call(v.menu, 0x10, v.popup, uintptr(unsafe.Pointer(mirrorText("画面缩放（Alt+Z）"))))
	v.window, _, err = mirrorCreateWindow.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(mirrorText("scrcpy LAN · 投屏 · Alt+Z 缩放"))), 0x06cf0000,
		uintptr(bounds.Left), uintptr(bounds.Top), uintptr(bounds.Right-bounds.Left), uintptr(bounds.Bottom-bounds.Top+24), 0, v.menu, module, 0)
	if v.window == 0 {
		return fmt.Errorf("创建投屏窗口: %w", err)
	}
	// Official scrcpy stays responsible for decoding, sound and all phone input.
	mirrorSetStyle.Call(child, ^uintptr(15), 0x50000000) // WS_CHILD | WS_VISIBLE
	mirrorSetStyle.Call(child, ^uintptr(19), 0)          // remove top-level extended styles
	parent, _, err := mirrorUser32.NewProc("SetParent").Call(child, v.window)
	if parent == 0 && err != syscall.Errno(0) {
		return fmt.Errorf("嵌入投屏窗口: %w", err)
	}
	v.child = child
	v.layout(v.window)
	mirrorUser32.NewProc("ShowWindow").Call(v.window, 5)
	mirrorUser32.NewProc("SetForegroundWindow").Call(v.window)
	mirrorUser32.NewProc("SetFocus").Call(child)
	if fullscreen {
		v.toggleFullscreen()
	}
	return nil
}

func captureMirrorCrop(ctx context.Context, b *backend, serial string) mirrorCropResult {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, b.adb, "-s", serial, "exec-out", "screencap", "-p")
	hideConsole(cmd)
	cmd.WaitDelay = time.Second
	data, err := cmd.Output()
	if err != nil {
		return mirrorCropResult{err: err}
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return mirrorCropResult{err: err}
	}
	bounds := img.Bounds()
	return mirrorCropResult{crop: blackBarCrop(img), w: bounds.Dx(), h: bounds.Dy()}
}

func runMirrorWindow(ctx context.Context, pid int, playback playbackOptions, b *backend, serial string, cancel context.CancelFunc) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// Match SDL's per-monitor pixel coordinates, including mixed-DPI desktops.
	if proc := mirrorUser32.NewProc("SetThreadDpiAwarenessContext"); proc.Find() == nil {
		previous, _, _ := proc.Call(^uintptr(3)) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2
		if previous != 0 {
			defer proc.Call(previous)
		}
	}
	var message mirrorMessage
	mirrorPeekMessage.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0, 0)
	thread, _, _ := mirrorThreadID.Call()
	stop := context.AfterFunc(ctx, func() { mirrorPostThread.Call(thread, 0x12, 0, 0) })
	defer stop()
	scanCtx, stopScan := context.WithCancel(ctx)
	var scans sync.WaitGroup
	defer func() { stopScan(); scans.Wait() }()
	v := &mirrorView{pid: uint32(pid), mode: scaleMode(playback.ScaleMode), frames: playback.Frames, cancel: cancel, cropResults: make(chan mirrorCropResult, 1)}
	if v.frames == nil {
		v.frames = &mirrorFrames{}
	}
	mirrorActiveView.Store(v)
	defer mirrorActiveView.CompareAndSwap(v, nil)
	defer func() {
		if v.child != 0 {
			mirrorPostMessage.Call(v.child, 0x10, 0, 0)
		}
		if v.window != 0 {
			mirrorDestroyWindow.Call(v.window)
		}
		if v.menu != 0 {
			mirrorUser32.NewProc("DestroyMenu").Call(v.menu)
		}
	}()
	state := &mirrorKeyboardState{pid: uint32(pid), view: v}
	mirrorKeys.Store(state)
	defer mirrorKeys.CompareAndSwap(state, nil)
	module, _, _ := mirrorModule.Call(0)
	hook, _, err := mirrorSetHook.Call(13, mirrorKeyCallback, module, 0)
	if hook == 0 {
		return fmt.Errorf("安装投屏快捷键: %w", err)
	}
	defer mirrorUnhook.Call(hook)
	timer, _, err := mirrorUser32.NewProc("SetTimer").Call(0, 0, 100, 0)
	if timer == 0 {
		return fmt.Errorf("启动投屏窗口检查: %w", err)
	}
	defer mirrorUser32.NewProc("KillTimer").Call(0, timer)
	deadline := time.Now().Add(15 * time.Second)
	for {
		result, _, err := mirrorGetMessage.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if result == 0 {
			return nil
		}
		if int32(result) == -1 {
			return fmt.Errorf("投屏窗口消息循环: %w", err)
		}
		if message.Message == 0x113 { // WM_TIMER
			w, h := v.frames.size()
			if w > 0 && h > 0 && (w != v.frameW || h != v.frameH) {
				v.frameW, v.frameH = w, h
				v.crop, v.candidate = image.Rectangle{}, image.Rectangle{}
				v.nextScan = time.Time{}
				if v.window != 0 {
					v.layout(v.window)
				}
			}
			if v.window == 0 {
				v.pendingChild = 0
				mirrorUser32.NewProc("EnumWindows").Call(mirrorFindChildCallback, 0)
				child := v.pendingChild
				if child != 0 && v.frameW > 0 {
					if err := v.attach(child, playback.Fullscreen); err != nil {
						return err
					}
				} else if time.Now().After(deadline) {
					return fmt.Errorf("等待投屏画面超时")
				}
			} else {
				alive, _, _ := mirrorUser32.NewProc("IsWindow").Call(v.child)
				if alive == 0 {
					v.cancel()
					return nil
				}
				v.keepLayout()
			}
			select {
			case crop := <-v.cropResults:
				v.scanning = false
				if v.mode == scaleAuto && crop.err == nil && !crop.crop.Empty() && math.Abs(float64(crop.w)/float64(crop.h)-float64(v.frameW)/float64(v.frameH)) < 0.01 {
					c := image.Rect(crop.crop.Min.X*v.frameW/crop.w, crop.crop.Min.Y*v.frameH/crop.h, crop.crop.Max.X*v.frameW/crop.w, crop.crop.Max.Y*v.frameH/crop.h)
					if v.crop.Empty() || c == v.candidate {
						v.crop = c
						v.layout(v.window)
					}
					v.candidate = c
				}
			default:
			}
			if v.window != 0 && v.mode == scaleAuto && !v.scanning && time.Now().After(v.nextScan) {
				v.scanning = true
				v.nextScan = time.Now().Add(3 * time.Second)
				scans.Go(func() {
					crop := captureMirrorCrop(scanCtx, b, serial)
					select {
					case v.cropResults <- crop:
					case <-scanCtx.Done():
					}
				})
			}
		}
		mirrorTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		mirrorDispatchMessage.Call(uintptr(unsafe.Pointer(&message)))
	}
}
