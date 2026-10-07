package main

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo"
)

// mygo pins the initial goroutine to the UI thread, so native checks run here.
// Opt in on an unlocked Windows desktop; ordinary CI uses the headless tester.
func TestMain(m *testing.M) {
	if os.Getenv("SCRCPY_GUI_LOG_NATIVE_TEST") == "1" {
		if err := checkNativeLogWindow(); err != nil {
			fmt.Println("native log window:", err)
			os.Exit(1)
		}
		fmt.Println("native log window: passed")
		os.Exit(0)
	}
	if os.Getenv("SCRCPY_GUI_MIRROR_NATIVE_TEST") == "1" {
		if err := checkNativeMirrorControls(); err != nil {
			fmt.Println("native scrcpy controls:", err)
			os.Exit(1)
		}
		fmt.Println("native scrcpy controls: passed")
		os.Exit(0)
	}
	if os.Getenv("SCRCPY_GUI_NATIVE_TEST") != "1" {
		os.Exit(m.Run())
	}
	if err := checkNativeWindowsControls(); err != nil {
		fmt.Println("native Windows controls:", err)
		os.Exit(1)
	}
	fmt.Println("native Windows controls: passed")
	os.Exit(0)
}

func checkNativeWindowsControls() error {
	done := make(chan error, 1)
	mygo.App.OnWindowCreated(func(w *mygo.Window) {
		go func() {
			defer w.Close()
			check := func() error {
				time.Sleep(500 * time.Millisecond)
				work := mygo.Screen.PrimaryDisplay().WorkArea
				bounds := w.Bounds()
				if bounds.Width > work.Width || bounds.Height > work.Height || !w.IsResizable() {
					return fmt.Errorf("window does not fit desktop or resize: %+v, work %+v", bounds, work)
				}
				if path := os.Getenv("SCRCPY_GUI_NATIVE_SCREENSHOT"); path != "" {
					data, err := w.CapturePage()
					if err != nil {
						return err
					}
					if err := os.WriteFile(path, data, 0600); err != nil {
						return err
					}
				}
				w.Minimize()
				if !w.IsMinimized() {
					return fmt.Errorf("minimize failed")
				}
				w.Restore()
				w.ToggleMaximize()
				if !w.IsMaximized() {
					return fmt.Errorf("maximize failed")
				}
				w.ToggleMaximize()
				w.ToggleFullScreen()
				full := w.Bounds()
				display := mygo.Screen.PrimaryDisplay().Bounds
				if !w.IsFullScreen() || full != display {
					return fmt.Errorf("fullscreen did not cover the monitor: %+v, monitor %+v", full, display)
				}
				w.SetFullScreen(false)
				if w.IsFullScreen() || w.Bounds() != bounds {
					return fmt.Errorf("fullscreen did not restore window bounds: %+v", w.Bounds())
				}
				return nil
			}
			done <- check()
		}()
	})
	main()
	return <-done
}
