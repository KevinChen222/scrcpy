package main

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Diagnostics live only in memory while the log window is open.
type sessionLog struct {
	mu      sync.Mutex
	enabled bool
	data    []byte
}

func (l *sessionLog) setEnabled(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enabled, l.data = enabled, nil
}

func (l *sessionLog) isEnabled() bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.enabled
}

func (l *sessionLog) Write(p []byte) (int, error) {
	if l == nil {
		return len(p), nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.enabled {
		l.data = append(l.data, p...)
		if len(l.data) > 65536 {
			l.data = l.data[len(l.data)-65536:]
			for len(l.data) > 0 && !utf8.RuneStart(l.data[0]) {
				l.data = l.data[1:]
			}
		}
	}
	return len(p), nil
}

func (l *sessionLog) printf(format string, args ...any) {
	if l != nil {
		fmt.Fprintf(l, "%s %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, args...))
	}
}

func (l *sessionLog) String() string {
	if l == nil {
		return ""
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.TrimSpace(string(l.data))
}

func (a *application) openLogWindow() {
	if a.logWindow != nil {
		a.logWindow.Show()
		return
	}
	a.logs.setEnabled(true)
	a.logs.printf("INFO 日志窗口已打开 · %s · %s", version, a.status)
	if a.errorText != "" {
		a.logs.printf("ERROR %s", a.errorText)
	}
	var scroll ui.ScrollState
	a.logWindow = mygo.NewWindow(mygo.WindowOptions{
		Title: "scrcpy LAN · 实时日志", Width: 800, Height: 500,
		MinWidth: 480, MinHeight: 280, TitleBarStyle: mygo.TitleBarDefault,
		Content: ui.View(func(c *ui.Context) {
			c.After(100 * time.Millisecond)
			text := a.logs.String()
			if scroll.Y >= scroll.MaxY {
				scroll.Y = math.MaxFloat32
			}
			ui.Column(c).Fill().Padding(16).Gap(10).Children(func() {
				ui.Text(c, "实时日志 · 关闭后停止收集并清空，不写入文件；保留最近 64 KiB。").FontSize(12)
				ui.TextArea(c, &text).ReadOnly(true).FillWidth().Grow(1).TrackScroll(&scroll)
			})
		}),
	})
	a.logWindow.OnClosed(func() {
		a.logs.setEnabled(false)
		a.logWindow = nil
	})
}
