package main

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

type uiPalette struct {
	canvas       ui.Color
	card         ui.Color
	surface      ui.Color
	surfaceHover ui.Color
	border       ui.Color
	borderStrong ui.Color
	title        ui.Color
	text         ui.Color
	muted        ui.Color
	accent       ui.Color
	accentDark   ui.Color
	accentSoft   ui.Color
	accentText   ui.Color
	live         ui.Color
	liveSoft     ui.Color
	warn         ui.Color
	warnSoft     ui.Color
	danger       ui.Color
	dangerSoft   ui.Color
	cardShadow   ui.Color
	pillShadow   ui.Color
	ctaShadow    ui.Color
}

func lightPalette() uiPalette {
	return uiPalette{
		canvas:       ui.Hex("#eef3f1"),
		card:         ui.Hex("#ffffff"),
		surface:      ui.Hex("#f3f7f5"),
		surfaceHover: ui.Hex("#ebf2ef"),
		border:       ui.Hex("#dce5e1"),
		borderStrong: ui.Hex("#c4d3cd"),
		title:        ui.Hex("#0f2621"),
		text:         ui.Hex("#1a2f2a"),
		muted:        ui.Hex("#5d716b"),
		accent:       ui.Hex("#0f766e"),
		accentDark:   ui.Hex("#0b5b55"),
		accentSoft:   ui.Hex("#e4f4f0"),
		accentText:   ui.Hex("#ffffff"),
		live:         ui.Hex("#10b981"),
		liveSoft:     ui.Hex("#e6f9f1"),
		warn:         ui.Hex("#b45309"),
		warnSoft:     ui.Hex("#fef3c7"),
		danger:       ui.Hex("#dc2626"),
		dangerSoft:   ui.Hex("#fef2f2"),
		cardShadow:   ui.RGBA(15, 38, 33, 0.055),
		pillShadow:   ui.RGBA(15, 38, 33, 0.09),
		ctaShadow:    ui.RGBA(15, 118, 110, 0.24),
	}
}

func darkPalette() uiPalette {
	return uiPalette{
		canvas:       ui.Hex("#0f1514"),
		card:         ui.Hex("#17201e"),
		surface:      ui.Hex("#1f2b28"),
		surfaceHover: ui.Hex("#263532"),
		border:       ui.Hex("#2b3b37"),
		borderStrong: ui.Hex("#3a4f4a"),
		title:        ui.Hex("#eef7f4"),
		text:         ui.Hex("#dce9e5"),
		muted:        ui.Hex("#8ca39d"),
		accent:       ui.Hex("#14b8a6"),
		accentDark:   ui.Hex("#0d9488"),
		accentSoft:   ui.Hex("#133430"),
		accentText:   ui.Hex("#06201c"),
		live:         ui.Hex("#34d399"),
		liveSoft:     ui.Hex("#103229"),
		warn:         ui.Hex("#fbbf24"),
		warnSoft:     ui.Hex("#332710"),
		danger:       ui.Hex("#f87171"),
		dangerSoft:   ui.Hex("#331719"),
		cardShadow:   ui.RGBA(0, 0, 0, 0.28),
		pillShadow:   ui.RGBA(0, 0, 0, 0.36),
		ctaShadow:    ui.RGBA(20, 184, 166, 0.25),
	}
}

func applyTheme(c *ui.Context, dark bool) uiPalette {
	var t ui.Theme
	var p uiPalette
	if dark {
		t = *ui.DarkTheme()
		p = darkPalette()
	} else {
		t = *ui.LightTheme()
		p = lightPalette()
	}
	t.Accent = p.accent
	t.AccentText = p.accentText
	t.Background = p.card
	t.Surface = p.surface
	t.SurfaceHover = p.surfaceHover
	t.Border = p.border
	t.Text = p.text
	t.TextMuted = p.muted
	t.Danger = p.danger
	t.Radius = 9
	t.FontSize = 13.5
	t.TooltipDelay = 350 * time.Millisecond
	c.SetTheme(&t)
	return p
}

var (
	colorAnim      = ui.Animation{Duration: 180 * time.Millisecond, Ease: ui.EaseOut, Oklab: true}
	surfaceMotion  = ui.ElementTransition{Colors: true, Shadows: true, Animation: &colorAnim}
	panelEnterExit = ui.ElementTransition{
		Animation: &ui.Snappy,
		Enter:     &ui.Motion{Y: -6},
		Exit:      &ui.Motion{Y: -4},
	}
	cardCollapseMotion = ui.ElementTransition{
		Animation: &ui.Snappy,
		Enter:     &ui.Motion{Collapse: true, Y: -6},
		Exit:      &ui.Motion{Collapse: true, Y: -6},
	}
)

type segPillMatch struct{ group string }

type segOption struct {
	label string
	icon  *ui.SVG
}

