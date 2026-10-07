scrcpy LAN 初版：使用 mygo 原生 Go UI 的 Windows 局域网投屏启动器。

- 自动发现 Android 无线调试服务，扫描本地 IPv4 网段的旧版 ADB 5555 端口。
- GUI 输入 Android 11+ 配对码，或用 IP:端口手动连接。
- 流畅 1024px / 30fps / 2Mbps、均衡 1600px / 60fps / 6Mbps、高清 1920px / 60fps / 12Mbps、原画 / 60fps / 20Mbps。
- 一键启动 / 停止 scrcpy 投屏，支持独立投屏窗口中的键鼠控制。
- Windows 10 / 11 x64 便携包，附带已校验的官方 scrcpy v5.0 和 adb；完整解压后运行 `scrcpy-lan.exe`。

手机需与电脑在同一局域网并开启无线调试。首次配对需要手机显示的配对码；配对端口与连接端口不同。Android 10 及以下需先通过 USB 开启 ADB TCP/IP。

本发布是社区 fork，Windows 可执行文件未签名。帧率是上限，实际表现取决于手机和网络。已验证自动化测试、便携包和本机 GUI 启动；当前环境没有可用 Android 真机，真实投屏需要用户设备验证。
