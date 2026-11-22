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
	onWrite func()
}

func (l *sessionLog) setEnabled(enabled bool) {
	l.mu.Lock()
	l.enabled, l.data = enabled, nil
	cb := l.onWrite
	l.mu.Unlock()
	if enabled && cb != nil {
		cb()
	}
}

func (l *sessionLog) setOnWrite(fn func()) {
	l.mu.Lock()
	l.onWrite = fn
	l.mu.Unlock()
}

func (l *sessionLog) clear() {
	l.mu.Lock()
	l.data = nil
	cb := l.onWrite
	l.mu.Unlock()
	if cb != nil {
		cb()
	}
}

func (l *sessionLog) isEnabled() bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.enabled
}

func (l *sessionLog) byteLen() int {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.data)
}

func (l *sessionLog) Write(p []byte) (int, error) {
	if l == nil {
		return len(p), nil
	}
	l.mu.Lock()
	var cb func()
	if l.enabled {
		l.data = append(l.data, p...)
		if len(l.data) > 65536 {
			l.data = l.data[len(l.data)-65536:]
			for len(l.data) > 0 && !utf8.RuneStart(l.data[0]) {
				l.data = l.data[1:]
			}
		}
		cb = l.onWrite
	}
	l.mu.Unlock()
	if cb != nil {
		cb()
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
	win := mygo.NewWindow(mygo.WindowOptions{
		Title: "scrcpy LAN · 实时日志", Width: 820, Height: 520,
		MinWidth: 500, MinHeight: 300, TitleBarStyle: mygo.TitleBarDefault,
		Content: ui.View(func(c *ui.Context) {
			p := applyTheme(c, a.darkMode)
			text := a.logs.String()
			usedKB := float64(a.logs.byteLen()) / 1024
			if scroll.Y >= scroll.MaxY-4 {
				scroll.Y = math.MaxFloat32
			}
			ui.Column(c).Fill().Padding(16).Gap(12).Background(p.canvas).Children(func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					livePulseDot(c, p.live, 10)
					ui.Text(c, "实时日志").FontSize(17).Bold().TextColor(p.title)
					ui.Badge(c, fmt.Sprintf("%.1f / 64 KiB", usedKB)).
						Background(p.accentSoft).TextColor(p.accent).
						ContentTransition(ui.ContentTransition{Numeric: true})
					ui.Box(c).Grow(1)
					copyBtn := ui.Button(c, "").Disabled(text == "").Tooltip("复制当前日志到剪贴板").Children(func() {
						ui.Icon(c, iconCopy).FontSize(13)
						ui.Text(c, "复制日志").SingleLine()
					})
					if copyBtn.Clicked() {
						c.WriteClipboard(text)
						c.Toast("已复制实时日志")
					}
					clearBtn := ui.Button(c, "").Disabled(text == "").Tooltip("清空当前窗口日志并继续收集").Children(func() {
						ui.Icon(c, iconTrash).FontSize(13)
						ui.Text(c, "清空").SingleLine()
					})
					if clearBtn.Clicked() {
						a.logs.clear()
					}
				})
				ui.Text(c, "仅在窗口开启时收集，关闭后立即停止并清空，不写入文件；自动保留最近 64 KiB。").
					FontSize(12).TextColor(p.muted)
				ui.Box(c).Grow(1).FillWidth().Padding(8).Radius(12).
					Background(p.card).Border(1, p.border).
					Shadow(0, 2, 10, 0, p.cardShadow).
					Children(func() {
						ui.TextArea(c, &text).ReadOnly(true).FillWidth().Grow(1).TrackScroll(&scroll)
					})
			})
		}),
	})
	a.logWindow = win
	a.logs.setOnWrite(win.Invalidate)
	win.OnClosed(func() {
		a.logs.setOnWrite(nil)
		a.logs.setEnabled(false)
		a.logWindow = nil
	})
}