func segmentedControl(c *ui.Context, p uiPalette, group string, selected *int, disabled, fill, slide bool, opts ...segOption) bool {
	parts := ui.SegmentedBase(c.Key("seg-"+group), selected, len(opts))
	track := parts.Track.Padding(3).Gap(3).Radius(10).Background(p.surface).Border(1, p.border).
		AlignItems(ui.Stretch).Transition(surfaceMotion)
	track.Children(func() {
		for i, opt := range opts {
			active := i == *selected
			seg := parts.Segment(i).Padding(6, 12).Radius(7).Disabled(disabled).Cursor(ui.CursorPointer)
			if !slide {
				seg.Transition(surfaceMotion)
				if active {
					seg.Background(p.card).Border(1, p.border).Shadow(0, 1, 4, 0, p.pillShadow)
				}
			}
			seg.Children(func() {
				if slide && active {
					ui.Box(c.Key("pill")).Absolute().Fill().ZIndex(-1).PassThrough().Radius(7).
						Background(p.card).Border(1, p.border).
						Shadow(0, 1, 4, 0, p.pillShadow).
						Transition(ui.ElementTransition{Animation: &ui.Snappy, Match: segPillMatch{group}})
				}
				ui.Row(c).Gap(6).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					textColor := p.muted
					if active {
						textColor = p.accent
					} else if seg.Hovered() && !disabled {
						textColor = p.text
					}
					if opt.icon != nil {
						ui.Icon(c, opt.icon).FontSize(14).TextColor(textColor)
					}
					label := ui.Text(c, opt.label).SingleLine().FontSize(13).TextColor(textColor)
					if active {
						label.Bold()
					}
				})
			})
		}
	})
	if fill {
		track.FillEqually()
	}
	return track.Changed()
}

func livePulseDot(c *ui.Context, color ui.Color, size float32) {
	ui.Box(c).Size(size, size).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
		phase := float32(p.Now().UnixMilli()%1800) / 1800
		if !p.Preferences().ReduceMotion {
			p.AnimationFrame()
		} else {
			phase = 0.25
		}
		haloScale := 0.55 + 0.45*ui.EaseOut(phase)
		haloAlpha := (1 - phase) * 0.42
		haloD := size * haloScale
		p.Fill(ui.Rect{
			X: r.X + (r.W-haloD)/2,
			Y: r.Y + (r.H-haloD)/2,
			W: haloD, H: haloD,
		}, ui.RGBA(color.R, color.G, color.B, haloAlpha), haloD/2)
		coreD := size * 0.56
		p.Fill(ui.Rect{
			X: r.X + (r.W-coreD)/2,
			Y: r.Y + (r.H-coreD)/2,
			W: coreD, H: coreD,
		}, color, coreD/2)
	})
}

func switchRow(c *ui.Context, p uiPalette, key string, on *bool, title, hint string, disabled bool) {
	row := ui.SwitchBase(c.Key(key), on).FillWidth().Padding(7, 10).Gap(10).Radius(9).
		AlignItems(ui.Center).Disabled(disabled).Cursor(ui.CursorPointer)
	bg := p.surface
	if *on {
		bg = p.accentSoft
	} else if row.Hovered() && !disabled {
		bg = p.surfaceHover
	}
	borderColor := p.border
	if *on {
		borderColor = p.accent.Mix(p.border, 0.55)
	}
	row.Background(bg).Border(1, borderColor).Transition(surfaceMotion)

	target := float32(0)
	if *on {
		target = 1
	}
	knobPos := row.AnimateTo("knob", target, ui.Snappy)
	row.Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			label := ui.Text(c, title).FontSize(13).TextColor(p.text)
			if *on {
				label.Bold()
			}
			if hint != "" {
				ui.Text(c, hint).FontSize(11).TextColor(p.muted)
			}
		})
		ui.Box(c).Size(36, 20).Shrink(0).PassThrough().Draw(func(painter *ui.Painter, r ui.Rect) {
			clamped := max(float32(0), min(float32(1), knobPos))
			trackOff := p.borderStrong
			trackColor := trackOff.MixOklab(p.accent, clamped)
			painter.Fill(r, trackColor, r.H/2)
			inset := float32(2.5)
			d := r.H - 2*inset
			knob := ui.Rect{
				X: r.X + inset + clamped*(r.W-r.H),
				Y: r.Y + inset,
				W: d, H: d,
			}
			painter.Shadow(knob, d/2, 0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.22))
			painter.Fill(knob, ui.RGB(255, 255, 255), d/2)
		})
	})
}

func radioDot(c *ui.Context, p uiPalette, active bool) {
	dot := ui.Box(c).Size(16, 16).Shrink(0).Radius(8).Center().
		Border(1.5, p.borderStrong).Transition(surfaceMotion)
	if active {
		dot.Border(1.5, p.accent).Background(p.accentSoft)
	}
	scale := float32(0)
	if active {
		scale = 1
	}
	s := dot.AnimateTo("dot", scale, ui.Bouncy)
	dot.Draw(func(painter *ui.Painter, r ui.Rect) {
		if s <= 0.02 {
			return
		}
		d := float32(8) * min(float32(1.15), max(float32(0), s))
		painter.Fill(ui.Rect{
			X: r.X + (r.W-d)/2,
			Y: r.Y + (r.H-d)/2,
			W: d, H: d,
		}, p.accent, d/2)
	})
}

func (a *application) currentModeSummary() string {
	if a.playback.AudioOnly {
		codec := strings.ToUpper(a.playback.audioCodec())
		if a.playback.losslessAudio() {
			return codec + " 无损"
		}
		if a.audioPreset == len(audioPresets) {
			return fmt.Sprintf("%s · %s Kbps", codec, strings.TrimSpace(a.customAudio))
		}
		return fmt.Sprintf("%s · %d Kbps", codec, audioPresets[a.audioPreset].BitrateKbps)
	}
	if a.preset == len(presets) {
		s := a.customSettings()
		res := strings.TrimSpace(s.Size) + "px"
		if strings.TrimSpace(s.Size) == "0" {
			res = "原生"
		}
		return fmt.Sprintf("自定义 · %s · %sfps", res, strings.TrimSpace(s.FPS))
	}
	return presets[a.preset].Name
}

