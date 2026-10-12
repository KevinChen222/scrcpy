//go:build linux && (amd64 || arm64)

package main

import (
	"context"
	"fmt"
	"image"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/egoist/mygo"
)

// GTK owns decorations, menus and the selection overlay. X11 reparents the
// original SDL window into a clipped viewport: decoded video never crosses
// into Go, and scrcpy still performs all audio and device input mapping.
var linuxMirror struct {
	once                                     sync.Once
	err                                      error
	gtk, gdk, xlib, cairo, object, glib      uintptr
	windowNew                                func(int32) uintptr
	windowTitle                              func(uintptr, string)
	windowSize                               func(uintptr, int32, int32)
	fullscreen, unfullscreen                 func(uintptr)
	boxNew                                   func(int32, int32) uintptr
	pack                                     func(uintptr, uintptr, bool, bool, uint32)
	add                                      func(uintptr, uintptr)
	overlayNew, areaNew, menuNew, menuBarNew func() uintptr
	overlayAdd                               func(uintptr, uintptr)
	menuItem                                 func(string) uintptr
	menuAppend                               func(uintptr, uintptr)
	submenu                                  func(uintptr, uintptr)
	popup                                    func(uintptr, uintptr, uintptr, uintptr, uintptr, uint32, uint32)
	showAll, show, hide, destroy, queueDraw  func(uintptr)
	sensitive                                func(uintptr, bool)
	noShowAll                                func(uintptr, bool)
	addEvents                                func(uintptr, int32)
	widgetWindow                             func(uintptr) uintptr
	width, height                            func(uintptr) int32
	scaleFactor                              func(uintptr) int32
	signal                                   func(uintptr, string, uintptr, uintptr, uintptr, uint32) uint64
	getCoords                                func(uintptr, *float64, *float64) bool
	getButton                                func(uintptr, *uint32) bool
	getKey                                   func(uintptr, *uint32) bool
	getState                                 func(uintptr, *uint32) bool
	xid                                      func(uintptr) uintptr
	windowState                              func(uintptr) uint32
	raise                                    func(uintptr)
	sourceRGBA                               func(uintptr, float64, float64, float64, float64)
	paint                                    func(uintptr)
	rectangle                                func(uintptr, float64, float64, float64, float64)
	lineWidth                                func(uintptr, float64)
	stroke                                   func(uintptr)
	openDisplay                              func(uintptr) uintptr
	closeDisplay                             func(uintptr) int32
	root                                     func(uintptr) uintptr
	queryTree                                func(uintptr, uintptr, *uintptr, *uintptr, *uintptr, *uint32) int32
	attributes                               func(uintptr, uintptr, *linuxWindowAttributes) int32
	atom                                     func(uintptr, string, bool) uintptr
	property                                 func(uintptr, uintptr, uintptr, int64, int64, bool, uintptr, *uintptr, *int32, *uintptr, *uintptr, *uintptr) int32
	free                                     func(uintptr) int32
	reparent                                 func(uintptr, uintptr, uintptr, int32, int32) int32
	moveResize                               func(uintptr, uintptr, int32, int32, uint32, uint32) int32
	mapWindow                                func(uintptr, uintptr) int32
	withdraw                                 func(uintptr, uintptr, int32) int32
	changeAttributes                         func(uintptr, uintptr, uintptr, *[14]uintptr) int32
	focus                                    func(uintptr, uintptr, int32, uintptr) int32
	flush                                    func(uintptr) int32
	pending                                  func(uintptr) int32
	nextEvent                                func(uintptr, *[24]uintptr) int32
	keycode                                  func(uintptr, uintptr) uint8
	grabKey                                  func(uintptr, int32, uint32, uintptr, bool, int32, int32) int32
	ungrabKey                                func(uintptr, int32, uint32, uintptr) int32
	setErrorHandler                          func(uintptr) uintptr
	connectionNumber                         func(uintptr) int32
	watchFD                                  func(int32, int32, uint32, uintptr, uintptr, uintptr) uint32
	removeSource                             func(uint32) bool
	views                                    map[uintptr]*linuxMirrorView
	displays                                 sync.Map
	previousError                            uintptr
}

