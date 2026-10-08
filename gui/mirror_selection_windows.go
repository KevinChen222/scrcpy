package main

import (
	"image"
	"unsafe"
)

func (v *mirrorView) beginSelection() {
	if !v.fullscreen || v.selecting || v.frameW <= 0 || v.frameH <= 0 {
		return
	}
	module, _, _ := mirrorModule.Call(0)
	overlay, _, _ := mirrorCreateWindow.Call(0x80000, uintptr(unsafe.Pointer(mirrorText("ScrcpyLANViewport"))),
		uintptr(unsafe.Pointer(mirrorText("选择显示区域"))), 0x50000000, 0, 0, 1, 1, v.window, 0, module, 0)
	if overlay == 0 {
		return
	}
	v.overlay, v.selecting, v.dragging = overlay, true, false
	v.selectionStart, v.selectionEnd = image.Point{}, image.Point{}
	mirrorViews.Store(overlay, v)
	// A translucent input surface shows the video while intercepting the entire
	// drag; none of the selection clicks are delivered to the phone.
	mirrorUser32.NewProc("SetLayeredWindowAttributes").Call(overlay, 0, 160, 2)
	v.layout(v.window)
	mirrorUser32.NewProc("SetFocus").Call(overlay)
}

func (v *mirrorView) endSelection() {
	if !v.selecting {
		return
	}
	v.selecting, v.dragging = false, false
	mirrorUser32.NewProc("ReleaseCapture").Call()
	mirrorDestroyWindow.Call(v.overlay)
	mirrorViews.Delete(v.overlay)
	v.overlay = 0
	mirrorUser32.NewProc("SetFocus").Call(v.child)
}

func (v *mirrorView) selectionProc(window uintptr, message uint32, wParam, lParam uintptr) uintptr {
	point := image.Pt(int(int16(lParam&0xffff)), int(int16((lParam>>16)&0xffff)))
	switch message {
	case 0x201: // WM_LBUTTONDOWN
		v.selectionStart, v.selectionEnd, v.dragging = point, point, true
		mirrorUser32.NewProc("SetCapture").Call(window)
	case 0x200: // WM_MOUSEMOVE
		if !v.dragging {
			return 0
		}
		v.selectionEnd = point
	case 0x202: // WM_LBUTTONUP
		if !v.dragging {
			return 0
		}
		v.selectionEnd, v.dragging = point, false
		mirrorUser32.NewProc("ReleaseCapture").Call()
		var client mirrorRect
		mirrorClientRect.Call(v.window, uintptr(unsafe.Pointer(&client)))
		rendered := v.renderedRect(int(client.Right), int(client.Bottom))
		crop := selectedMirrorCrop(image.Rectangle{Min: v.selectionStart, Max: v.selectionEnd}.Canon(), rendered, v.frameW, v.frameH)
		if !crop.Empty() {
			v.mode, v.crop = scaleManual, crop
			v.endSelection()
			v.layout(v.window)
			return 0
		}
	case 0x215: // WM_CAPTURECHANGED
		v.dragging = false
		return 0
	case 0x20: // WM_SETCURSOR
		cursor, _, _ := mirrorUser32.NewProc("LoadCursorW").Call(0, 32515) // crosshair
		mirrorUser32.NewProc("SetCursor").Call(cursor)
		return 1
	case 0xf: // WM_PAINT
		v.paintSelection(window)
		return 0
	default:
		r, _, _ := mirrorDefaultProc.Call(window, uintptr(message), wParam, lParam)
		return r
	}
	mirrorUser32.NewProc("InvalidateRect").Call(window, 0, 1)
	return 0
}

func (v *mirrorView) paintSelection(window uintptr) {
	paint := struct {
		DC              uintptr
		Erase           int32
		Rect            mirrorRect
		Restore, Update int32
		Reserved        [32]byte
	}{}
	dc, _, _ := mirrorUser32.NewProc("BeginPaint").Call(window, uintptr(unsafe.Pointer(&paint)))
	defer mirrorUser32.NewProc("EndPaint").Call(window, uintptr(unsafe.Pointer(&paint)))
	gdi := mirrorGDI32
	pen, _, _ := gdi.NewProc("CreatePen").Call(0, 3, 0x00ffff)
	oldPen, _, _ := gdi.NewProc("SelectObject").Call(dc, pen)
	brush, _, _ := gdi.NewProc("GetStockObject").Call(5) // NULL_BRUSH
	oldBrush, _, _ := gdi.NewProc("SelectObject").Call(dc, brush)
	r := image.Rectangle{Min: v.selectionStart, Max: v.selectionEnd}.Canon()
	if !r.Empty() {
		gdi.NewProc("Rectangle").Call(dc, uintptr(r.Min.X), uintptr(r.Min.Y), uintptr(r.Max.X), uintptr(r.Max.Y))
	}
	gdi.NewProc("SelectObject").Call(dc, oldBrush)
	gdi.NewProc("SelectObject").Call(dc, oldPen)
	gdi.NewProc("DeleteObject").Call(pen)
	font, _, _ := gdi.NewProc("CreateFontW").Call(24, 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(mirrorText("Microsoft YaHei"))))
	oldFont, _, _ := gdi.NewProc("SelectObject").Call(dc, font)
	gdi.NewProc("SetBkMode").Call(dc, 1)
	gdi.NewProc("SetTextColor").Call(dc, 0xffffff)
	var bounds mirrorRect
	mirrorClientRect.Call(window, uintptr(unsafe.Pointer(&bounds)))
	bounds.Left, bounds.Top = 16, 16
	bounds.Right -= 16
	text := mirrorText("拖动鼠标框选要显示的画面，松开确认。Esc 取消；Alt+Z 可重新选择或重置。")
	mirrorUser32.NewProc("DrawTextW").Call(dc, uintptr(unsafe.Pointer(text)), ^uintptr(0), uintptr(unsafe.Pointer(&bounds)), 0x10) // DT_WORDBREAK
	gdi.NewProc("SelectObject").Call(dc, oldFont)
	gdi.NewProc("DeleteObject").Call(font)
}
