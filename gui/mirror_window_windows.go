package main

import (
	"context"
	"fmt"
	"image"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	mirrorGDI32             = syscall.NewLazyDLL("gdi32.dll")
	mirrorCreateWindow      = mirrorUser32.NewProc("CreateWindowExW")
	mirrorDestroyWindow     = mirrorUser32.NewProc("DestroyWindow")
	mirrorDefaultProc       = mirrorUser32.NewProc("DefWindowProcW")
	mirrorSetStyle          = mirrorUser32.NewProc("SetWindowLongPtrW")
	mirrorSetPosition       = mirrorUser32.NewProc("SetWindowPos")
	mirrorClientRect        = mirrorUser32.NewProc("GetClientRect")
	mirrorSetMenu           = mirrorUser32.NewProc("SetMenu")
	mirrorViews             sync.Map // window handle -> view; view state stays on its own thread
	mirrorViewCallback      = syscall.NewCallback(mirrorViewProc)
	mirrorFindChildCallback = syscall.NewCallback(func(window, _ uintptr) uintptr {
		thread, _, _ := mirrorThreadID.Call()
		entry, found := mirrorKeys.Load(thread)
		if !found {
			return 0
		}
		v := entry.(*mirrorKeyboardState).view
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

// Each device owns a message-loop thread and independent window/input state.
type mirrorView struct {
	window, child, menu, popup, overlay uintptr
	pendingChild                        uintptr
	pid                                 uint32
	title                               string
	mode                                scaleMode
	frames                              *mirrorFrames
	frameW, frameH                      int
	crop                                image.Rectangle
	selecting, dragging                 bool
	selectionStart, selectionEnd        image.Point
	fullscreen                          bool
	menuOpen                            bool
	windowed                            mirrorRect
	cancel                              context.CancelFunc
}

func mirrorText(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func mirrorViewProc(window uintptr, message uint32, wParam, lParam uintptr) uintptr {
	entry, found := mirrorViews.Load(window)
	if found {
		v := entry.(*mirrorView)
		if window == v.overlay {
			return v.selectionProc(window, message, wParam, lParam)
		}
		switch message {
		case 0x5: // WM_SIZE
			v.layout(window)
		case 0x7: // WM_SETFOCUS
			if v.selecting {
				mirrorUser32.NewProc("SetFocus").Call(v.overlay)
			} else {
				mirrorUser32.NewProc("SetFocus").Call(v.child)
			}
		case 0x211: // WM_ENTERMENULOOP
			v.menuOpen = true
		case 0x212: // WM_EXITMENULOOP
			v.menuOpen = false
		case 0x111: // WM_COMMAND
			id := int(wParam & 0xffff)
			if id >= 100 && id < 100+len(scaleModes) {
				v.endSelection()
				v.mode = scaleMode(scaleModes[id-100])
				v.crop = image.Rectangle{}
				v.layout(window)
			}
			if id == 200 {
				v.beginSelection()
			}
			if id == 201 {
				v.endSelection()
				v.mode, v.crop = scaleFit, image.Rectangle{}
				v.layout(window)
			}
		case 0x8001: // Shortcut commands from our low-level hook.
			switch wParam {
			case 1:
				v.toggleFullscreen()
			case 2:
				v.showScaleMenu()
			case 3:
				if v.selecting {
					v.endSelection()
					v.layout(window)
				} else if v.fullscreen {
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

func (v *mirrorView) renderedRect(width, height int) scaleRect {
	mode, crop := v.mode, v.crop
	if v.selecting {
		mode, crop = scaleFit, image.Rectangle{}
	}
	return scaledMirrorRect(width, height, v.frameW, v.frameH, crop, mode)
}

func (v *mirrorView) layout(window uintptr) {
	if v.child == 0 {
		return
	}
	var client mirrorRect
	mirrorClientRect.Call(window, uintptr(unsafe.Pointer(&client)))
	r := v.renderedRect(int(client.Right), int(client.Bottom))
	mirrorSetPosition.Call(v.child, 0, uintptr(r.X), uintptr(r.Y), uintptr(r.W), uintptr(r.H), 0x34) // frame changed, no activate/z-order
	if v.overlay != 0 {
		mirrorSetPosition.Call(v.overlay, 0, 0, 0, uintptr(client.Right), uintptr(client.Bottom), 0x10) // HWND_TOP, no activate
	}
	for i, mode := range scaleModes {
		flags := uintptr(0)
		if scaleMode(mode) == v.mode {
			flags = 8
		} // MF_CHECKED
		mirrorUser32.NewProc("CheckMenuItem").Call(v.popup, uintptr(100+i), flags)
	}
	flags := uintptr(1) // MF_GRAYED: selection is available in fullscreen only
	if v.fullscreen {
		flags = 0
	}
	mirrorUser32.NewProc("EnableMenuItem").Call(v.popup, 200, flags)
	flags = 0
	if v.mode == scaleManual {
		flags = 8
	}
	mirrorUser32.NewProc("CheckMenuItem").Call(v.popup, 200, flags)
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
	want := v.renderedRect(int(client.Right), int(client.Bottom))
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
	if v.selecting {
		mirrorUser32.NewProc("SetFocus").Call(v.overlay)
	} else {
		mirrorUser32.NewProc("SetFocus").Call(v.child)
	}
}

func (v *mirrorView) toggleFullscreen() {
	v.endSelection()
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
	class.Icon, _, _ = mirrorUser32.NewProc("LoadIconW").Call(module, 1)
	class.SmallIcon = class.Icon
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
	mirrorUser32.NewProc("AppendMenuW").Call(v.popup, 0x800, 0, 0) // separator
	mirrorUser32.NewProc("AppendMenuW").Call(v.popup, 0, 200, uintptr(unsafe.Pointer(mirrorText("选择显示区域（仅全屏）"))))
	mirrorUser32.NewProc("AppendMenuW").Call(v.popup, 0, 201, uintptr(unsafe.Pointer(mirrorText("重置显示区域"))))
	mirrorUser32.NewProc("AppendMenuW").Call(v.menu, 0x10, v.popup, uintptr(unsafe.Pointer(mirrorText("画面缩放（Alt+Z）"))))
	v.window, _, err = mirrorCreateWindow.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(mirrorText("scrcpy LAN · "+v.title+" · Alt+Z 缩放"))), 0x06cf0000,
		uintptr(bounds.Left), uintptr(bounds.Top), uintptr(bounds.Right-bounds.Left), uintptr(bounds.Bottom-bounds.Top+24), 0, v.menu, module, 0)
	if v.window == 0 {
		return fmt.Errorf("创建投屏窗口: %w", err)
	}
	mirrorViews.Store(v.window, v)
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

func runMirrorWindow(ctx context.Context, pid int, playback playbackOptions, serial string, cancel context.CancelFunc) error {
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
	v := &mirrorView{pid: uint32(pid), title: serial, mode: scaleMode(playback.ScaleMode), frames: playback.Frames, cancel: cancel}
	if v.frames == nil {
		v.frames = &mirrorFrames{}
	}
	defer func() {
		v.endSelection()
		if v.child != 0 {
			mirrorPostMessage.Call(v.child, 0x10, 0, 0)
		}
		if v.window != 0 {
			mirrorDestroyWindow.Call(v.window)
			mirrorViews.Delete(v.window)
		}
		if v.menu != 0 {
			mirrorUser32.NewProc("DestroyMenu").Call(v.menu)
		}
	}()
	state := &mirrorKeyboardState{pid: uint32(pid), view: v}
	mirrorKeys.Store(thread, state)
	defer mirrorKeys.Delete(thread)
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
				v.endSelection()
				v.crop = image.Rectangle{}
				if v.mode == scaleManual {
					v.mode = scaleFit
				}
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
		}
		mirrorTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		mirrorDispatchMessage.Call(uintptr(unsafe.Pointer(&message)))
	}
}