// Callbacks are allocated once per signature, not once per device.
var (
	linuxMirrorXReady = purego.NewCallback(func(_ int32, _ uint32, id uintptr) int32 {
		if v := linuxMirror.views[id]; v != nil && !v.closed {
			v.readKeys()
		}
		return 1
	})
	linuxMirrorBackground = purego.NewCallback(func(_ uintptr, cr uintptr, _ uintptr) int32 {
		linuxMirror.sourceRGBA(cr, 0, 0, 0, 1)
		linuxMirror.paint(cr)
		return 0
	})
	linuxMirrorDestroy = purego.NewCallback(func(_ uintptr, id uintptr) {
		if v := linuxMirror.views[id]; v != nil {
			v.closed = true
			v.cancel()
		}
	})
	linuxMirrorResize = purego.NewCallback(func(_ uintptr, _ uintptr, id uintptr) {
		if v := linuxMirror.views[id]; v != nil && !v.closed {
			v.layout()
		}
	})
	linuxMirrorKey = purego.NewCallback(func(_ uintptr, event uintptr, id uintptr) int32 {
		v := linuxMirror.views[id]
		if v == nil || v.closed {
			return 0
		}
		var key, mod uint32
		linuxMirror.getKey(event, &key)
		linuxMirror.getState(event, &mod)
		if v.key(key, mod) {
			return 1
		}
		return 0
	})
	linuxMirrorPointer = purego.NewCallback(func(widget, event, id uintptr) int32 {
		v := linuxMirror.views[id]
		if v == nil || v.closed {
			return 0
		}
		if !v.selecting {
			if v.child != 0 {
				linuxMirror.focus(v.display, v.child, 1, 0)
				linuxMirror.flush(v.display)
			}
			return 0
		}
		var x, y float64
		linuxMirror.getCoords(event, &x, &y)
		point := image.Pt(int(x), int(y))
		var button uint32
		if linuxMirror.getButton(event, &button) && button == 1 {
			// GdkEvent's first member is its type: press=4, release=7.
			kind := *(*int32)(linuxPointer(event))
			if kind == 4 {
				v.start = point
				v.end = point
				v.dragging = true
			}
			if kind == 7 && v.dragging {
				v.end = point
				v.dragging = false
				r := v.renderedRect()
				crop := selectedMirrorCrop(image.Rectangle{Min: v.start, Max: v.end}.Canon(), r, v.frameW, v.frameH)
				if !crop.Empty() {
					v.mode = scaleManual
					v.crop = crop
					v.endSelection()
				}
			}
		} else if v.dragging {
			v.end = point
		}
		linuxMirror.queueDraw(widget)
		return 1
	})
	linuxMirrorDraw = purego.NewCallback(func(_ uintptr, cr, id uintptr) int32 {
		v := linuxMirror.views[id]
		if v == nil || !v.selecting {
			return 0
		}
		linuxMirror.sourceRGBA(cr, 0, 0, 0, 0.25)
		linuxMirror.paint(cr)
		if v.dragging {
			r := image.Rectangle{Min: v.start, Max: v.end}.Canon()
			linuxMirror.sourceRGBA(cr, 0.1, 1, 0.8, 1)
			linuxMirror.lineWidth(cr, 3)
			linuxMirror.rectangle(cr, float64(r.Min.X), float64(r.Min.Y), float64(r.Dx()), float64(r.Dy()))
			linuxMirror.stroke(cr)
		}
		return 1
	})
	linuxMirrorMenu = purego.NewCallback(func(_ uintptr, data uintptr) {
		v := linuxMirror.views[data>>8]
		if v == nil || v.closed {
			return
		}
		action := int(data & 255)
		if action < len(scaleModes) {
			v.mode = scaleMode(scaleModes[action])
			v.crop = image.Rectangle{}
			v.endSelection()
		}
		if action == 10 {
			v.beginSelection()
		}
		if action == 11 {
			v.mode = scaleFit
			v.crop = image.Rectangle{}
			v.endSelection()
		}
		v.layout()
	})
	linuxMirrorXError = purego.NewCallback(func(display, event uintptr) int32 {
		if _, ours := linuxMirror.displays.Load(display); ours {

			return 0
		} // child may close between queries
		if linuxMirror.previousError != 0 {
			result, _, _ := purego.SyscallN(linuxMirror.previousError, display, event)
			return int32(result)
		}
		return 0
	})
)