func (a *application) view(c *ui.Context) {
drain:
	for {
		select {
		case update := <-a.updates:
			update()
		default:
			break drain
		}
	}
	// If no window invalidation hook is attached (headless tests) or QR countdown
	// is ticking, schedule lightweight periodic ticks.
	if a.pairQR != nil && a.pairQR.running {
		c.After(time.Second)
	} else if a.notify.Load() == nil && (a.busy || len(a.sessions) > 0) {
		c.After(100 * time.Millisecond)
	}

	p := applyTheme(c, a.darkMode)
	disabled := a.busy || a.b == nil
	settingsDisabled := disabled
	if a.window != nil {
		if c.Shortcut(0, ui.KeyF11) {
			a.window.ToggleFullScreen()
		}
		if !a.showPair && a.window.IsFullScreen() && c.Shortcut(0, ui.KeyEscape) {
			a.window.SetFullScreen(false)
		}
	}

	width, _ := c.Size()
	deviceWidth := min(float32(312), max(float32(256), width*0.265))
	settingsColumns := 1
	if width-28-14-deviceWidth-28 >= 590 {
		settingsColumns = 2
	}

	ui.Column(c).Fill().Padding(14).Gap(10).Background(p.canvas).Transition(surfaceMotion).Children(func() {
		// Top Header Bar
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Grow(1).Gap(11).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(38, 38).Radius(11).Center().Shrink(0).
					Gradient(p.accent, p.accentDark, 135).
					Shadow(0, 3, 10, 0, p.ctaShadow).
					Children(func() {
						ui.Icon(c, iconBrand).FontSize(20).TextColor(ui.RGB(255, 255, 255))
					})
				ui.Column(c).Gap(1).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "scrcpy LAN").FontSize(21).Bold().TextColor(p.title)
						ui.Badge(c, version).Background(p.accentSoft).TextColor(p.accent)
					})
					ui.Text(c, "USB 有线 / 局域网无线投屏").FontSize(12).TextColor(p.muted)
				})
			})
			ui.Column(c).Gap(3).AlignItems(ui.End).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					connMode := 0
					if a.wired {
						connMode = 1
					}
					if segmentedControl(c, p, "conn-mode", &connMode, settingsDisabled, false, true,
						segOption{"无线 / 局域网", iconWifi},
						segOption{"USB 有线", iconUSB},
					) {
						a.setMode(connMode == 1)
					}
					themeIcon := iconMoon
					themeTip := "切换深色外观"
					if a.darkMode {
						themeIcon = iconSun
						themeTip = "切换浅色外观"
					}
					themeBtn := ui.ButtonBase(c.Key("theme-toggle")).Size(34, 34).Radius(9).Center().
						Background(p.card).Border(1, p.border).
						Shadow(0, 1, 3, 0, p.cardShadow).
						Label(themeTip).Tooltip(themeTip).Cursor(ui.CursorPointer).
						Transition(surfaceMotion)
					if themeBtn.Hovered() {
						themeBtn.Background(p.surfaceHover)
					}
					themeBtn.Children(func() {
						ui.Icon(c, themeIcon).FontSize(15).TextColor(p.text).
							ContentTransition(ui.ContentTransition{})
					})
					if themeBtn.Clicked() {
						a.setDarkMode(!a.darkMode)
					}
				})
				ui.Text(c, "插入数据线或切换连接方式后，请重新扫描。").FontSize(11).TextColor(p.muted)
			})
		})

		// Main Two-Column Workspace
		devices := a.visibleDevices()
		ui.Row(c).Grow(1).Gap(14).AlignItems(ui.Stretch).Children(func() {
			// Left Sidebar (Independent Scroll)
			ui.Scroll(c).Width(deviceWidth).Shrink(0).Children(func() {
				ui.Column(c).Gap(10).Children(func() {
					// Active Sessions Card
					if len(a.sessions) > 0 {
						ui.Column(c.Key("active-sessions")).Padding(14).Gap(9).Radius(13).
							Background(p.card).Border(1, p.accent.Mix(p.border, 0.55)).
							Shadow(0, 2, 10, 0, p.cardShadow).
							Transition(cardCollapseMotion).
							Children(func() {
								ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
									livePulseDot(c, p.live, 10)
									ui.Textf(c, "运行中的设备 · %d", len(a.sessions)).FontSize(15).Bold().TextColor(p.title).
										ContentTransition(ui.ContentTransition{Numeric: true})
								})
								for _, session := range a.sessions {
									ui.Column(c.Key("session-"+session.key)).Padding(10).Gap(5).Radius(9).
										Background(p.surface).Border(1, p.border).
										Transition(cardCollapseMotion).
										Children(func() {
											ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
												ui.Text(c, session.name).Bold().FontSize(13).SingleLine().Grow(1).TextColor(p.title)
												ui.Badge(c, "运行中").Background(p.liveSoft).TextColor(p.live)
											})
											ui.Text(c, session.serial).FontSize(11).TextColor(p.muted).SingleLine()
											ui.Text(c, session.label).FontSize(11.5).TextColor(p.accent)
											label := "停止此设备"
											if session.stopping {
												label = "正在停止…"
											}
											if ui.Button(c, label).Disabled(session.stopping).FillWidth().Clicked() {
												a.stop(session)
											}
										})
								}
							})
					}

					// Discovered Devices Card
					ui.Column(c).Padding(14).Gap(9).Radius(13).
						Background(p.card).Border(1, p.border).
						Shadow(0, 2, 10, 0, p.cardShadow).
						Transition(surfaceMotion).
						Children(func() {
							ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
								title := "附近的设备"
								if a.wired {
									title = "USB 设备"
								}
								ui.Text(c, title).FontSize(15.5).Bold().Grow(1).TextColor(p.title)
								scanBtn := ui.Button(c, "").Disabled(disabled).Tooltip("刷新设备列表").Children(func() {
									ic := ui.Icon(c, iconRefresh).FontSize(13)
									if a.busy {
										ic.Rotate(ic.Loop("scan-spin", 900*time.Millisecond, ui.Linear) * 360)
									}
									ui.Text(c, "重新扫描").FontSize(12.5).SingleLine()
								})
								if scanBtn.Clicked() {
									a.discover()
								}
							})
							if a.wired {
								ui.Text(c, "使用数据线连接手机，开启 USB 调试，并在手机上允许授权。").FontSize(11.5).TextColor(p.muted)
							} else if len(a.networks) > 0 {
								ui.Text(c, "自动发现无线调试；同时扫描所选网段的 5555 端口。").FontSize(11.5).TextColor(p.muted)
								ui.Select(c, &a.network, a.networks).Label("本地扫描网段").Disabled(disabled).FillWidth()
							} else {
								ui.Text(c, "未检测到本地 IPv4 网段，仍可通过 mDNS 或手动连接发现设备。").FontSize(11.5).TextColor(p.muted)
							}
							ui.Divider(c)
							if len(devices) == 0 {
								ui.Column(c).Padding(10).Gap(6).Radius(9).Background(p.surface).Children(func() {
									ui.Text(c, "等待你的 Android 设备").FontSize(14).Bold().TextColor(p.title)
									steps := "1. 手机与电脑连接同一个 Wi-Fi\n2. 开发者选项 → 开启无线调试\n3. 首次使用，点击下方打开配对"
									if a.wired {
										steps = "1. 用 USB 数据线连接手机与电脑\n2. 开发者选项 → 开启 USB 调试\n3. 允许授权后，点击重新扫描"
									}
									ui.Text(c, steps).FontSize(11.5).TextColor(p.muted).LineHeight(1.55)
								})
							}
							for _, d := range devices {
								a.deviceCard(c, p, d)
							}
							ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
								ui.Textf(c, "%d 个设备 / 服务", len(devices)).FontSize(11.5).TextColor(p.muted).
									ContentTransition(ui.ContentTransition{Numeric: true})
							})
						})

					// Wireless Helpers
					if !a.wired {
						ui.Column(c.Key("manual-connect")).Padding(14).Gap(8).Radius(13).
							Background(p.card).Border(1, p.border).
							Shadow(0, 2, 10, 0, p.cardShadow).
							Transition(surfaceMotion).
							Children(func() {
								ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
									ui.Icon(c, iconLink).FontSize(14).TextColor(p.accent)
									ui.Text(c, "手动连接").FontSize(15).Bold().TextColor(p.title)
								})
								ui.Text(c, "填写无线调试主页上的 IP:端口；旧版 ADB 默认端口为 5555。").FontSize(11.5).TextColor(p.muted)
								addrInput := ui.TextInput(c, &a.address).Label("连接地址").Placeholder("192.168.1.10:37123").Disabled(disabled).FillWidth()
								if addrInput.Submitted() && !disabled && strings.TrimSpace(a.address) != "" {
									a.connect()
								}
								if ui.Button(c, "连接设备").Disabled(disabled || a.address == "").FillWidth().Clicked() {
									a.connect()
								}
							})
						ui.Column(c.Key("wireless-pair")).Padding(14).Gap(8).Radius(13).
							Background(p.card).Border(1, p.border).
							Shadow(0, 2, 10, 0, p.cardShadow).
							Transition(surfaceMotion).
							Children(func() {
								ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
									ui.Icon(c, iconQR).FontSize(14).TextColor(p.accent)
									ui.Text(c, "首次无线配对").FontSize(15).Bold().TextColor(p.title)
								})
								ui.Text(c, "Android 11+：支持二维码扫码或 6 位配对码配对。").FontSize(11.5).TextColor(p.muted)
								if ui.Button(c, "打开配对").Disabled(disabled).FillWidth().Clicked() {
									a.showPair = true
								}
							})
					}
				})
			})

			// Right Control Deck (Independent Scroll)
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Padding(14).Gap(10).Radius(13).
					Background(p.card).Border(1, p.border).
					Shadow(0, 2, 10, 0, p.cardShadow).
					Transition(surfaceMotion).
					Children(func() {
						// Deck Mode Header
						ui.Row(c).Gap(10).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
							mediaMode := 0
							if a.playback.AudioOnly {
								mediaMode = 1
							}
							if segmentedControl(c, p, "media-mode", &mediaMode, settingsDisabled, false, true,
								segOption{"画面和声音", iconMonitor},
								segOption{"仅音频", iconAudio},
							) {
								a.playback.AudioOnly = mediaMode == 1
							}
							ui.Badge(c, a.currentModeSummary()).
								Background(p.accentSoft).TextColor(p.accent).
								ContentTransition(ui.ContentTransition{Morph: true})
						})

						// Settings Grid
						ui.Grid(c).Columns(settingsColumns).Gap(14).AlignItems(ui.Start).Children(func() {
							// Column 1: Video Presets or Audio Codec/Bitrate
							ui.Column(c).Gap(7).Children(func() {
								if a.playback.AudioOnly {
									a.audioSettingsColumn(c, p, settingsDisabled)
								} else {
									a.videoPresetsColumn(c, p, settingsDisabled)
								}
							})
							// Column 2: Playback Settings
							ui.Column(c).Gap(6).Children(func() {
								a.playbackSettingsColumn(c, p, settingsDisabled)
							})
						})
					})
			})
		})

		// Fixed Bottom Action Dock
		ui.Column(c).Padding(11, 14).Gap(6).Radius(12).
			Background(p.card).Border(1, p.border).
			Shadow(0, 2, 12, 0, p.cardShadow).
			Transition(surfaceMotion).
			Children(func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Row(c).Grow(1).MinWidth(0).Gap(8).AlignItems(ui.Center).Children(func() {
						if a.busy {
							ui.Spinner(c).Label("操作进行中")
						} else if len(a.sessions) > 0 {
							livePulseDot(c, p.live, 10)
						} else if a.errorText != "" {
							ui.Icon(c, iconAlert).FontSize(15).TextColor(p.danger)
						}
						ui.Text(c, a.status).Bold().FontSize(14).TextColor(p.title).
							ContentTransition(ui.ContentTransition{Morph: true})
					})
					logBtn := ui.Button(c, "").Tooltip("查看实时 ADB 与 scrcpy 诊断日志").Children(func() {
						ui.Icon(c, iconTerminal).FontSize(13)
						ui.Text(c, "打开日志窗口").FontSize(13).SingleLine()
					})
					if logBtn.Clicked() {
						a.openLogWindow()
					}
					if session := a.selectedSession(); session == nil {
						pairing := false
						for _, d := range devices {
							if d.key() == a.selected {
								pairing = d.Pairing
								break
							}
						}
						label := "开始投屏"
						if a.playback.AudioOnly {
							label = "开始音频"
						}
						ctaDisabled := disabled || a.selected == "" || pairing
						cta := ui.PrimaryButton(c, "").Disabled(ctaDisabled).Padding(8, 16).Radius(9).Children(func() {
							ui.Icon(c, iconPlay).FontSize(12)
							ui.Text(c, label).Bold().SingleLine()
						})
						if !ctaDisabled {
							cta.Shadow(0, 3, 10, 0, p.ctaShadow)
							if cta.Hovered() {
								cta.AnimatePose(ui.Pose{Y: -1}, ui.Snappy)
							} else {
								cta.AnimatePose(ui.Pose{}, ui.Snappy)
							}
						}
						if cta.Clicked() {
							a.start()
						}
					} else {
						label := "停止投屏"
						if session.playback.AudioOnly {
							label = "停止音频"
						}
						stopBtn := ui.Button(c, "").Disabled(session.stopping).Padding(8, 15).Radius(9).
							Background(p.dangerSoft).Border(1, p.danger.Mix(p.border, 0.45)).TextColor(p.danger).
							Children(func() {
								ui.Icon(c, iconStop).FontSize(11).TextColor(p.danger)
								ui.Text(c, label).Bold().TextColor(p.danger).SingleLine()
							})
						if stopBtn.Clicked() {
							a.stop(session)
						}
					}
					if len(a.sessions) > 1 {
						stopAll := ui.Button(c.Key("stop-all"), "停止全部").Transition(panelEnterExit)
						if stopAll.Clicked() {
							for _, session := range a.sessions {
								a.stop(session)
							}
						}
					}
					if a.busy {
						cancelBtn := ui.Button(c.Key("cancel-job"), "取消操作").Transition(panelEnterExit)
						if cancelBtn.Clicked() {
							a.cancelJob()
						}
					}
				})
				if a.detail != "" {
					ui.Text(c, a.detail).FontSize(11.5).TextColor(p.muted)
				}
				if a.errorText != "" {
					ui.Column(c.Key("error-box")).Padding(8).Radius(8).
						Background(p.dangerSoft).Border(1, p.danger.Mix(p.border, 0.5)).
						Transition(panelEnterExit).
						Children(func() {
							ui.TextArea(c, &a.errorText).ReadOnly(true).Height(88).TextColor(p.danger).Label("错误详情")
						})
				}
			})

		// Subtle Platform Footer
		platform := "Windows"
		if runtime.GOOS == "linux" {
			platform = "Linux"
		}
		ui.Text(c, fmt.Sprintf("scrcpy + mygo  ·  H.264  ·  %s 便携版  ·  %s", platform, version)).
			FontSize(11).TextColor(p.muted)
	})

	// Animated Pairing Modal
	a.pairModal(c, p, disabled)
}

