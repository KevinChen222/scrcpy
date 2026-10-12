//go:build linux && (amd64 || arm64)

package main

import (
	"context"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ebitengine/purego"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

var linuxNativeReady = make(chan struct{})
var linuxNativeResult = make(chan error)

func TestMain(m *testing.M) {
	if os.Getenv("SCRCPY_GUI_LINUX_NATIVE_TEST") != "1" && os.Getenv("SCRCPY_GUI_QR_NATIVE_TEST") != "1" && os.Getenv("SCRCPY_GUI_LINUX_MIRROR_SERIAL") == "" {
		os.Exit(m.Run())
	}
	result := make(chan int, 1)
	go func() { result <- m.Run() }()
	select {
	case code := <-result:
		os.Exit(code)
	case <-linuxNativeReady:
		linuxNativeResult <- mygo.App.Run()
		os.Exit(<-result)
	}
}

// Opt-in native test with a buffered GTK child, or the official scrcpy client
// when SCRCPY_GUI_LINUX_MIRROR_SERIAL names an authorized phone. Both exercise
// the production launch path, X11 embedding, fullscreen and selection.
func TestLinuxMirrorNative(t *testing.T) {
	serial := os.Getenv("SCRCPY_GUI_LINUX_MIRROR_SERIAL")
	if os.Getenv("SCRCPY_GUI_LINUX_NATIVE_TEST") != "1" && serial == "" {
		t.Skip("set SCRCPY_GUI_LINUX_NATIVE_TEST=1 on an X11 desktop")
	}
	t.Setenv("GDK_BACKEND", "x11")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	frames := &mirrorFrames{}
	playback := playbackOptions{Frames: frames, ScaleMode: string(scaleFit), BufferMS: 2000}
	var child *exec.Cmd
	var output *sessionLog
	if serial != "" {
		b, err := newBackend()
		if err != nil {
			t.Fatal(err)
		}
		b.logs = &sessionLog{}
		b.logs.setEnabled(true)
		child, output, err = b.start(ctx, serial, presets[1], playback)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		dir := t.TempDir()
		source := filepath.Join(dir, "video.c")
		binary := filepath.Join(dir, "video")
		code := `#include <gtk/gtk.h>
#include <stdio.h>
static gboolean draw(GtkWidget *w,cairo_t *c,gpointer p) {
 int width=gtk_widget_get_allocated_width(w),height=gtk_widget_get_allocated_height(w);
 cairo_set_source_rgb(c,.1,.2,.3);cairo_paint(c);
 cairo_set_source_rgb(c,.1,.8,.6);cairo_rectangle(c,width*.2,height*.2,width*.6,height*.6);cairo_fill(c);
 return FALSE;
}
int main(int argc,char **argv) {
	gtk_init(NULL,NULL);GtkWidget *w=gtk_window_new(GTK_WINDOW_TOPLEVEL),*a=gtk_drawing_area_new();
 gtk_window_set_title(GTK_WINDOW(w),"scrcpy LAN native test video");gtk_window_set_default_size(GTK_WINDOW(w),640,480);
 gtk_container_add(GTK_CONTAINER(w),a);g_signal_connect(a,"draw",G_CALLBACK(draw),NULL);
	g_signal_connect(w,"destroy",G_CALLBACK(gtk_main_quit),NULL);gtk_widget_show_all(w);
	printf("INFO: Texture: 640x480\n");gtk_main();return 0;
}`
		if err := os.WriteFile(source, []byte(code), 0600); err != nil {
			t.Fatal(err)
		}
		flags, err := exec.Command("pkg-config", "--cflags", "--libs", "gtk+-3.0").Output()
		if err != nil {
			t.Fatal(err)
		}
		args := append([]string{source, "-o", binary}, strings.Fields(string(flags))...)
		if out, err := exec.Command("gcc", args...).CombinedOutput(); err != nil {
			t.Fatalf("native helper: %v %s", err, out)
		}
		b := &backend{scrcpy: binary, adb: "unused"}
		child, _, err = b.start(ctx, "native-test", presets[1], playback)
		if err != nil {
			t.Fatal(err)
		}
	}
	exited := make(chan error, 1)
	go func() { exited <- child.Wait() }()
	defer func() { cancel(); <-exited }()
	done := make(chan error, 1)
	mygo.App.SetName("scrcpy LAN Linux native test")
	mygo.App.WhenReady(func() {
		keeper := mygo.NewWindow(mygo.WindowOptions{Title: "scrcpy LAN Linux native test", Width: 840, Height: 600, Content: ui.View(func(c *ui.Context) { ui.Text(c, "正在验证 Linux 投屏窗口") })})
		go func() {
			done <- runMirrorWindow(ctx, child.Process.Pid, playback, "native-test", cancel)
		}()
		go func() {
			err := exerciseLinuxMirror(ctx, child.Process.Pid, frames, exited, serial != "")
			if err != nil {
				t.Error(err)
				if output != nil {
					t.Log(output.String())
				}
			} else if output != nil {
				if !strings.Contains(output.String(), "Decoder 'audio':") || !strings.Contains(output.String(), "INFO: Texture:") {
					t.Error("real fullscreen test did not decode both audio and video")
				}
				t.Log("real audio/video decoded; F11 fullscreen stayed alive for 12 seconds; menu, Esc, F11 and Alt+Q passed")
			}
			cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Error(err)
				}
			case <-time.After(3 * time.Second):
				t.Error("mirror cleanup timed out")
			}
			mygo.RunOnMain(func() { keeper.Close(); mygo.App.Quit() })
		}()
	})
	close(linuxNativeReady)
	if err := <-linuxNativeResult; err != nil {
		t.Fatal(err)
	}
}

