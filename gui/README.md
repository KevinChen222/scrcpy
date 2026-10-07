# scrcpy LAN

基于 [scrcpy](https://github.com/Genymobile/scrcpy) 的社区 fork，新增使用 [mygo](https://github.com/egoist/mygo) 原生 Go UI 的局域网投屏启动器。初版面向 Windows 10 / 11 x64，界面使用 Direct3D 11 绘制，不需要浏览器、WebView2、Node.js 或额外安装 Go。

## 下载和启动

从 [本 fork 的 Releases](https://github.com/KevinChen222/scrcpy/releases) 下载 `scrcpy-lan-v0.1.0-windows-x64.zip`，**完整解压**，双击 `scrcpy-lan.exe`。`runtime` 目录附带官方 scrcpy v5.0、adb 和运行依赖，勿单独移动 exe。发布包未进行代码签名。

GUI 用于设备发现、配对和档位选择；视频、声音和键鼠控制由原版 scrcpy 在独立投屏窗口中处理。关闭 GUI 会停止它启动的投屏，不会关闭全局 ADB 服务。

## Android 11 及以上

1. 手机和电脑连接同一个 Wi-Fi，手机开启「开发者选项 → 无线调试」。
2. 首次使用，手机进入「使用配对码配对设备」。GUI 点击「重新扫描」，选择发现的配对服务，或点击「打开配对」，在弹窗里填写配对页面的 IP:端口和 6 位配对码，点击「配对设备」。
3. 配对成功后，选择连接服务和投屏档位，点击「开始投屏」。扫描列表也会显示已连接的 USB / ADB 设备。
4. 如果路由器阻止 mDNS，使用「手动连接」，填写**无线调试主页**上的 IP:端口。这个连接端口与配对码页面的配对端口不同，切换无线调试后也可能变化。

手机必须开启无线调试并允许连接；程序无法发现或投屏未开启 ADB 的普通局域网手机。Windows 提示网络访问时需允许 adb 在私人网络中通信。访客 Wi-Fi、AP 隔离、VPN 或防火墙可能阻止发现 / 连接。[Android 官方无线调试说明](https://developer.android.com/tools/adb#wireless-android11-command-line)

## Android 10 及以下 / 旧版 ADB

首次需要 USB 连接并在手机上授权 USB 调试。在此目录的终端执行：

```powershell
.\runtime\adb.exe devices
.\runtime\adb.exe -s 手机序列号 tcpip 5555
```

确认手机和电脑在同一 Wi-Fi，拔掉 USB，GUI 扫描本地网段或手动输入手机 IP（默认 5555）。扫描采用最多 32 个并发连接、每个 250ms 超时，仅探测所选 IPv4 网段的 TCP 5555 端口。大于 /24 的本地网络只扫描电脑所在 /24，其他网段可手动连接。开放端口仅标为「待验证」，启动投屏时会检查真实 ADB 连接和授权状态。

## 投屏档位

| 档位 | 分辨率长边上限 | 帧率上限 | 视频码率 |
| --- | ---: | ---: | ---: |
| 流畅 | 1024 px | 30 fps | 2 Mbps |
| 均衡（默认） | 1600 px | 60 fps | 6 Mbps |
| 高清 | 1920 px | 60 fps | 12 Mbps |
| 原画 | 设备原生分辨率 | 60 fps | 20 Mbps |

保持手机屏幕比例，统一采用兼容性较好的 H.264。帧率为上限，实际帧率由内容刷新、设备编码能力和网络决定。档位在启动投屏时应用，切换需先停止再启动。初版同时运行一个投屏会话；音频沿用 scrcpy 默认行为，Android 11+ 支持音频转发。

## 从源码构建

需要 Go 1.27.1+ 和 PowerShell 7。mygo 固定在 `869768d02d9154d5b2f9f3a6ebb39767f8f96983`，投屏运行包固定为官方 scrcpy v5.0 并检查 SHA-256。

```powershell
cd gui
go test ./...
go vet ./...
.\build.ps1 -GuiVersion v0.1.0
```

输出为 `gui/build/scrcpy-lan-v0.1.0-windows-x64.zip` 和对应 `.sha256`。构建脚本会验证测试、使用 `CGO_ENABLED=0` 编译并移除原生 UI 调试检查器，然后附带运行文件和许可证。开发时可将 `runtime` 放到开发版 exe 旁，或把 adb / scrcpy 加入 PATH。

测试覆盖 mDNS / ADB 解析、服务去重、地址及配对码校验、拒绝未授权连接、子进程参数和取消，以及原生界面的设备 / 档位选择。真实手机投屏仍需在自己的局域网和手机上验证。

## 许可证

本 fork 和新增 GUI 沿用根目录 Apache-2.0。mygo 为 MIT；便携包中的 `licenses` 与 `THIRD_PARTY_NOTICES.md` 提供 Go 依赖许可信息，官方 scrcpy 运行文件及许可证保留在 `runtime`。