func (a *application) deviceCard(c *ui.Context, p uiPalette, d device) {
	selected := a.selected == d.key()
	state := d.State
	badgeBg, badgeFg := p.accentSoft, p.accent
	running := false
	switch state {
	case "device":
		state = "已连接"
		badgeBg, badgeFg = p.accentSoft, p.accent
	case "unauthorized":
		state = "请在手机上授权"
		badgeBg, badgeFg = p.warnSoft, p.warn
	case "offline":
		state = "离线"
		badgeBg, badgeFg = p.surface, p.muted
	case "待配对":
		badgeBg, badgeFg = p.warnSoft, p.warn
	}
	for _, session := range a.sessions {
		if session.matches(d) {
			state = "运行中"
			running = true
			badgeBg, badgeFg = p.liveSoft, p.live
			break
		}
	}

	row := ui.ButtonBase(c.Key("dev-"+d.key())).FillWidth().Padding(9, 10).Radius(10).
		Cursor(ui.CursorPointer).Transition(surfaceMotion)
	if selected {
		row.Background(p.accentSoft).Border(1, p.accent).Shadow(0, 2, 8, 0, p.cardShadow)
	} else if row.Hovered() {
		row.Background(p.surfaceHover).Border(1, p.borderStrong)
	} else {
		row.Background(p.card).Border(1, p.border)
	}
	pose := ui.Pose{}
	if row.Hovered() && !selected {
		pose = ui.Pose{Y: -1}
	}
	row.AnimatePose(pose, ui.Snappy)
	row.Children(func() {
		ui.Row(c).FillWidth().Gap(9).AlignItems(ui.Center).Children(func() {
			iconBg := p.surface
			iconFg := p.muted
			if selected {
				iconBg, iconFg = p.card, p.accent
			}
			ui.Box(c).Size(30, 30).Radius(8).Center().Shrink(0).Background(iconBg).Children(func() {
				ic := iconPhone
				if d.Pairing {
					ic = iconQR
				}
				ui.Icon(c, ic).FontSize(15).TextColor(iconFg)
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, d.displayName()).Bold().FontSize(13).SingleLine().Grow(1).TextColor(p.title)
					if running {
						livePulseDot(c, p.live, 8)
					}
				})
				ui.Text(c, d.connectionLabel()).FontSize(11).TextColor(p.muted).SingleLine()
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					meta := state
					if d.Source != "" {
						meta = d.Source + " · " + state
					}
					ui.Text(c, meta).FontSize(11).TextColor(badgeFg).
						Padding(1, 6).Radius(5).Background(badgeBg)
				})
			})
		})
	})
	if row.Clicked() {
		a.selected = d.key()
		if d.Pairing {
			a.pairQRMode = false
			a.pairAddress, a.showPair = d.Address, true
		}
	}
}

