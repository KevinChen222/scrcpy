package main

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
)

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
	if a.busy || len(a.sessions) > 0 {
		c.After(100 * time.Millisecond)
	}
	t := *c.Theme()
	t.Accent, t.Radius, t.FontSize = ui.Hex("#157a66"), 8, 14
	c.SetTheme(&t)
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
	deviceWidth := min(float32(320), max(float32(260), width*0.27))
	settingsColumns := 1
	if width-40-16-deviceWidth-34 >= 620 {
		settingsColumns = 2
	}
	ui.Column(c).Fill().Padding(20).Gap(14).Background(ui.Hex("#f3f6f5")).Children(func() {
		ui.Row(c).Gap(16).Children(func() {
			ui.Column(c).Grow(1).Gap(4).Children(func() {
				ui.Text(c, "scrcpy LAN").FontSize(26).Bold().TextColor(ui.Hex("#132f2a"))
				ui.Text(c, "USB 有线 / 局域网无线投屏 · "+version).TextColor(t.TextMuted)
			})
			ui.Column(c).Gap(6).Children(func() {
				ui.Row(c).Gap(12).Children(func() {
					for _, mode := range []struct {
						name  string
						wired bool
					}{{"无线 / 局域网", false}, {"USB 有线", true}} {
						button := ui.Button(c, mode.name).Disabled(settingsDisabled)
						if a.wired == mode.wired {
							button.Background(ui.Hex("#e7f4ef")).Border(1, t.Accent)
						}
						if button.Clicked() {
							a.setMode(mode.wired)
						}
					}
				})
				ui.Text(c, "插入数据线或切换连接方式后，请重新扫描。").FontSize(12).TextColor(t.TextMuted)
			})
		})
		devices := a.visibleDevices()
		ui.Row(c).Grow(1).Gap(16).AlignItems(ui.Stretch).Children(func() {
			ui.Scroll(c).Width(deviceWidth).Shrink(0).Children(func() {
				ui.Column(c).Gap(12).Children(func() {
					if len(a.sessions) > 0 {
						ui.Column(c).Padding(16).Gap(10).Background(t.Background).Radius(12).Border(1, t.Border).Children(func() {
							ui.Textf(c, "运行中的设备 · %d", len(a.sessions)).FontSize(17).Bold()
							for _, session := range a.sessions {
								ui.Column(c).Key("session-" + session.key).Gap(4).Children(func() {
									ui.Text(c, session.name).Bold().SingleLine()
									ui.Text(c, session.serial).FontSize(12).TextColor(t.TextMuted).SingleLine()
									ui.Text(c, session.label).FontSize(12).TextColor(t.Accent)
									label := "停止此设备"
									if session.stopping {
										label = "正在停止…"
									}
									if ui.Button(c, label).Disabled(session.stopping).Clicked() {
										a.stop(session)
									}
								})
							}
						})
					}
					ui.Column(c).Padding(16).Gap(10).Background(t.Background).Radius(12).Border(1, t.Border).Children(func() {
						ui.Row(c).Gap(10).Children(func() {
							title := "附近的设备"
							if a.wired {
								title = "USB 设备"
							}
							ui.Text(c, title).FontSize(17).Bold().Grow(1)
							if ui.Button(c, "重新扫描").Disabled(disabled).Clicked() {
								a.discover()
							}
						})
						if a.wired {
							ui.Text(c, "使用数据线连接手机，开启 USB 调试，并在手机上允许授权。").FontSize(12).TextColor(t.TextMuted)
						} else if len(a.networks) > 0 {
							ui.Text(c, "自动发现无线调试；同时扫描所选网段的 5555 端口。").FontSize(12).TextColor(t.TextMuted)
							ui.Select(c, &a.network, a.networks).Label("本地扫描网段").Disabled(disabled).FillWidth()
						} else {
							ui.Text(c, "未检测到本地 IPv4 网段，仍可通过 mDNS 或手动连接发现设备。").FontSize(12).TextColor(t.TextMuted)
						}
						ui.Divider(c)
						if len(devices) == 0 {
							ui.Column(c).Padding(12, 0).Gap(8).Children(func() {
								ui.Text(c, "等待你的 Android 设备").FontSize(17).Bold()
								steps := "1. 手机与电脑连接同一个 Wi-Fi\n2. 开发者选项 → 开启无线调试\n3. 首次使用，点击下方打开配对"
								if a.wired {
									steps = "1. 用 USB 数据线连接手机与电脑\n2. 开发者选项 → 开启 USB 调试\n3. 允许授权后，点击重新扫描"
								}
								ui.Text(c, steps).FontSize(12).TextColor(t.TextMuted).LineHeight(1.6)
							})
						}
						for _, d := range devices {
							ui.Box(c).Key(d.key()).Children(func() {
								row := ui.ButtonBase(c).FillWidth().Padding(10).Radius(8).Border(1, t.Border)
								if a.selected == d.key() {
									row.Background(ui.Hex("#e7f4ef")).Border(1, t.Accent)
								}
								row.Children(func() {
									ui.Column(c).Grow(1).Gap(3).Children(func() {
										ui.Text(c, d.Name).Bold().SingleLine()
										ui.Text(c, d.key()).FontSize(12).TextColor(t.TextMuted).SingleLine()
										state := d.State
										switch state {
										case "device":
											state = "已连接"
										case "unauthorized":
											state = "请在手机上授权"
										case "offline":
											state = "离线"
										}
										ui.Text(c, d.Source+" · "+state).FontSize(12).TextColor(t.Accent)
									})
								})
								if row.Clicked() {
									a.selected = d.key()
									if d.Pairing {
										a.pairAddress, a.showPair = d.Address, true
									}
								}
							})
						}
						ui.Textf(c, "%d 个设备 / 服务", len(devices)).FontSize(12).TextColor(t.TextMuted)
					})
					if !a.wired {
						ui.Column(c).Padding(16).Gap(10).Background(t.Background).Radius(12).Border(1, t.Border).Children(func() {
							ui.Text(c, "手动连接").FontSize(17).Bold()
							ui.Text(c, "填写无线调试主页上的 IP:端口；旧版 ADB 默认端口为 5555。").FontSize(12).TextColor(t.TextMuted)
							ui.TextInput(c, &a.address).Label("连接地址").Placeholder("192.168.1.10:37123").Disabled(disabled).FillWidth()
							if ui.Button(c, "连接设备").Disabled(disabled || a.address == "").Clicked() {
								a.connect()
							}
						})
						ui.Column(c).Padding(16).Gap(10).Background(t.Background).Radius(12).Border(1, t.Border).Children(func() {
							ui.Text(c, "首次无线配对").FontSize(17).Bold()
							ui.Text(c, "Android 11+：打开无线调试 → 使用配对码配对设备。配对端口与连接端口不同。").FontSize(12).TextColor(t.TextMuted)
							if ui.Button(c, "打开配对").Disabled(disabled).Clicked() {
								a.showPair = true
							}
						})
					}
				})
			})
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Padding(16).Gap(14).Background(t.Background).Radius(12).Border(1, t.Border).Children(func() {
					ui.Row(c).Gap(10).Children(func() {
						for _, mode := range []struct {
							name      string
							audioOnly bool
						}{{"画面和声音", false}, {"仅音频", true}} {
							button := ui.Button(c, mode.name).Disabled(settingsDisabled)
							if a.playback.AudioOnly == mode.audioOnly {
								button.Background(ui.Hex("#e7f4ef")).Border(1, t.Accent)
							}
							if button.Clicked() {
								a.playback.AudioOnly = mode.audioOnly
							}
						}
					})
					ui.Grid(c).Columns(settingsColumns).Gap(18).AlignItems(ui.Start).Children(func() {
						ui.Column(c).Gap(10).Children(func() {
							if a.playback.AudioOnly {
								ui.Text(c, "音频编码").FontSize(19).Bold()
								ui.Select(c, &a.playback.AudioCodec, []string{"opus", "aac", "flac", "raw"}).Label("音频编码格式").Disabled(settingsDisabled).FillWidth()
								ui.Text(c, "Opus：日常推荐；AAC：兼容选择；FLAC：无损压缩，音质优先；RAW：未压缩，带宽更高。").FontSize(12).TextColor(t.TextMuted)
								if !a.playback.losslessAudio() {
									ui.Text(c, "音频码率").FontSize(19).Bold()
									for i, p := range audioPresets {
										button := ui.Button(c, p.Name).Disabled(settingsDisabled).FillWidth()
										if a.audioPreset == i {
											button.Background(ui.Hex("#e7f4ef")).Border(1, t.Accent)
										}
										if button.Clicked() {
											a.audioPreset = i
										}
									}
									custom := ui.Button(c, "自定义音频码率").Disabled(settingsDisabled).FillWidth()
									if a.audioPreset == len(audioPresets) {
										custom.Background(ui.Hex("#e7f4ef")).Border(1, t.Accent)
									}
									if custom.Clicked() {
										a.audioPreset = len(audioPresets)
									}
									if a.audioPreset == len(audioPresets) {
										ui.TextInput(c, &a.customAudio).Label("音频码率（Kbps，6–9000）").Disabled(settingsDisabled).FillWidth()
									}
									ui.Text(c, "默认 128 Kbps。高码率请求可能被编码器限制或拒绝；追求音质可选 FLAC。").FontSize(12).TextColor(t.TextMuted)
								} else {
									ui.Text(c, "无损模式无需设置码率。FLAC 需手机支持对应编码器。").FontSize(12).TextColor(t.TextMuted)
								}
								ui.Text(c, "仅转发声音，不打开投屏窗口。需 Android 11+；Android 11 启动前请解锁。").FontSize(12).TextColor(t.TextMuted)
							} else {
								ui.Text(c, "投屏档位").FontSize(19).Bold()
								for i, p := range presets {
									if p.USBOnly && !a.wired {
										continue
									}
									ui.Box(c).Key(p.Name).Children(func() {
										button := ui.ButtonBase(c).FillWidth().Padding(12).Radius(8).Border(1, t.Border).Disabled(settingsDisabled)
										if a.preset == i {
											button.Background(ui.Hex("#e7f4ef")).Border(1, t.Accent)
										}
										button.Children(func() {
											ui.Column(c).Gap(5).Grow(1).Children(func() {
												ui.Text(c, p.Name).Bold()
												ui.Text(c, p.Detail).FontSize(12).TextColor(t.TextMuted)
											})
										})
										if button.Clicked() {
											a.preset = i
										}
									})
								}
								custom := ui.Button(c, "自定义").Disabled(settingsDisabled).FillWidth()
								if a.preset == len(presets) {
									custom.Background(ui.Hex("#e7f4ef")).Border(1, t.Accent)
								}
								if custom.Clicked() {
									a.preset = len(presets)
								}
								if a.preset == len(presets) {
									settings := a.customSettings()
									ui.TextInput(c, &settings.Size).Label("分辨率长边（px，0 为原生）").Disabled(settingsDisabled).FillWidth()
									ui.TextInput(c, &settings.FPS).Label("帧率上限（fps）").Disabled(settingsDisabled).FillWidth()
									ui.TextInput(c, &settings.BitrateMbps).Label("视频码率（Mbps）").Disabled(settingsDisabled).FillWidth()
								}
								ui.Text(c, "设置用于下次启动，不影响运行中的设备。更改已有设备档位需单独停止后重启。").FontSize(12).TextColor(t.TextMuted)
							}
						})
						ui.Column(c).Gap(10).Children(func() {
							ui.Text(c, "播放设置").FontSize(19).Bold()
							bufferLabel := "音视频缓存"
							if a.playback.AudioOnly {
								bufferLabel = "音频缓存"
							}
							ui.Text(c, bufferLabel).Bold()
							ui.Row(c).Gap(8).Wrap().Children(func() {
								for _, ms := range []int{0, 500, 1000, 2000} {
									label := "关闭缓存"
									if ms > 0 {
										label = bufferSeconds(ms) + " 秒缓存"
									}
									button := ui.Button(c, label).Disabled(settingsDisabled)
									if !a.useCustomBuffer && a.playback.BufferMS == ms {
										button.Background(ui.Hex("#e7f4ef")).Border(1, t.Accent)
									}
									if button.Clicked() {
										a.playback.BufferMS = ms
										a.useCustomBuffer = false
									}
								}
								custom := ui.Button(c, "自定义缓存").Disabled(settingsDisabled)
								if a.useCustomBuffer {
									custom.Background(ui.Hex("#e7f4ef")).Border(1, t.Accent)
								}
								if custom.Clicked() {
									a.useCustomBuffer = true
								}
							})
							if a.useCustomBuffer {
								ui.TextInput(c, &a.customBuffer).Label("缓存时间（秒，0–60）").Placeholder("例如 0.5；0 关闭额外缓存").Disabled(settingsDisabled).FillWidth()
							}
							ui.Text(c, "缓存缓解抖动并增加延迟，默认 2 秒。自定义支持三位小数；下次启动时生效。").FontSize(12).TextColor(t.TextMuted)
							ui.Checkbox(c, &a.playback.TurnScreenOff, "启动后熄灭设备屏幕（关闭屏幕电源）").Disabled(settingsDisabled)
							ui.Text(c, "勾选熄屏后，结束时自动请求重新亮屏。").FontSize(12).TextColor(t.TextMuted)
							ui.Checkbox(c, &a.playback.MuteOnStop, "结束后静音设备媒体声音").Disabled(settingsDisabled)
							ui.Text(c, "默认开启，结束后媒体音量设为 0；可用手机音量键调回。").FontSize(12).TextColor(t.TextMuted)
							if !a.playback.AudioOnly {
								ui.Text(c, "熄屏后继续投屏和操作；Alt+O 熄屏，Alt+Shift+O 亮屏。手机实体电源键会重新亮屏。").FontSize(12).TextColor(t.TextMuted)
								ui.Checkbox(c, &a.playback.KeyboardUHID, "电脑键盘输入（UHID，支持手机输入法）").Disabled(settingsDisabled)
								ui.Text(c, "点击投屏中的输入框后打字；中文由手机输入法处理。首次按 Alt+K 配置实体键盘。不兼容时取消勾选，恢复基础键盘输入。").FontSize(12).TextColor(t.TextMuted)
								ui.Checkbox(c, &a.playback.Fullscreen, "投屏启动时全屏（覆盖任务栏）").Disabled(settingsDisabled)
								ui.Select(c, &a.playback.ScaleMode, scaleModes).Label("画面缩放模式").Disabled(settingsDisabled).FillWidth()
								ui.Text(c, "F11 全屏后，Alt+Z → 选择显示区域，鼠标拖选后自动放大。Esc 取消选区或退出全屏。").FontSize(12).TextColor(t.TextMuted)
							}
						})
					})
				})
			})
		})
		ui.Column(c).Padding(14).Gap(6).Background(t.Background).Radius(10).Border(1, t.Border).Children(func() {
			ui.Row(c).Gap(12).Children(func() {
				ui.Text(c, a.status).Bold().Grow(1)
				if ui.Button(c, "打开日志窗口").Clicked() {
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
					if ui.PrimaryButton(c, label).Disabled(disabled || a.selected == "" || pairing).Clicked() {
						a.start()
					}
				} else {
					label := "停止投屏"
					if session.playback.AudioOnly {
						label = "停止音频"
					}
					if ui.Button(c, label).Disabled(session.stopping).Clicked() {
						a.stop(session)
					}
				}
				if len(a.sessions) > 1 && ui.Button(c, "停止全部").Clicked() {
					for _, session := range a.sessions {
						a.stop(session)
					}
				}
				if a.busy && ui.Button(c, "取消操作").Clicked() {
					a.cancelJob()
				}
			})
			if a.detail != "" {
				ui.Text(c, a.detail).FontSize(12).TextColor(t.TextMuted)
			}
			if a.errorText != "" {
				ui.TextArea(c, &a.errorText).ReadOnly(true).Height(100).TextColor(t.Danger).Label("错误详情")
			}
		})
		ui.Text(c, fmt.Sprintf("scrcpy + mygo  ·  H.264  ·  Windows 便携版  ·  %s", version)).FontSize(11).TextColor(t.TextMuted)
	})
	ui.Modal(c, &a.showPair, func() {
		ui.Column(c).Width(420).Gap(14).Children(func() {
			ui.Text(c, "配对 Android 设备").FontSize(22).Bold()
			ui.Text(c, "在手机的无线调试中打开「使用配对码配对设备」，填写该页面的 IP:端口及 6 位配对码。").TextColor(t.TextMuted)
			ui.TextInput(c, &a.pairAddress).Label("配对地址").Placeholder("例如 192.168.1.10:40123").FillWidth()
			ui.TextInput(c, &a.pairCode).Label("配对码").Placeholder("6 位配对码").Password().FillWidth()
			ui.Text(c, "配对端口与投屏连接端口不同。").FontSize(12).TextColor(t.TextMuted)
			ui.Row(c).Gap(10).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Clicked() {
					a.showPair, a.pairCode = false, ""
				}
				if ui.PrimaryButton(c, "配对设备").Disabled(disabled || a.pairAddress == "" || a.pairCode == "").Clicked() {
					a.pair()
				}
			})
		})
	})
}
