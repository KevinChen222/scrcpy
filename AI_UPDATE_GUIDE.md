# scrcpy LAN 更新指南

这是项目背景与默认约定；本次明确要求优先。请完成实现、验证和交付，开始时核对实际 Git / Release，不把下列版本当作实时状态。

## 项目

- Windows 10 / 11 x64 中文原生 Go GUI，负责 Android 设备发现、配对、连接和参数选择；投屏由官方 scrcpy 处理。
- fork：`KevinChen222/scrcpy`，`origin` 指向它，发布目标 `master`；上游 `Genymobile/scrcpy` 使用 `upstream`，不要推送到上游。
- 本机目录 `D:\codex\scrpy`。现有工作区先检查并保留未提交修改，不重复 clone / fork。
- 当前交付版本：GUI `v0.6.1` / tag `gui-v0.6.1`；[Release](https://github.com/KevinChen222/scrcpy/releases/tag/gui-v0.6.1)。官方运行包 `v5.0`，依赖以 `gui/go.mod` 为准。

## 约束与入口

- 保持 mygo 原生 UI、`CGO_ENABLED=0` 和便携包；不要增加浏览器、登录、云服务或自动更新。
- GUI 改动集中在 `gui/` 和 GUI 工作流；不修改上游 `app/` / `server/`，正常更新合并 `upstream/master`。运行包升级核对官方版本、SHA-256、参数兼容性和许可；破坏性更新才适配 GUI。
- `main.go`：主线程状态、后台任务、会话生命周期；`view.go`：中文界面；`backend.go`：ADB / 发现 / 进程；`presets.go`：参数与校验；`logs.go`：按需日志窗口。
- `mirror_scale.go`：比例、手动选区和坐标；`mirror_window_windows.go` / `mirror_shortcuts_windows.go` / `mirror_selection_windows.go`：各设备独立嵌入官方 scrcpy 的原生投屏窗口、缩放菜单、全屏和拖选。只改 GUI，官方 runtime 保持原样。
- `gui/README.md`：完整功能、使用与测试说明；`gui/release-notes.md`：最终版本主要更新；`gui/build.ps1` 和 `.github/workflows/gui.yml`：便携构建与 CI。

## 必须保留的行为

- 无线 mDNS 区分配对端口和连接端口；保留手动连接。旧版只扫描所选本地 IPv4 网段 TCP 5555，最多 254 地址 / 32 并发 / 250ms；USB 不扫描网络。
- 启动前验证真实 ADB `device` 状态，检查输出与退出码；保留空 Serial 的多设备去重回归。配对码不记录。
- 多设备独立会话，可混用仅音频 / 音视频以及 USB / 无线；同一 ADB 连接不重复启动。单独停止不影响其他设备；停止、异常和 GUI 退出回收自己启动的进程，保留全局 ADB 服务。后台操作可取消，UI 状态仅主线程更新。
- 熄屏会话结束自动恢复亮屏；默认开启结束后媒体静音，通过 ADB 将该设备媒体音量设为 0 并验证，允许启动前取消。UHID 中文由手机输入法处理。纯音频支持 Opus / AAC / FLAC / RAW，后两者不传码率。
- 自定义单位：分辨率 px、帧率 fps、视频 Mbps、音频 Kbps（6–9000 请求值）、缓存秒（0–60，三位小数）。缓存预设 0.5 / 1 / 2 秒，默认 2 秒，0 关闭额外缓存；仅音频不传视频设置。
- 日志只在独立窗口打开时收集，关闭清空并停止，不写文件；有界内存、线程安全。沿用紧凑布局、独立滚动和固定开始 / 停止栏。
- 投屏中 Alt+Z 切换保持比例 / 裁剪铺满 / 拉伸；F11 全屏后可拖选显示区域或重置，已移除自动黑边识别与截帧。选区按解码坐标保存到会话结束，尺寸变化重置；子窗口裁剪保留 scrcpy 输入坐标映射。各窗口独立管理快捷键和选区，Esc 优先关闭菜单、取消选区，再退出全屏。

## 更新与发布

1. 检查 Git 状态、远端、目标分支和现有 Release；只改需求相关内容，简短中文说明改动与验证。
2. Go 命令在 `gui/` 运行。格式化改动后执行 `./build.ps1 -GuiVersion vX.Y.Z`（含测试 / vet / 编译 / 官方校验 / 打包），不无理由重复检查。
3. UI 检查中文、默认 / 最小布局与交互、真实 Windows 启动；生命周期改动尽量用授权真机验证。模拟或截图不等于真机成功，明确实际验证边界。
4. 同步构建默认版本、工作流参数 / 资产路径、README 下载与发布说明；精简更新本文。版本 `vX.Y.Z`，tag `gui-vX.Y.Z`，资产 `scrcpy-lan-vX.Y.Z-windows-x64.zip` 与 `.sha256`，完整包含 runtime、说明和许可。
5. 默认先交付本地包，等用户实测确认；**本次明确要求推送 / 发布即已有授权**。提交推送到 fork 的目标分支，等待对应 GUI Windows CI 成功，再创建新 tag / Release（`--verify-tag --notes-file gui/release-notes.md`）。不强推、不覆盖 tag。
6. 核对 Release 已发布、两项资产 uploaded、服务端 ZIP SHA-256 一致；中文报告下载链接、验证与真实剩余限制。

文件检查优先 FastCtx；Windows 用 PowerShell 7、结构化参数和退出码检查。登录失效让用户本机运行 `gh auth login`，不索取 token。沙箱按平台处理；Git 信任仅限已核实的具体仓库。