func (a *application) videoPresetsColumn(c *ui.Context, p uiPalette, settingsDisabled bool) {
	ui.Text(c, "投屏档位").FontSize(16).Bold().TextColor(p.title)
	for i, pr := range presets {
		if pr.USBOnly && !a.wired {
			continue
		}
		active := a.preset == i
		btn := ui.ButtonBase(c.Key("preset-"+pr.Name)).FillWidth().Padding(9, 11).Radius(10).
			Disabled(settingsDisabled).Cursor(ui.CursorPointer).Transition(surfaceMotion)
		if active {
			btn.Background(p.accentSoft).Border(1, p.accent).Shadow(0, 2, 8, 0, p.cardShadow)
		} else if btn.Hovered() && !settingsDisabled {
			btn.Background(p.surfaceHover).Border(1, p.borderStrong)
		} else {
			btn.Background(p.card).Border(1, p.border)
		}
		pose := ui.Pose{}
		if btn.Hovered() && !settingsDisabled && !active {
			pose = ui.Pose{Y: -1}
		}
		btn.AnimatePose(pose, ui.Snappy)
		btn.Children(func() {
			ui.Row(c).FillWidth().Gap(9).AlignItems(ui.Center).Children(func() {
				radioDot(c, p, active)
				ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, pr.Name).Bold().FontSize(13.5).TextColor(p.title)
					ui.Text(c, pr.Detail).FontSize(11.5).TextColor(p.muted)
				})
			})
		})
		if btn.Clicked() {
			a.preset = i
		}
	}
	customActive := a.preset == len(presets)
	customBtn := ui.ButtonBase(c.Key("preset-custom")).FillWidth().Padding(8, 11).Radius(10).
		Disabled(settingsDisabled).Cursor(ui.CursorPointer).Transition(surfaceMotion)
	if customActive {
		customBtn.Background(p.accentSoft).Border(1, p.accent)
	} else if customBtn.Hovered() && !settingsDisabled {
		customBtn.Background(p.surfaceHover).Border(1, p.borderStrong)
	} else {
		customBtn.Background(p.surface).Border(1, p.border)
	}
	customBtn.Children(func() {
		ui.Row(c).FillWidth().Gap(9).AlignItems(ui.Center).Children(func() {
			radioDot(c, p, customActive)
			ui.Text(c, "自定义").Bold().FontSize(13).TextColor(p.title)
		})
	})
	if customBtn.Clicked() {
		a.preset = len(presets)
	}
	if customActive {
		settings := a.customSettings()
		ui.Column(c.Key("custom-video-fields")).Padding(10).Gap(7).Radius(10).
			Background(p.surface).Border(1, p.border).Transition(panelEnterExit).
			Children(func() {
				ui.TextInput(c, &settings.Size).Label("分辨率长边（px，0 为原生）").Disabled(settingsDisabled).FillWidth()
				ui.TextInput(c, &settings.FPS).Label("帧率上限（fps）").Disabled(settingsDisabled).FillWidth()
				ui.TextInput(c, &settings.BitrateMbps).Label("视频码率（Mbps）").Disabled(settingsDisabled).FillWidth()
			})
	}
	ui.Text(c, "设置用于下次启动，不影响运行中的设备。更改已有设备档位需单独停止后重启。").FontSize(11.5).TextColor(p.muted)
}

