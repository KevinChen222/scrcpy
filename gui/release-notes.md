scrcpy LAN v0.7.4：Windows / Linux 同步升级 mygo 原生 UI。

主要更新：

- 两平台 mygo 从 v0.4.5 升级到 v0.4.6，引入按需裁剪遮罩、遮罩缓存复用、文字布局缓存优化，以及焦点重绘和 keyed 元素状态修复。实际性能收益取决于界面和桌面环境，未承诺具体提升幅度。
- 保持 Go 1.27.2、CGO_ENABLED=0、中文原生 GUI 和便携包；不引入浏览器、自动更新、游戏插件或 mygo 音频播放。
- 官方 scrcpy v5.0.1 运行包及校验值不变，音视频仍由官方 scrcpy 处理。保留多设备、二维码配对、实时日志、三种缩放、全屏选区和 Linux 高 DPI / 启动超时修复。

下载与使用：

- Windows：`scrcpy-lan-v0.7.4-windows-x64.zip` 与 `.sha256`，完整解压后运行 `scrcpy-lan.exe`。
- Linux：`scrcpy-lan-v0.7.4-linux-x64.tar.gz` 与 `.sha256`，解压后运行 `./start.sh`。需要 GTK 3、OpenGL、Cairo、X11 / XWayland 和 GNU coreutils 的 `stdbuf`。

两个包均附带官方 scrcpy、adb、说明和许可证，保留整个 `runtime` 目录。发布以同一提交的 Windows / Linux CI 成功为前提，资产使用该次 CI 产物及 SHA-256，不上传本地构建包。

验证：Linux 本地测试、vet、便携构建与 SHA-256 检查通过；本地构建的启动器在真实 X11 桌面启动，SIGTERM 正常退出。100% / 200% 缩放的模拟视频原生窗口回归通过，覆盖嵌入、三种缩放、12 秒持续全屏、菜单、快捷键、选区、旋转重置及清理。另在真实 GTK 窗口检查中文默认 / 最小布局、二维码绘制和日志窗口的打开、关闭清空及重新打开；这些 UI 检查使用模拟后台，不操作手机。

验证边界：本次没有连接的授权手机，未重新验证真实音视频、扫码配对、UHID、熄屏 / 亮屏和结束后静音；不把模拟窗口或截图当作真机成功。Windows 仅由 CI 测试、vet、构建和核对运行包，本机不能验证 Windows 桌面或手机。长时间播放和日志滚动 / 复制仍需实际使用检查。Windows 包未进行代码签名。

二维码要求 Android 11+、同一 Wi-Fi 及可用 mDNS；网络受限时可使用配对码与手动连接。Windows 退出关闭所用 ADB 服务，Linux 保留共享服务；强制结束不能保证完成手机静音或亮屏清理。