func initLinuxMirror() error {
	linuxMirror.once.Do(func() {
		bind := func(name string, handle *uintptr, functions map[string]any) {
			if linuxMirror.err != nil {
				return
			}
			h, err := purego.Dlopen(name, purego.RTLD_NOW|purego.RTLD_GLOBAL)
			if err != nil {
				linuxMirror.err = err
				return
			}
			*handle = h
			for symbol, fn := range functions {
				p, err := purego.Dlsym(h, symbol)
				if err != nil {
					linuxMirror.err = err
					return
				}
				purego.RegisterFunc(fn, p)
			}
		}
		bind("libgtk-3.so.0", &linuxMirror.gtk, map[string]any{
			"gtk_window_new": &linuxMirror.windowNew, "gtk_window_set_title": &linuxMirror.windowTitle, "gtk_window_set_default_size": &linuxMirror.windowSize,
			"gtk_window_fullscreen": &linuxMirror.fullscreen, "gtk_window_unfullscreen": &linuxMirror.unfullscreen,
			"gtk_box_new": &linuxMirror.boxNew, "gtk_box_pack_start": &linuxMirror.pack, "gtk_container_add": &linuxMirror.add,
			"gtk_overlay_new": &linuxMirror.overlayNew, "gtk_overlay_add_overlay": &linuxMirror.overlayAdd, "gtk_drawing_area_new": &linuxMirror.areaNew,
			"gtk_menu_new": &linuxMirror.menuNew, "gtk_menu_bar_new": &linuxMirror.menuBarNew, "gtk_menu_item_new_with_label": &linuxMirror.menuItem,
			"gtk_menu_shell_append": &linuxMirror.menuAppend, "gtk_menu_item_set_submenu": &linuxMirror.submenu, "gtk_menu_popup": &linuxMirror.popup,
			"gtk_widget_show_all": &linuxMirror.showAll, "gtk_widget_show": &linuxMirror.show, "gtk_widget_hide": &linuxMirror.hide, "gtk_widget_destroy": &linuxMirror.destroy,
			"gtk_widget_queue_draw": &linuxMirror.queueDraw, "gtk_widget_set_sensitive": &linuxMirror.sensitive, "gtk_widget_set_no_show_all": &linuxMirror.noShowAll,
			"gtk_widget_add_events": &linuxMirror.addEvents, "gtk_widget_get_window": &linuxMirror.widgetWindow, "gtk_widget_get_allocated_width": &linuxMirror.width, "gtk_widget_get_allocated_height": &linuxMirror.height,
			"gtk_widget_get_scale_factor": &linuxMirror.scaleFactor,
		})
		bind("libgdk-3.so.0", &linuxMirror.gdk, map[string]any{
			"gdk_event_get_coords": &linuxMirror.getCoords, "gdk_event_get_button": &linuxMirror.getButton, "gdk_event_get_keyval": &linuxMirror.getKey, "gdk_event_get_state": &linuxMirror.getState,
			"gdk_x11_window_get_xid": &linuxMirror.xid, "gdk_window_get_state": &linuxMirror.windowState, "gdk_window_raise": &linuxMirror.raise,
		})
		bind("libglib-2.0.so.0", &linuxMirror.glib, map[string]any{"g_unix_fd_add_full": &linuxMirror.watchFD, "g_source_remove": &linuxMirror.removeSource})
		bind("libgobject-2.0.so.0", &linuxMirror.object, map[string]any{"g_signal_connect_data": &linuxMirror.signal})
		bind("libcairo.so.2", &linuxMirror.cairo, map[string]any{
			"cairo_set_source_rgba": &linuxMirror.sourceRGBA, "cairo_paint": &linuxMirror.paint, "cairo_rectangle": &linuxMirror.rectangle, "cairo_set_line_width": &linuxMirror.lineWidth, "cairo_stroke": &linuxMirror.stroke,
		})
		bind("libX11.so.6", &linuxMirror.xlib, map[string]any{
			"XOpenDisplay": &linuxMirror.openDisplay, "XCloseDisplay": &linuxMirror.closeDisplay, "XDefaultRootWindow": &linuxMirror.root,
			"XQueryTree": &linuxMirror.queryTree, "XGetWindowAttributes": &linuxMirror.attributes, "XInternAtom": &linuxMirror.atom, "XGetWindowProperty": &linuxMirror.property, "XFree": &linuxMirror.free,
			"XReparentWindow": &linuxMirror.reparent, "XMoveResizeWindow": &linuxMirror.moveResize, "XMapWindow": &linuxMirror.mapWindow,
			"XWithdrawWindow": &linuxMirror.withdraw, "XChangeWindowAttributes": &linuxMirror.changeAttributes,
			"XSetInputFocus": &linuxMirror.focus, "XFlush": &linuxMirror.flush, "XPending": &linuxMirror.pending, "XNextEvent": &linuxMirror.nextEvent,
			"XConnectionNumber": &linuxMirror.connectionNumber, "XKeysymToKeycode": &linuxMirror.keycode, "XGrabKey": &linuxMirror.grabKey, "XUngrabKey": &linuxMirror.ungrabKey, "XSetErrorHandler": &linuxMirror.setErrorHandler,
		})
		if linuxMirror.err == nil {
			linuxMirror.views = make(map[uintptr]*linuxMirrorView)
			linuxMirror.previousError = linuxMirror.setErrorHandler(linuxMirrorXError)
		}
	})
	return linuxMirror.err
}