func (a *application) audioSettingsColumn(c *ui.Context, p uiPalette, settingsDisabled bool) {
	ui.Text(c, "音频编码").FontSize(16).Bold().TextColor(p.title)
	ui.Select(c, &a.playback.AudioCodec, []string{"opus", "aac", "flac", "raw"}).Label("音频编码格式").Disabled(settingsDisabled).FillWidth()
	ui.Text(c, "Opus：日常推荐；AAC：兼容选择；FLAC：无损压缩，音质优先；RAW：未压缩，带宽更高。").FontSize(11.5).TextColor(p.muted)
	if !a.playback.losslessAudio() {
		ui.Text(c, "音频码率").FontSize(16).Bold().TextColor(p.title)
		for i, ap := range audioPresets {
			active := a.audioPreset == i
			btn := ui.ButtonBase(c.Key("audio-"+ap.Name)).FillWidth().Padding(8, 11).Radius(9).
				Disabled(settingsDisabled).Cursor(ui.CursorPointer).Transition(surfaceMotion)
			if active {
				btn.Background(p.accentSoft).Border(1, p.accent)
			} else if btn.Hovered() && !settingsDisabled {
				btn.Background(p.surfaceHover).Border(1, p.borderStrong)
			} else {
				btn.Background(p.card).Border(1, p.border)
			}
			btn.Children(func() {
				ui.Row(c).FillWidth().Gap(9).AlignItems(ui.Center).Children(func() {
					radioDot(c, p, active)
					ui.Text(c, ap.Name).FontSize(13).TextColor(p.title)
				})
			})
			if btn.Clicked() {
				a.audioPreset = i
			}
		}
		customActive := a.audioPreset == len(audioPresets)
		custom := ui.ButtonBase(c.Key("audio-custom")).FillWidth().Padding(8, 11).Radius(9).
			Disabled(settingsDisabled).Cursor(ui.CursorPointer).Transition(surfaceMotion)
		if customActive {
			custom.Background(p.accentSoft).Border(1, p.accent)
		} else if custom.Hovered() && !settingsDisabled {
			custom.Background(p.surfaceHover).Border(1, p.borderStrong)
		} else {
			custom.Background(p.surface).Border(1, p.border)
		}
		custom.Children(func() {
			ui.Row(c).FillWidth().Gap(9).AlignItems(ui.Center).Children(func() {
				radioDot(c, p, customActive)
				ui.Text(c, "自定义音频码率").Bold().FontSize(13).TextColor(p.title)
			})
		})
		if custom.Clicked() {
			a.audioPreset = len(audioPresets)
		}
		if customActive {
			ui.Column(c.Key("custom-audio-field")).Padding(10).Gap(6).Radius(10).
				Background(p.surface).Border(1, p.border).Transition(panelEnterExit).
				Children(func() {
					ui.TextInput(c, &a.customAudio).Label("音频码率（Kbps，6–9000）").Disabled(settingsDisabled).FillWidth()
				})
		}
		ui.Text(c, "默认 128 Kbps。高码率请求可能被编码器限制或拒绝；追求音质可选 FLAC。").FontSize(11.5).TextColor(p.muted)
	} else {
		ui.Text(c, "无损模式无需设置码率。FLAC 需手机支持对应编码器。").FontSize(11.5).TextColor(p.muted)
	}
	ui.Text(c, "仅转发声音，不打开投屏窗口。需 Android 11+；Android 11 启动前请解锁。").FontSize(11.5).TextColor(p.muted)
}

