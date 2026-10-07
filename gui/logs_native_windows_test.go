package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func checkNativeLogWindow() error {
	a := newApplication(context.Background(), nil)
	done := make(chan error, 1)
	mygo.App.WhenReady(func() {
		root := mygo.NewWindow(mygo.WindowOptions{Title: "scrcpy LAN · 日志验证", Width: 1080, Height: 760, Content: ui.View(a.view)})
		root.OnClosed(func() { mygo.App.Quit() })
		go func() {
			defer root.Close()
			check := func() error {
				var first *mygo.Window
				mygo.RunOnMain(func() { a.openLogWindow(); first = a.logWindow; a.openLogWindow() })
				if !a.logs.isEnabled() {
					return fmt.Errorf("opening the window did not enable collection")
				}
				for i := range 100 {
					a.logs.printf("INFO 实时中文日志 %d", i)
				}
				a.logs.printf("ERROR 测试错误消息")
				time.Sleep(500 * time.Millisecond)
				if path := os.Getenv("SCRCPY_GUI_LOG_SCREENSHOT"); path != "" {
					data, err := first.CapturePage()
					if err != nil {
						return err
					}
					if err := os.WriteFile(path, data, 0600); err != nil {
						return err
					}
				}
				first.Close()
				var closed bool
				mygo.RunOnMain(func() { closed = a.logWindow == nil })
				if !closed || a.logs.isEnabled() || a.logs.String() != "" {
					return fmt.Errorf("closing the window did not disable and clear collection")
				}
				a.logs.printf("ERROR 关闭后不得记录")
				var second *mygo.Window
				mygo.RunOnMain(func() { a.openLogWindow(); second = a.logWindow })
				if first == second || strings.Contains(a.logs.String(), "关闭后不得记录") {
					return fmt.Errorf("reopening retained a closed window or old diagnostics")
				}
				second.Close()
				return nil
			}
			done <- check()
		}()
	})
	if err := mygo.App.Run(); err != nil {
		return err
	}
	return <-done
}