func exerciseLinuxMirror(ctx context.Context, pid int, frames *mirrorFrames, exited chan error, realDevice bool) error {
	var v *linuxMirrorView
	wait := func(check func() bool) bool {
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			ok := false
			mygo.RunOnMain(func() { ok = check() })
			if ok {
				return true
			}
			select {
			case <-ctx.Done():
				return false
			case <-time.After(40 * time.Millisecond):
			}
		}
		return false
	}
	if !wait(func() bool {
		for _, candidate := range linuxMirror.views {
			if candidate.pid == pid {
				v = candidate
				return !v.closed
			}
		}
		return false
	}) {
		return fmt.Errorf("child was not embedded")
	}
	var getGeometry func(uintptr, uintptr, *uintptr, *int32, *int32, *uint32, *uint32, *uint32, *uint32) int32
	var translate func(uintptr, uintptr, uintptr, int32, int32, *int32, *int32, *uintptr) int32
	var visible func(uintptr) bool
	purego.RegisterLibFunc(&getGeometry, linuxMirror.xlib, "XGetGeometry")
	purego.RegisterLibFunc(&translate, linuxMirror.xlib, "XTranslateCoordinates")
	purego.RegisterLibFunc(&visible, linuxMirror.gtk, "gtk_widget_get_visible")
	xtst, err := purego.Dlopen("libXtst.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return err
	}
	var fakeKey func(uintptr, uint32, bool, uintptr) int32
	var fakeButton func(uintptr, uint32, bool, uintptr) int32
	var fakeMotion func(uintptr, int32, int32, int32, uintptr) int32
	purego.RegisterLibFunc(&fakeKey, xtst, "XTestFakeKeyEvent")
	purego.RegisterLibFunc(&fakeButton, xtst, "XTestFakeButtonEvent")
	purego.RegisterLibFunc(&fakeMotion, xtst, "XTestFakeMotionEvent")
	key := func(sym uint32, alt bool) {
		mygo.RunOnMain(func() {
			linuxMirror.focus(v.display, v.child, 1, 0)
			if alt {
				fakeKey(v.display, uint32(linuxMirror.keycode(v.display, 0xffe9)), true, 0)
			}
			code := uint32(linuxMirror.keycode(v.display, uintptr(sym)))
			fakeKey(v.display, code, true, 0)
			fakeKey(v.display, code, false, 0)
			if alt {
				fakeKey(v.display, uint32(linuxMirror.keycode(v.display, 0xffe9)), false, 0)
			}
			linuxMirror.flush(v.display)
		})
	}
	geometry := func() bool {
		var attributes linuxWindowAttributes
		if linuxMirror.attributes(v.display, v.child, &attributes) == 0 || attributes.MapState != 2 {
			return false
		}
		var parent, children uintptr
		var count uint32
		var treeRoot uintptr
		linuxMirror.queryTree(v.display, v.child, &treeRoot, &parent, &children, &count)
		if children != 0 {
			linuxMirror.free(children)
		}
		if parent != linuxMirror.xid(linuxMirror.widgetWindow(v.viewport)) {
			return false
		}
		var root uintptr
		var x, y int32
		var w, h, border, depth uint32
		if getGeometry(v.display, v.child, &root, &x, &y, &w, &h, &border, &depth) == 0 {
			return false
		}
		// Compare to the native viewport's pixels, independently of the layout
		// helper; using GTK's logical allocation hid the HiDPI rendering bug.
		var viewport linuxWindowAttributes
		if linuxMirror.attributes(v.display, parent, &viewport) == 0 {
			return false
		}
		mode, crop := v.mode, v.crop
		if v.selecting {
			mode, crop = scaleFit, image.Rectangle{}
		}
		r := scaledMirrorRect(int(viewport.Width), int(viewport.Height), v.frameW, v.frameH, crop, mode)
		return int(x) == r.X && int(y) == r.Y && int(w) == r.W && int(h) == r.H
	}
	for _, mode := range []scaleMode{scaleFit, scaleFill, scaleStretch} {
		mygo.RunOnMain(func() { v.mode = mode; v.layout() })
		if !wait(geometry) {
			return fmt.Errorf("%s geometry differs from viewport", mode)
		}
		time.Sleep(300 * time.Millisecond)
		if err := captureLinuxMirror(v, "window-"+string(mode)); err != nil {
			return err
		}
	}
	key(0xffc8, false)
	if !wait(func() bool { return v.full && linuxMirror.windowState(linuxMirror.widgetWindow(v.window))&(1<<4) != 0 }) {
		var state uint32
		var full bool
		mygo.RunOnMain(func() { state = linuxMirror.windowState(linuxMirror.widgetWindow(v.window)); full = v.full })
		return fmt.Errorf("F11 did not enter fullscreen: requested=%t, GDK state=%x", full, state)
	}
	for _, mode := range []scaleMode{scaleFit, scaleFill, scaleStretch} {
		mygo.RunOnMain(func() { v.mode = mode; v.layout() })
		if !wait(geometry) {
			return fmt.Errorf("fullscreen %s geometry differs from native viewport pixels", mode)
		}
		time.Sleep(300 * time.Millisecond)
		if err := captureLinuxMirror(v, "fullscreen-"+string(mode)); err != nil {
			return err
		}
	}
	// Stay fullscreen beyond the reported delayed failure, rather than leaving
	// immediately after checking the host's window state.
	for deadline := time.Now().Add(12 * time.Second); time.Now().Before(deadline); {
		select {
		case err := <-exited:
			exited <- err // leave the process result for cleanup
			return fmt.Errorf("video process exited in fullscreen: %v", err)
		case <-ctx.Done():
			return fmt.Errorf("fullscreen session ended: %w", ctx.Err())
		case <-time.After(200 * time.Millisecond):
		}
		if !wait(geometry) {
			return fmt.Errorf("fullscreen child geometry did not settle")
		}
	}
	if err := captureLinuxMirror(v, "fullscreen"); err != nil {
		return err
	}
	key('z', true)
	if !wait(func() bool { return visible(v.menu) }) {
		return fmt.Errorf("Alt+Z did not open the menu")
	}
	// GTK owns menu dismissal before the wrapper handles fullscreen Escape.
	mygo.RunOnMain(func() {
		fakeKey(v.display, uint32(linuxMirror.keycode(v.display, 0xff1b)), true, 0)
		fakeKey(v.display, uint32(linuxMirror.keycode(v.display, 0xff1b)), false, 0)
		linuxMirror.flush(v.display)
	})
	if !wait(func() bool { return !visible(v.menu) && v.full }) {
		return fmt.Errorf("Escape did not dismiss menu before fullscreen")
	}
	if realDevice {
		// Keep phone input untouched: exercise pointer selection and injected
		// frame rotation only on the simulated window below.
		key(0xff1b, false)
		if !wait(func() bool { return !v.full }) {
			return fmt.Errorf("Escape did not leave real fullscreen")
		}
		time.Sleep(300 * time.Millisecond)
		if err := captureLinuxMirror(v, "restored"); err != nil {
			return err
		}
		key(0xffc8, false)
		if !wait(func() bool { return v.full }) {
			return fmt.Errorf("F11 did not restore real fullscreen")
		}
		key('q', true)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(3 * time.Second):
			return fmt.Errorf("Alt+Q did not stop the real session")
		}
	}
	mygo.RunOnMain(func() { v.beginSelection() })
	if !wait(func() bool {
		var overlay, viewport linuxWindowAttributes
		return linuxMirror.attributes(v.display, linuxMirror.xid(linuxMirror.widgetWindow(v.overlay)), &overlay) != 0 &&
			linuxMirror.attributes(v.display, linuxMirror.xid(linuxMirror.widgetWindow(v.viewport)), &viewport) != 0 &&
			overlay.MapState == 2 && overlay.Width == viewport.Width && overlay.Height == viewport.Height && geometry()
	}) {
		return fmt.Errorf("selection overlay did not cover the native viewport")
	}
	mygo.RunOnMain(func() {
		var x, y int32
		var child uintptr
		translate(v.display, linuxMirror.xid(linuxMirror.widgetWindow(v.overlay)), linuxMirror.root(v.display), 0, 0, &x, &y, &child)
		var bounds linuxWindowAttributes
		linuxMirror.attributes(v.display, linuxMirror.xid(linuxMirror.widgetWindow(v.overlay)), &bounds)
		w, h := bounds.Width, bounds.Height
		fakeMotion(v.display, -1, x+w/3, y+h/3, 0)
		fakeButton(v.display, 1, true, 0)
		fakeMotion(v.display, -1, x+2*w/3, y+2*h/3, 0)
		fakeButton(v.display, 1, false, 0)
		linuxMirror.flush(v.display)
	})
	if !wait(func() bool { return !v.selecting && v.mode == scaleManual && !v.crop.Empty() }) {
		var detail string
		mygo.RunOnMain(func() {
			detail = fmt.Sprintf("selecting=%t dragging=%t start=%v end=%v rect=%+v scale=%d", v.selecting, v.dragging, v.start, v.end, v.renderedRect(), linuxMirror.scaleFactor(v.viewport))
		})
		return fmt.Errorf("drag selection was not applied: %s", detail)
	}
	var crop image.Rectangle
	mygo.RunOnMain(func() { crop = v.crop })
	key(0xff1b, false)
	if !wait(func() bool { return !v.full && v.crop == crop }) {
		return fmt.Errorf("Escape did not leave fullscreen and preserve crop")
	}
	key(0xffc8, false)
	if !wait(func() bool { return v.full }) {
		return fmt.Errorf("F11 did not restore fullscreen")
	}
	mygo.RunOnMain(func() { v.beginSelection() })
	key(0xff1b, false)
	if !wait(func() bool { return !v.selecting && v.full && v.crop == crop }) {
		return fmt.Errorf("selection cancellation lost prior crop or left fullscreen")
	}
	fmt.Fprintln(frames, "INFO: Texture: 480x640")
	if !wait(func() bool { return v.mode == scaleFit && v.crop.Empty() && v.frameW == 480 && v.frameH == 640 }) {
		return fmt.Errorf("rotation did not reset selection")
	}
	key('q', true)
	select {
	case <-ctx.Done():
		return nil
	case <-time.After(3 * time.Second):
		return fmt.Errorf("Alt+Q did not stop the session")
	}
}