func (a *application) playbackSettingsColumn(c *ui.Context, p uiPalette, settingsDisabled bool) {
	ui.Text(c, "播放设置").FontSize(16).Bold().TextColor(p.title)
	bufferLabel := "音视频缓存"
	if a.playback.AudioOnly {
		bufferLabel = "音频缓存"
	}
	ui.Text(c, bufferLabel).Bold().FontSize(13).TextColor(p.text)
	ui.Row(c).Gap(6).Wrap().Children(func() {
		for _, ms := range []int{0, 500, 1000, 2000} {
			label := "关闭缓存"
			if ms > 0 {
				label = bufferSeconds(ms) + " 秒缓存"
			}
			active := !a.useCustomBuffer && a.playback.BufferMS == ms
			chip := ui.ButtonBase(c.Key(fmt.Sprintf("buf-%d", ms))).Padding(5, 10).Radius(8).
				Disabled(settingsDisabled).Cursor(ui.CursorPointer).Transition(surfaceMotion)
			if active {
				chip.Background(p.accentSoft).Border(1, p.accent)
			} else if chip.Hovered() && !settingsDisabled {
				chip.Background(p.surfaceHover).Border(1, p.borderStrong)
			} else {
				chip.Background(p.surface).Border(1, p.border)
			}
			chip.Children(func() {
				tx := ui.Text(c, label).FontSize(12.5).SingleLine()
				if active {
					tx.Bold().TextColor(p.accent)
				} else {
					tx.TextColor(p.text)
				}
			})
			if chip.Clicked() {
				a.playback.BufferMS = ms
				a.useCustomBuffer = false
			}
		}
		customChip := ui.ButtonBase(c.Key("buf-custom")).Padding(5, 10).Radius(8).
			Disabled(settingsDisabled).Cursor(ui.CursorPointer).Transition(surfaceMotion)
		if a.useCustomBuffer {
			customChip.Background(p.accentSoft).Border(1, p.accent)
		} else if customChip.Hovered() && !settingsDisabled {
			customChip.Background(p.surfaceHover).Border(1, p.borderStrong)
		} else {
			customChip.Background(p.surface).Border(1, p.border)
		}
		customChip.Children(func() {
			tx := ui.Text(c, "自定义缓存").FontSize(12.5).SingleLine()
			if a.useCustomBuffer {
				tx.Bold().TextColor(p.accent)
			} else {
				tx.TextColor(p.text)
			}
		})
		if customChip.Clicked() {
			a.useCustomBuffer = true
		}
	})
	if a.useCustomBuffer {
		ui.Box(c.Key("custom-buffer-field")).FillWidth().Transition(panelEnterExit).Children(func() {
			ui.TextInput(c, &a.customBuffer).Label("缓存时间（秒，0–60）").Placeholder("例如 0.5；0 关闭额外缓存").Disabled(settingsDisabled).FillWidth()
		})
	}
	ui.Text(c, "缓存缓解抖动并增加延迟，默认 2 秒。自定义支持三位小数；下次启动时生效。").FontSize(11).TextColor(p.muted)
	if !a.playback.AudioOnly {
		ui.Select(c, &a.playback.ScaleMode, scaleModes).Label("画面缩放模式").Disabled(settingsDisabled).FillWidth()
	}
	screenHint := "结束后自动亮屏；Alt+O 熄屏，Alt+Shift+O 亮屏。"
	if a.playback.AudioOnly {
		screenHint = "勾选熄屏后，结束时自动请求重新亮屏。"
	}
	switchRow(c, p, "sw-screen-off", &a.playback.TurnScreenOff, "启动后熄灭设备屏幕（关闭屏幕电源）", screenHint, settingsDisabled)
	switchRow(c, p, "sw-mute-stop", &a.playback.MuteOnStop, "结束后静音设备媒体声音", "默认开启，结束后媒体音量设为 0；可用手机音量键调回。", settingsDisabled)
	if !a.playback.AudioOnly {
		switchRow(c, p, "sw-uhid", &a.playback.KeyboardUHID, "电脑键盘输入（UHID，支持手机输入法）", "点击输入框打字，Alt+K 配置实体键盘；不兼容时关闭。", settingsDisabled)
		switchRow(c, p, "sw-fullscreen", &a.playback.Fullscreen, "投屏启动时全屏（覆盖任务栏）", "F11 全屏后 Alt+Z 可框选显示区域；Esc 取消或退出。", settingsDisabled)
	}
}