// Native pointers originate in GTK/Xlib, outside the Go heap.
func linuxPointer(p uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&p)) }

type linuxMirrorView struct {
	id, window, viewport, overlay, menubar, menu, selectionItem, display, child, pidAtom uintptr
	pid                                                                                  int
	mode                                                                                 scaleMode
	frames                                                                               *mirrorFrames
	frameW, frameH                                                                       int
	crop                                                                                 image.Rectangle
	selecting, dragging, closed, full, escapeGrabbed                                     bool
	start, end                                                                           image.Point
	focusPending                                                                         bool
	cancel                                                                               context.CancelFunc
	keySource                                                                            uint32
	keys                                                                                 map[uint8]uint32
	withdrawn                                                                            time.Time
}

type linuxWindowAttributes struct {
	X, Y, Width, Height, Border, Depth              int32
	Visual, Root                                    uintptr
	Class, BitGravity, WinGravity, BackingStore     int32
	BackingPlanes, BackingPixel                     uintptr
	SaveUnder                                       int32
	Colormap                                        uintptr
	MapInstalled, MapState                          int32
	AllEventMask, YourEventMask, DoNotPropagateMask int64
	OverrideRedirect                                int32
	Screen                                          uintptr
}

func (v *linuxMirrorView) findChild(window uintptr, depth int) uintptr {
	var actual, n, remaining, data uintptr
	var format int32
	if linuxMirror.property(v.display, window, v.pidAtom, 0, 1, false, 0, &actual, &format, &n, &remaining, &data) == 0 && data != 0 {
		pid := uintptr(0)
		if format == 32 && n == 1 {
			pid = *(*uintptr)(linuxPointer(data))
		}
		linuxMirror.free(data)
		if format == 32 && n == 1 && int(pid) == v.pid {
			var a linuxWindowAttributes
			// Toolkits also give their invisible client-leader/input-only
			// windows a PID. Embed only the visible video window.
			if linuxMirror.attributes(v.display, window, &a) != 0 && a.Class == 1 && a.MapState == 2 {
				return window
			}
		}
	}
	if depth == 0 {
		return 0
	}
	var root, parent, children uintptr
	var count uint32
	if linuxMirror.queryTree(v.display, window, &root, &parent, &children, &count) == 0 || children == 0 {
		return 0
	}
	defer linuxMirror.free(children)
	if count > 4096 {
		return 0
	}
	for _, child := range unsafe.Slice((*uintptr)(linuxPointer(children)), int(count)) {
		if found := v.findChild(child, depth-1); found != 0 {
			return found
		}
	}
	return 0
}

