# scrcpy LAN · Linux

Linux 原生 GUI，沿用 Windows 版的设备发现、配对和会话管理。
GTK 3 / OpenGL 绘制操作台，官方 scrcpy v5.0.1 负责视频、音频、手机键鼠和剪贴板；不在 Go 中复制视频帧。

下载 [v0.7.3 Linux x64 便携包](https://github.com/KevinChen222/scrcpy/releases/download/gui-v0.7.3/scrcpy-lan-v0.7.3-linux-x64.tar.gz) 和 [SHA-256](https://github.com/KevinChen222/scrcpy/releases/download/gui-v0.7.3/scrcpy-lan-v0.7.3-linux-x64.sha256)。每个 [GUI Release](https://github.com/KevinChen222/scrcpy/releases) 同时提供 Windows / Linux 包，使用同一 GUI 版本。

## 本机启动

在项目根目录运行：

```bash
./gui/launch-linux.sh
```

也可解压发布包，在解压后的目录运行 `./start.sh`。保留 `runtime` 目录。
官方 Linux x86_64 scrcpy 和 adb 随包提供，无需安装 Windows 运行环境。

桌面「scrcpy LAN」快捷方式使用 Windows 版相同的 `gui/assets/icon.png` 图标，
固定指向项目的 `gui/launch-linux.sh`。每次 `build-linux.sh` 成功打包后会更新
`gui/build/current-linux`，桌面入口自动使用这次构建的版本，不需要改快捷方式路径。

桌面依赖：GTK 3、OpenGL、Cairo、X11、GNU coreutils 的 `stdbuf`（Ubuntu 24.04 桌面通常已提供）。
本版本的投屏窗口需要 X11；Wayland 桌面需要 XWayland 和有效的 `DISPLAY`，启动器会将 GTK 和 SDL 切换到 X11。
没有实现纯 Wayland 的跨进程窗口嵌入。

## 功能

- USB / 局域网设备发现、mDNS、手动连接、Android 无线调试二维码 / 配对码配对。
- 手机名称与身份识别、连接别名合并、多设备独立运行及停止。
- 视频档位、自定义分辨率 / 帧率 / 码率、音视频缓存。
- 仅音频、Opus / AAC / FLAC / RAW、预设及自定义音频码率。
- UHID 键盘、启动后熄屏、结束后静音及亮屏恢复。
- 独立实时日志窗口，日志仅在开启时收集、保留最近 64 KiB。
- 投屏保持比例 / 裁剪铺满 / 拉伸铺满，运行中切换，全屏手动选区。

投屏窗口菜单「画面缩放」或 **Alt+Z** 切换模式。
**F11** 切换全屏，**Esc** 优先取消选区或关闭菜单，然后退出全屏。
普通投屏窗口中的 Esc 仍交给 scrcpy，用于手机返回。**Alt+Q** 结束当前投屏。
全屏时菜单栏隐藏，Alt+Z 仍可使用。

v0.7.3 修复高 DPI 桌面下画面挤在左上角、右侧和下方留下大片黑区的问题：GTK 逻辑尺寸按当前显示缩放倍率转换成 X11 像素尺寸，手动选区仍使用 GTK 逻辑坐标。保持比例、裁剪铺满、拉伸及全屏 / 还原均按实际视口显示。

保留 v0.7.2 的启动超时修复：通过 `stdbuf -oL` 实时读取官方 scrcpy 的画面尺寸，避免输出管道缓冲导致约 15 秒后会话退出。官方运行包保持原样；关闭日志窗口仍停止收集日志。

全屏菜单选择「选择显示区域」，鼠标左键拖选、松开确认，Esc 取消。
选区不会向手机发送拖动；选区内的后续点击仍由 scrcpy 映射。
窗口缩放、全屏切换保留选区，解码尺寸或横竖屏变化重置选区。

关闭 GUI 或收到 SIGINT / SIGTERM 后等待会话停止与手机清理。
Linux 子进程使用独立进程组，取消时回收该进程组；直接子进程在 GUI 被强制结束时由内核终止。
Linux 退出不会关闭共享的 ADB 服务，避免影响 Android Studio 等工具。
强制结束不能保证手机静音、亮屏恢复或所有脱离进程组的后代清理。

120 fps 是请求上限，实际性能取决于手机编码器、桌面 GPU 与连接。
若需要最低交互延迟，选择「关闭缓存」；默认 2 秒缓存用于播放稳定性，会增加可见延迟。

详细设备操作和参数说明见 `Windows-and-common.md`（发布包）或项目 `gui/README.md`。

二维码配对：点击「打开配对」→「二维码配对」，在手机无线调试中选择「使用二维码配对设备」扫码，程序自动配对。手机与电脑需在同一 Wi-Fi，mDNS 不可被网络隔离或防火墙阻止。等待 2 分钟后可重新生成；Esc、关闭弹窗或切换配对码会取消等待。配对成功后选择连接设备开始投屏。

## 构建及验证

需要 Go 1.27.2+、curl、tar、sha256sum，使用固定版本 mygo v0.4.5 和官方 scrcpy v5.0.1。
运行包来源及校验值见 `build-linux.sh`。

```bash
./gui/build-linux.sh
```

脚本只构建 GUI，先运行 GUI 测试和 vet，再生成 `gui/build/` 下的可执行目录、tar.gz 和 SHA-256 文件。
Go 自动下载所需工具链；缓存默认放在项目 `.tools/` 下。

默认版本从 `gui/VERSION` 读取；可运行 `./gui/build-linux.sh vX.Y.Z` 显式指定。GitHub Actions 同一提交同时构建 Windows / Linux，发布使用双平台 CI 产物及校验文件。

普通测试不操作手机。X11 窗口集成验证需要解锁的桌面、gcc 和 GTK 3 开发包：

```bash
cd gui
SCRCPY_GUI_LINUX_NATIVE_TEST=1 go test -count=1 -run TestLinuxMirrorNative -v .
```

窗口集成测试使用带标准输出缓冲的模拟视频窗口，按 X11 原生视口像素独立检查实时尺寸、三种缩放、全屏持续 12 秒、菜单、快捷键、选区及旋转重置，不修改手机。可用 `GDK_SCALE=1` / `GDK_SCALE=2` 检查 100% / 200% 缩放；设置 `SCRCPY_GUI_LINUX_MIRROR_CAPTURE_DIR` 保存各模式、全屏和还原后的实际窗口截图，检查视频实际显示范围。

真机音视频全屏回归需要已授权手机和解锁的 X11 桌面。将便携包的 `runtime` 加到 PATH 后运行：

```bash
SCRCPY_GUI_LINUX_MIRROR_SERIAL='设备连接标识' go test -buildvcs=false -count=1 -run '^TestLinuxMirrorNative$' -timeout=75s -v .
```

真机模式检查音视频解码、12 秒全屏存活、F11 / Esc / Alt+Z / Alt+Q，只结束自己启动的会话；鼠标选区与模拟旋转在模拟窗口中验证。声音听感、UHID 和电源行为需另行验证。

二维码原生窗口验证：`SCRCPY_GUI_QR_NATIVE_TEST=1 go test -buildvcs=false -count=1 -run TestLinuxQRNative -v .`。设置 `SCRCPY_GUI_QR_NATIVE_SCREENSHOT` 可保存截图；测试使用模拟后台，只验证真实 GTK 窗口绘制及关闭，不操作手机。与投屏原生检查分开运行。
