scrcpy LAN v0.7.3：Windows / Linux 双平台便携版，修复 Linux 全屏与高 DPI 显示。

主要更新：

- 仓库同时维护 Windows 10 / 11 x64 和 Linux x64，使用同一 GUI 版本及官方 scrcpy v5.0.1。GitHub Actions 在同一提交上分别测试、vet、构建、核对官方运行包 SHA-256；每个 Release 提供两个平台包及各自校验文件。
- 新增 Linux 原生操作台与 GTK / X11 投屏窗口，支持独立多设备、三种缩放、F11 / Esc / Alt+Z / Alt+Q 及全屏手动选区。支持 X11 或 XWayland；Linux 退出保留共享 ADB 服务。
- 修复 Linux 音视频启动约 15 秒后退出的问题：使用 GNU coreutils 的 `stdbuf -oL` 实时读取画面尺寸，避免输出管道缓冲导致窗口接管超时。
- 修复 200% 桌面缩放时画面挤在左上角的问题：GTK 逻辑尺寸转换为 X11 像素，选区保留逻辑坐标。原生回归独立读取实际视口像素，覆盖 100% / 200% 缩放、全屏及还原。
- 新增 Android 无线调试二维码配对，保留 6 位配对码模式。每次随机生成临时凭据，2 分钟过期；关闭、Esc、切换模式或退出时取消等待，密码不写入日志。
- 使用 Go 1.27.2、mygo v0.4.5 和纯 Go 二维码库，保持 CGO_ENABLED=0。`gui/VERSION` 统一两平台默认版本，运行包及许可证完整保留。

下载与使用：

- Windows：`scrcpy-lan-v0.7.3-windows-x64.zip` 与 `.sha256`，完整解压后运行 `scrcpy-lan.exe`。
- Linux：`scrcpy-lan-v0.7.3-linux-x64.tar.gz` 与 `.sha256`，解压后运行 `./start.sh`。需要 GTK 3、OpenGL、Cairo、X11 / XWayland 和 GNU coreutils 的 `stdbuf`，Ubuntu 24.04 桌面通常已提供。

两个包均附带官方 scrcpy v5.0.1、adb、说明和许可证，保留 `runtime` 目录。发布资产使用对应提交的 Windows / Linux CI 产物，待两平台 CI 成功后上传。

验证：Linux GUI 单元测试、vet、本地便携构建与 SHA-256 检查通过；100% / 200% 原生窗口回归覆盖三种缩放、12 秒持续全屏、菜单、快捷键、选区及旋转重置。已授权 realme RMX5062（Android 16）使用默认 2 秒音视频缓存，通过真实音视频解码、普通 / 全屏三种缩放、F11 / Esc / Alt+Z / Alt+Q 和还原显示；已检查实际截图，Linux 窗口修复由用户实测确认。发布以 Windows / Linux 对应提交的 CI 测试、vet、构建、运行包校验及 Linux 原生回归通过为条件。

验证边界：Windows 本版由 Windows CI 构建，尚未进行真实 Windows 桌面及手机验证。Linux 已检查短时音视频解码与窗口显示，长时间播放、声音听感、真实鼠标选区、UHID、电源清理及真实二维码扫码仍需设备验证。Windows 包未进行代码签名。

二维码要求 Android 11+、同一 Wi-Fi 及可用 mDNS；受网络隔离、VPN 或防火墙影响时可使用配对码与手动连接。Windows 退出会关闭所用 ADB 服务，Linux 保留共享服务；强制结束不能保证完成手机静音或亮屏操作。