func (v *linuxMirrorView) create(serial string, fullscreen bool) {
	v.window = linuxMirror.windowNew(0)
	v.id = v.window
	linuxMirror.views[v.id] = v
	linuxMirror.windowTitle(v.window, "scrcpy LAN · "+serial)
	linuxMirror.windowSize(v.window, int32(min(1000, max(360, v.frameW))), int32(min(760, max(360, v.frameH))))
	box := linuxMirror.boxNew(1, 0)
	linuxMirror.add(v.window, box)
	v.menubar = linuxMirror.menuBarNew()
	v.menu = linuxMirror.menuNew()
	item := linuxMirror.menuItem("画面缩放 · Alt+Z")
	linuxMirror.submenu(item, v.menu)
	linuxMirror.menuAppend(v.menubar, item)
	addItem := func(label string, action int) uintptr {
		item := linuxMirror.menuItem(label)
		linuxMirror.menuAppend(v.menu, item)
		linuxMirror.signal(item, "activate", linuxMirrorMenu, v.id<<8|uintptr(action), 0, 0)
		return item
	}
	for i, mode := range scaleModes {
		addItem(mode, i)
	}
	v.selectionItem = addItem("选择显示区域（仅全屏，Esc 取消）", 10)
	addItem("重置显示区域", 11)
	linuxMirror.pack(box, v.menubar, false, false, 0)
	overlay := linuxMirror.overlayNew()
	v.viewport = linuxMirror.areaNew()
	v.overlay = linuxMirror.areaNew()
	linuxMirror.add(overlay, v.viewport)
	linuxMirror.overlayAdd(overlay, v.overlay)
	linuxMirror.pack(box, overlay, true, true, 0)
	linuxMirror.noShowAll(v.overlay, true)
	linuxMirror.addEvents(v.viewport, 1<<8)
	linuxMirror.addEvents(v.overlay, (1<<2)|(1<<8)|(1<<9))
	linuxMirror.signal(v.window, "destroy", linuxMirrorDestroy, v.id, 0, 0)
	linuxMirror.signal(v.window, "key-press-event", linuxMirrorKey, v.id, 0, 0)
	linuxMirror.signal(v.viewport, "draw", linuxMirrorBackground, v.id, 0, 0)
	linuxMirror.signal(v.viewport, "size-allocate", linuxMirrorResize, v.id, 0, 0)
	linuxMirror.signal(v.viewport, "button-press-event", linuxMirrorPointer, v.id, 0, 0)
	for _, name := range []string{"button-press-event", "button-release-event", "motion-notify-event"} {
		linuxMirror.signal(v.overlay, name, linuxMirrorPointer, v.id, 0, 0)
	}
	linuxMirror.signal(v.overlay, "draw", linuxMirrorDraw, v.id, 0, 0)
	linuxMirror.showAll(v.window)
	parent := linuxMirror.xid(linuxMirror.widgetWindow(v.viewport))
	var attributes [14]uintptr
	attributes[11] = 1 // XSetWindowAttributes.override_redirect (offset 88)
	linuxMirror.changeAttributes(v.display, v.child, 1<<9, &attributes)
	linuxMirror.reparent(v.display, v.child, parent, 0, 0)
	linuxMirror.mapWindow(v.display, v.child)
	v.keys = make(map[uint8]uint32)
	for _, key := range []uint32{0xffc8, 'z', 'q', 0xff1b} {
		v.keys[linuxMirror.keycode(v.display, uintptr(key))] = key
	}
	// Passive grabs are scoped to this child's focus; Lock and NumLock are ignored.
	for _, key := range []uint32{0xffc8, 'z', 'q'} {
		v.grab(key, true)
	}
	if fullscreen {
		v.toggleFullscreen()
	}
	v.keySource = linuxMirror.watchFD(0, linuxMirror.connectionNumber(v.display), 1, linuxMirrorXReady, v.id, 0)
	v.focusPending = true
	v.layout()
	linuxMirror.flush(v.display)
}