func (a *application) pairModal(c *ui.Context, p uiPalette, disabled bool) {
	if a.showPair {
		w, h := c.Size()
		ui.DialogBase(c, &a.showPair, func(backdrop, panel ui.Element) {
			backdrop.Background(ui.RGBA(0, 0, 0, 0.45)).Transition(ui.ElementTransition{
				Duration: 150 * time.Millisecond,
				Enter:    &ui.Motion{},
				Exit:     &ui.Motion{},
			})
			panel.Width(420).MaxWidth(w-32).MaxHeight(h-32).
				Padding(18).Gap(9).Radius(16).
				Background(p.card).Border(1, p.border).
				Shadow(0, 14, 38, 0, ui.RGBA(0, 0, 0, 0.28)).
				Transition(ui.ElementTransition{
					Colors:    true,
					Animation: &ui.Snappy,
					Enter:     &ui.Motion{Scale: 0.96, Y: 8},
					Exit:      &ui.Motion{Scale: 0.96, Y: 6},
				})
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconQR).FontSize(19).TextColor(p.accent)
				ui.Text(c, "配对 Android 设备").FontSize(19).Bold().TextColor(p.title)
			})
			pairTab := 1
			if a.pairQRMode {
				pairTab = 0
			}
			if segmentedControl(c, p, "pair-mode", &pairTab, false, true, false,
				segOption{"二维码配对", iconQR},
				segOption{"配对码配对", iconLink},
			) {
				if pairTab == 0 && !a.pairQRMode {
					if !disabled || a.pairQR != nil {
						a.pairQRMode = true
						a.startQRPair()
					}
				} else if pairTab == 1 && a.pairQRMode {
					a.cancelQRPair()
					a.pairQRMode = false
				}
			}
			if a.pairQRMode {
				ui.Text(c, "手机与电脑连接同一 Wi-Fi，打开无线调试 →「使用二维码配对设备」扫描下方二维码。").FontSize(12).TextColor(p.muted)
				if q := a.pairQR; q != nil {
					if q.bitmap != nil {
						ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
							ui.Box(c).Padding(8).Radius(12).Background(ui.RGB(255, 255, 255)).
								Border(1, p.border).Shadow(0, 2, 10, 0, p.cardShadow).
								Children(func() {
									ui.Image(c, q.bitmap).Width(196).Height(196)
								})
						})
					}
					if q.running && !q.expiresAt.IsZero() {
						rem := max(time.Duration(0), time.Until(q.expiresAt))
						frac := float64(rem) / float64(qrPairTimeout)
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Spinner(c).Label("等待扫码")
							ui.Text(c, q.message).FontSize(12).TextColor(p.muted).Grow(1)
							ui.Badge(c, fmt.Sprintf("%d:%02d", int(rem.Minutes()), int(rem.Seconds())%60)).
								Background(p.accentSoft).TextColor(p.accent).
								ContentTransition(ui.ContentTransition{Numeric: true})
						})
						ui.Progress(c, frac).FillWidth()
					} else {
						ui.Text(c, q.message).FontSize(12).TextColor(p.muted)
					}
				}
			} else {
				ui.Text(c, "在手机的无线调试中打开「使用配对码配对设备」，填写该页面的 IP:端口及 6 位配对码。").FontSize(12.5).TextColor(p.muted)
				ui.TextInput(c, &a.pairAddress).Label("配对地址").Placeholder("例如 192.168.1.10:40123").FillWidth()
				codeInput := ui.TextInput(c, &a.pairCode).Label("配对码").Placeholder("6 位配对码").Password().FillWidth()
				if codeInput.Submitted() && !disabled && a.pairAddress != "" && a.pairCode != "" {
					a.pair()
				}
				ui.Text(c, "配对端口与投屏连接端口不同。").FontSize(11.5).TextColor(p.muted)
			}
			ui.Row(c).Gap(10).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Clicked() {
					a.closePair()
				}
				if a.pairQRMode {
					if ui.PrimaryButton(c, "重新生成二维码").Disabled(a.busy || a.b == nil).Clicked() {
						a.startQRPair()
					}
				} else if ui.PrimaryButton(c, "配对设备").Disabled(disabled || a.pairAddress == "" || a.pairCode == "").Clicked() {
					a.pair()
				}
			})
		})
	}
	// Modal also closes on Escape or a click outside its panel.
	if !a.showPair && (a.pairQR != nil || a.pairCode != "") {
		a.closePair()
	}
}
