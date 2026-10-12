//go:build linux && (amd64 || arm64)

package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Use the existing Linux TestMain event loop, without running a real ADB or
// touching a phone. A native capture exercises QR rendering in the GTK window.
func TestLinuxQRNative(t *testing.T) {
	if os.Getenv("SCRCPY_GUI_QR_NATIVE_TEST") != "1" {
		t.Skip("set SCRCPY_GUI_QR_NATIVE_TEST=1 on an X11 desktop")
	}
	t.Setenv("GDK_BACKEND", "x11")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := &backend{run: func(ctx context.Context, args ...string) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}}
	a := newApplication(ctx, b)
	mygo.App.WhenReady(func() {
		a.showPair, a.pairQRMode = true, true
		a.startQRPair()
		win := mygo.NewWindow(mygo.WindowOptions{
			Title: "scrcpy LAN QR native test", Width: 840, Height: 600,
			Content: ui.View(a.view),
		})
		go func() {
			time.Sleep(500 * time.Millisecond)
			data, err := win.CapturePage()
			if err != nil || len(data) == 0 {
				t.Errorf("native QR capture failed: %v", err)
			} else if path := os.Getenv("SCRCPY_GUI_QR_NATIVE_SCREENSHOT"); path != "" {
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Error(err)
				}
			}
			mygo.RunOnMain(func() { a.closePair(); win.Close(); mygo.App.Quit() })
		}()
	})
	close(linuxNativeReady)
	if err := <-linuxNativeResult; err != nil {
		t.Fatal(err)
	}
	a.workers.Wait()
}
