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
	if a.busy || a.session != nil {
		c.After(100 * time.Millisecond)
	}
	t := *c.Theme()
	t.Accent, t.Radius, t.FontSize = ui.Hex("#157a66"), 8, 14
	c.SetTheme(&t)
	disabled := a.busy || a.b == nil
	ui.Column(c).Fill().Padding(24).Gap(18).Background(ui.Hex("#f3f6f5")).Children(func() {
		ui.Row(c).Gap(16).Children(func() {
			ui.Column(c).Grow(1).Gap(4).Children(func() {
				ui.Text(c, "scrcpy LAN").FontSize(30).Bold().TextColor(ui.Hex("#132f2a"))
				ui.Text(c, "在同一个网络里，让手机屏幕来到桌面。").TextColor(t.TextMuted)
			})
			ui.Text(c, "MYGO NATIVE  /  "+version).FontSize(11).TextColor(t.TextMuted)
		})
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Row(c).Gap(18).AlignItems(ui.Stretch).Children(func() {
				ui.Column(c).Grow(1).Gap(16).Children(func() {
					ui.Column(c).Padding(20).Gap(14).Background(t.Background).Radius(12).Border(1, t.Border).Children(func() {
						ui.Row(c).Gap(10).Children(func() {
							ui.Text(c, "附近的设备").FontSize(19).Bold().Grow(1)
							if ui.Button(c, "重新扫描").Disabled(disabled).Clicked() {
								a.discover()
							}
						})
						ui.Text(c, "自动发现无线调试；同时扫描所选网段的 5555 端口。").TextColor(t.TextMuted)
						if len(a.networks) > 0 {
							ui.Select(c, &a.network, a.networks).Label("本地扫描网段").Disabled(disabled).FillWidth()
						} else {
							ui.Text(c, "未检测到本地 IPv4 网段，仍可通过 mDNS 或手动连接发现设备。").TextColor(t.TextMuted)
						}
						ui.Divider(c)
						if len(a.devices) == 0 {
							ui.Column(c).Padding(24, 8).Gap(10).Children(func() {
								ui.Text(c, "等待你的 Android 设备").FontSize(17).Bold()
								ui.Text(c, "1. 手机与电脑连接同一个 Wi-Fi\n2. 开发者选项 → 开启无线调试\n3. 首次使用，在右侧输入配对码").TextColor(t.TextMuted).LineHeight(1.8)
							})
						}
						for _, d := range a.devices {
							ui.Box(c).Key(d.key()).Children(func() {
								row := ui.ButtonBase(c).FillWidth().Padding(14).Radius(8).Border(1, t.Border)
								if a.selected == d.key() {
									row.Background(ui.Hex("#e7f4ef")).Border(1, t.Accent)
								}
								row.Children(func() {
									ui.Column(c).Grow(1).Gap(5).Children(func() {
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
						ui.Textf(c, "%d 个设备 / 服务", len(a.devices)).FontSize(12).TextColor(t.TextMuted)
					})
					ui.Column(c).Padding(20).Gap(12).Background(t.Background).Radius(12).Border(1, t.Border).Children(func() {
						ui.Text(c, "手动连接").FontSize(17).Bold()
						ui.Text(c, "填写无线调试主页上的 IP:端口；旧版 ADB 默认端口为 5555。").TextColor(t.TextMuted)
						ui.TextInput(c, &a.address).Label("连接地址").Placeholder("192.168.1.10:37123").Disabled(disabled).FillWidth()
						if ui.Button(c, "连接设备").Disabled(disabled || a.address == "").Clicked() {
							a.connect()
						}
					})
				})
				ui.Column(c).Width(370).Gap(16).Children(func() {
					ui.Column(c).Padding(20).Gap(12).Background(t.Background).Radius(12).Border(1, t.Border).Children(func() {
						ui.Text(c, "投屏档位").FontSize(19).Bold()
						for i, p := range presets {
							ui.Box(c).Key(p.Name).Children(func() {
								button := ui.ButtonBase(c).FillWidth().Padding(12).Radius(8).Border(1, t.Border).Disabled(a.session != nil || disabled)
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
						ui.Text(c, "分辨率保持屏幕比例，帧率为上限；实际表现取决于手机和网络。切换档位需停止后重启投屏。").FontSize(12).TextColor(t.TextMuted)
						if a.session == nil {
							pairing := false
							for _, d := range a.devices {
								if d.key() == a.selected {
									pairing = d.Pairing
									break
								}
							}
							if ui.PrimaryButton(c, "开始投屏").FillWidth().Padding(12).Disabled(disabled || a.selected == "" || pairing).Clicked() {
								a.start()
							}
						} else if ui.Button(c, "停止投屏").FillWidth().Disabled(a.stopping).Clicked() {
							a.stopping = true
							a.stopSession()
						}
					})
					ui.Column(c).Padding(20).Gap(10).Background(t.Background).Radius(12).Border(1, t.Border).Children(func() {
						ui.Text(c, "首次无线配对").FontSize(17).Bold()
						ui.Text(c, "Android 11+：打开无线调试 → 使用配对码配对设备。配对端口与连接端口不同。").FontSize(12).TextColor(t.TextMuted)
						if ui.Button(c, "打开配对").Disabled(disabled).Clicked() {
							a.showPair = true
						}
					})
				})
			})
		})
		ui.Column(c).Padding(14).Gap(6).Background(t.Background).Radius(10).Border(1, t.Border).Children(func() {
			ui.Row(c).Gap(12).Children(func() {
				ui.Text(c, a.status).Bold().Grow(1)
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
