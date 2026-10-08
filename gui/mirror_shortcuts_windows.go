package main

import (
	"sync"
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
	mirrorKeys             sync.Map // message-loop thread ID -> keyboard state
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
	pid          uint32
	view         *mirrorView
	shortcutHeld uint32
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
	thread, _, _ := mirrorThreadID.Call()
	entry, found := mirrorKeys.Load(thread)
	if code == 0 && found {
		state := entry.(*mirrorKeyboardState)
		v := state.view
		if state.shortcutHeld == *key {
			if message == 0x101 || message == 0x105 {
				state.shortcutHeld = 0
			}
			return 1
		}
		foreground, _, _ := mirrorForeground.Call()
		root, _, _ := mirrorUser32.NewProc("GetAncestor").Call(foreground, 2)
		if v.window != 0 && (foreground == v.window || root == v.window) && (message == 0x100 || message == 0x104) {
			command := uintptr(0)
			if *key == 0x7a && !mirrorModifiersDown() {
				command = 1
			}
			if *key == 0x1b && (v.fullscreen || v.selecting) && !v.menuOpen && !mirrorModifiersDown() {
				command = 3
			}
			alt, _, _ := mirrorKeyState.Call(0x12)
			ctrl, _, _ := mirrorKeyState.Call(0x11)
			shift, _, _ := mirrorKeyState.Call(0x10)
			if *key == 'Z' && alt&0x8000 != 0 && ctrl&0x8000 == 0 && shift&0x8000 == 0 {
				command = 2
			}
			if *key == 'F' && alt&0x8000 != 0 && ctrl&0x8000 == 0 && shift&0x8000 == 0 {
				command = 1
			}
			if command != 0 {
				mirrorPostMessage.Call(v.window, 0x8001, command, 0)
				state.shortcutHeld = *key
				return 1
			}
		}
		r, _, _ := mirrorNextHook.Call(0, uintptr(code), message, uintptr(unsafe.Pointer(key)))
		return r
	}
	result, _, _ := mirrorNextHook.Call(0, uintptr(code), message, uintptr(unsafe.Pointer(key)))
	return result
}