func captureLinuxMirror(v *linuxMirrorView, name string) error {
	dir := os.Getenv("SCRCPY_GUI_LINUX_MIRROR_CAPTURE_DIR")
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	var capture func(uintptr, int32, int32, int32, int32) uintptr
	purego.RegisterLibFunc(&capture, linuxMirror.gdk, "gdk_pixbuf_get_from_window")
	pixbufLib, err := purego.Dlopen("libgdk_pixbuf-2.0.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return err
	}
	defer purego.Dlclose(pixbufLib)
	var save func(uintptr, string, string, uintptr, uintptr, *uintptr) bool
	var unref func(uintptr)
	purego.RegisterLibFunc(&save, pixbufLib, "gdk_pixbuf_savev")
	purego.RegisterLibFunc(&unref, linuxMirror.object, "g_object_unref")
	mygo.RunOnMain(func() {
		pixbuf := capture(linuxMirror.widgetWindow(v.window), 0, 0, linuxMirror.width(v.window), linuxMirror.height(v.window))
		if pixbuf == 0 {
			err = fmt.Errorf("capture %s failed", name)
			return
		}
		defer unref(pixbuf)
		var nativeErr uintptr
		if !save(pixbuf, filepath.Join(dir, name+".png"), "png", 0, 0, &nativeErr) {
			err = fmt.Errorf("save %s failed", name)
		}
	})
	return err
}