func (v *linuxMirrorView) grab(key uint32, on bool) {
	mods := uint32(0)
	if key == 'z' || key == 'q' {
		mods = 8
	} // Mod1 (Alt)
	for _, lock := range []uint32{0, 2, 16, 18} {
		code := int32(linuxMirror.keycode(v.display, uintptr(key)))
		if on {
			linuxMirror.grabKey(v.display, code, mods|lock, v.child, false, 1, 1)
		} else {
			linuxMirror.ungrabKey(v.display, code, mods|lock, v.child)
		}
	}
}

func (v *linuxMirrorView) renderedRect() scaleRect {
	return v.renderedRectAtScale(1)
}

func (v *linuxMirrorView) pixelRect() scaleRect {
	return v.renderedRectAtScale(int(max(1, linuxMirror.scaleFactor(v.viewport))))
}

func (v *linuxMirrorView) renderedRectAtScale(scale int) scaleRect {
	mode, crop := v.mode, v.crop
	if v.selecting {
		mode, crop = scaleFit, image.Rectangle{}
	}
	return scaledMirrorRect(int(linuxMirror.width(v.viewport))*scale, int(linuxMirror.height(v.viewport))*scale, v.frameW, v.frameH, crop, mode)
}

func (v *linuxMirrorView) layout() {
	if v.child == 0 || v.viewport == 0 || v.closed {
		return
	}
	viewportWindow := linuxMirror.widgetWindow(v.viewport)
	if viewportWindow == 0 {
		return
	} // GTK has not realized this widget yet.
	// GTK allocations/pointer events are logical units; X11 child geometry is
	// in device pixels. Keep selection in logical units and scale only here.
	r := v.pixelRect()
	var root, parent, children uintptr
	var count uint32
	linuxMirror.queryTree(v.display, v.child, &root, &parent, &children, &count)
	if children != 0 {
		linuxMirror.free(children)
	}
	viewport := linuxMirror.xid(viewportWindow)
	if parent != viewport {
		linuxMirror.reparent(v.display, v.child, viewport, int32(r.X), int32(r.Y))
	}
	// SDL may restore its own geometry after a rotation. Reapply the viewport
	// even if our desired rectangle did not change.
	linuxMirror.moveResize(v.display, v.child, int32(r.X), int32(r.Y), uint32(max(1, r.W)), uint32(max(1, r.H)))
	linuxMirror.mapWindow(v.display, v.child)
	if v.focusPending {
		var attributes linuxWindowAttributes
		if linuxMirror.attributes(v.display, v.child, &attributes) != 0 && attributes.MapState == 2 {
			linuxMirror.focus(v.display, v.child, 1, 0)
			v.focusPending = false
		}
	}
	linuxMirror.sensitive(v.selectionItem, v.full)
	if v.selecting {
		linuxMirror.raise(linuxMirror.widgetWindow(v.overlay))
	}
	linuxMirror.flush(v.display)
}

func (v *linuxMirrorView) toggleFullscreen() {
	if v.full {
		linuxMirror.unfullscreen(v.window)
		linuxMirror.show(v.menubar)
		v.full = false
		v.endSelection()
	} else {
		linuxMirror.hide(v.menubar)
		linuxMirror.fullscreen(v.window)
		v.full = true
	}
	v.updateEscapeGrab()
	v.layout()
}

func (v *linuxMirrorView) updateEscapeGrab() {
	needed := v.full || v.selecting
	if needed != v.escapeGrabbed {
		v.grab(0xff1b, needed)
		v.escapeGrabbed = needed
	}
}

func (v *linuxMirrorView) beginSelection() {
	if !v.full {
		return
	}
	v.selecting = true
	v.dragging = false
	linuxMirror.show(v.overlay)
	v.layout()
}

func (v *linuxMirrorView) endSelection() {
	v.selecting = false
	v.dragging = false
	if v.overlay != 0 {
		linuxMirror.hide(v.overlay)
	}
	v.layout()
}

