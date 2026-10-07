package main

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"
)

var (
	mirrorUser32           = syscall.NewLazyDLL("user32.dll")
	mirrorKernel32         = syscall.NewLazyDLL("kernel32.dll")
	mirrorSetHook          = mirrorUser32.NewProc("SetWindowsHookExW")
	mirrorUnhook           = mirrorUser32.NewProc("UnhookWindowsHookEx")
	mirrorNextHook         = mirrorUser32.NewProc("CallNextHookEx")
	mirrorForeground       = mirrorUser32.NewProc("GetForegroundWindow")
	mirrorWindowPID        = mirrorUser32.NewProc("GetWindowThreadProcessId")
	mirrorKeyState         = mirrorUser32.NewProc("GetAsyncKeyState")
	mirrorWindowStyle      = mirrorUser32.NewProc("GetWindowLongPtrW")
	mirrorWindowRect       = mirrorUser32.NewProc("GetWindowRect")
	mirrorMonitor          = mirrorUser32.NewProc("MonitorFromWindow")
	mirrorMonitorInfo      = mirrorUser32.NewProc("GetMonitorInfoW")
	mirrorPostMessage      = mirrorUser32.NewProc("PostMessageW")
	mirrorPostThread       = mirrorUser32.NewProc("PostThreadMessageW")
	mirrorPeekMessage      = mirrorUser32.NewProc("PeekMessageW")
	mirrorGetMessage       = mirrorUser32.NewProc("GetMessageW")
	mirrorTranslateMessage = mirrorUser32.NewProc("TranslateMessage")
	mirrorDispatchMessage  = mirrorUser32.NewProc("DispatchMessageW")
	mirrorThreadID         = mirrorKernel32.NewProc("GetCurrentThreadId")
	mirrorModule           = mirrorKernel32.NewProc("GetModuleHandleW")
	mirrorKeys             atomic.Pointer[mirrorKeyboardState]
	mirrorKeyCallback      = syscall.NewCallback(handleMirrorKey)
)

type mirrorRect struct{ Left, Top, Right, Bottom int32 }

type mirrorMessage struct {
	Window         uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	X, Y           int32
	Private        uint32
}

type mirrorKeyboardState struct {
	pid        uint32
	escapeHeld bool // Used only on the hook's message-loop thread.
}

func mirrorIsFullscreen(window uintptr) bool {
	style, _, _ := mirrorWindowStyle.Call(window, ^uintptr(15)) // GWL_STYLE = -16
	if style&0x00c00000 != 0 {                                  // WS_CAPTION: ordinary/maximized windows retain it.
		return false
	}
	var bounds mirrorRect
	ok, _, _ := mirrorWindowRect.Call(window, uintptr(unsafe.Pointer(&bounds)))
	if ok == 0 {
		return false
	}
	monitor, _, _ := mirrorMonitor.Call(window, 2)
	info := struct {
		Size          uint32
		Monitor, Work mirrorRect
		Flags         uint32
	}{}
	info.Size = uint32(unsafe.Sizeof(info))
	ok, _, _ = mirrorMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info)))
	return ok != 0 && bounds == info.Monitor
}

func mirrorModifiersDown() bool {
	for _, key := range []uintptr{0x10, 0x11, 0x12, 0x5b, 0x5c} { // Shift, Ctrl, Alt, Win
		state, _, _ := mirrorKeyState.Call(key)
		if state&0x8000 != 0 {
			return true
		}
	}
	return false
}

func handleMirrorKey(code int32, message uintptr, key *uint32) uintptr {
	state := mirrorKeys.Load()
	if code == 0 && state != nil && *key == 0x1b {
		if state.escapeHeld {
			if message == 0x101 || message == 0x105 { // key up / system key up
				state.escapeHeld = false
			}
			return 1 // Also consume repeats and the matching release.
		}
		if message == 0x100 && !mirrorModifiersDown() { // plain WM_KEYDOWN
			window, _, _ := mirrorForeground.Call()
			var pid uint32
			mirrorWindowPID.Call(window, uintptr(unsafe.Pointer(&pid)))
			if pid == state.pid && mirrorIsFullscreen(window) {
				// scrcpy v5.0 handles F11; send it directly to this window.
				ok, _, _ := mirrorPostMessage.Call(window, 0x100, 0x7a, 0x00570001)
				if ok != 0 {
					mirrorPostMessage.Call(window, 0x101, 0x7a, 0xc0570001)
					state.escapeHeld = true
					return 1
				}
			}
		}
	}
	result, _, _ := mirrorNextHook.Call(0, uintptr(code), message, uintptr(unsafe.Pointer(key)))
	return result
}

// Only the foreground fullscreen window belonging to this session consumes Esc.
// In windowed mode scrcpy retains its normal Android Back shortcut.
func runMirrorShortcuts(ctx context.Context, pid int) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var message mirrorMessage
	mirrorPeekMessage.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0, 0) // create queue
	thread, _, _ := mirrorThreadID.Call()
	module, _, _ := mirrorModule.Call(0)
	state := &mirrorKeyboardState{pid: uint32(pid)}
	mirrorKeys.Store(state)
	defer mirrorKeys.CompareAndSwap(state, nil)
	hook, _, err := mirrorSetHook.Call(13, mirrorKeyCallback, module, 0) // WH_KEYBOARD_LL
	if hook == 0 {
		return fmt.Errorf("安装投屏 Esc 快捷键: %w", err)
	}
	defer mirrorUnhook.Call(hook)
	stop := context.AfterFunc(ctx, func() { mirrorPostThread.Call(thread, 0x12, 0, 0) }) // WM_QUIT
	defer stop()
	for {
		result, _, err := mirrorGetMessage.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if result == 0 {
			return nil
		}
		if int32(result) == -1 {
			return fmt.Errorf("投屏快捷键消息循环: %w", err)
		}
		mirrorTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		mirrorDispatchMessage.Call(uintptr(unsafe.Pointer(&message)))
	}
}