func (v *linuxMirrorView) key(key, mod uint32) bool {
	switch {
	case key == 0xffc8 && mod&(1|4|8|64) == 0:
		v.toggleFullscreen()
	case key == 'z' && mod&8 != 0:
		linuxMirror.popup(v.menu, 0, 0, 0, 0, 0, 0)
	case key == 'q' && mod&8 != 0:
		v.cancel()
	case key == 0xff1b && mod&(1|4|8|64) == 0:
		if v.selecting {
			v.endSelection()
		} else if v.full {
			v.toggleFullscreen()
		} else {
			return false
		}
	default:
		return false
	}
	return true
}

func (v *linuxMirrorView) tick() {
	if v.closed {
		return
	}
	w, h := v.frames.size()
	if w > 0 && h > 0 && (w != v.frameW || h != v.frameH) {
		v.frameW, v.frameH = w, h
		v.crop = image.Rectangle{}
		if v.mode == scaleManual {
			v.mode = scaleFit
		}
		v.endSelection()
	}
	v.readKeys()
	v.layout()
}

func (v *linuxMirrorView) readKeys() {
	// Readable-fd notifications dispatch keys immediately on GTK's thread.
	// A separate X connection receives only the passive shortcuts we grabbed.
	for linuxMirror.pending(v.display) > 0 {
		var event [24]uintptr
		linuxMirror.nextEvent(v.display, &event)
		if int32(event[0]) == 2 { // XKeyEvent state/keycode at byte 80/84 on LP64
			state := *(*uint32)(unsafe.Add(unsafe.Pointer(&event), 80))
			code := *(*uint32)(unsafe.Add(unsafe.Pointer(&event), 84))

			v.key(v.keys[uint8(code)], state)
		}
	}
}

func runMirrorWindow(ctx context.Context, pid int, playback playbackOptions, serial string, cancel context.CancelFunc) error {
	// Backend tests start processes without running a desktop event loop.
	if !mygo.App.IsReady() {
		<-ctx.Done()
		return nil
	}
	v := &linuxMirrorView{pid: pid, mode: scaleMode(playback.ScaleMode), frames: playback.Frames, cancel: cancel}
	if v.frames == nil {
		v.frames = &mirrorFrames{}
	}
	var err error
	mygo.RunOnMain(func() {
		err = initLinuxMirror()
		if err != nil {
			return
		}
		v.display = linuxMirror.openDisplay(0)
		if v.display == 0 {
			err = fmt.Errorf("投屏窗口需要 X11 或 XWayland 的 DISPLAY")
			return
		}
		linuxMirror.displays.Store(v.display, true)
		v.pidAtom = linuxMirror.atom(v.display, "_NET_WM_PID", false)
	})
	if err != nil {
		return err
	}
	if v.display == 0 {
		return ctx.Err()
	}
	defer mygo.RunOnMain(func() {
		if v.window != 0 && !v.closed {
			linuxMirror.destroy(v.window)
		}
		if v.keySource != 0 {
			linuxMirror.removeSource(v.keySource)
		}
		delete(linuxMirror.views, v.id)
		linuxMirror.closeDisplay(v.display)
		linuxMirror.displays.Delete(v.display)
	})
	deadline := time.Now().Add(15 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			closed, attached := false, false
			mygo.RunOnMain(func() {
				if v.window == 0 {
					v.frameW, v.frameH = v.frames.size()
					if v.child != 0 {
						if time.Since(v.withdrawn) >= 200*time.Millisecond {
							v.create(serial, playback.Fullscreen)
						}
					} else if v.frameW > 0 {
						v.child = v.findChild(linuxMirror.root(v.display), 3)
						if v.child != 0 {
							// Let the WM release its frame before embedding. Otherwise
							// an outstanding MapRequest can reparent SDL back out.
							linuxMirror.withdraw(v.display, v.child, 0)
							linuxMirror.flush(v.display)
							v.withdrawn = time.Now()
						}
					}
				} else {
					v.tick()
				}
				closed, attached = v.closed, v.window != 0
			})
			if closed {
				return nil
			}
			if !attached && time.Now().After(deadline) {
				return fmt.Errorf("等待 Linux 投屏窗口超时（需要 X11/XWayland）")
			}
		}
	}
}
