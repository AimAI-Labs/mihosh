# AGENTS.md

> Mihosh (基于 Go 1.25 的 Mihomo TUI/CLI 客户端)
> 第一原则 (Fact-Based)：行动前必须阅读当前实际代码与测试，绝不依赖过期文档或臆测。

## 0. 项目规约
- 规则一：先想清楚再动手。不许偷偷做假设，歧义必问，提倡最简方案。
- 规则二：简洁至上。不写投机性功能，不做过度抽象。
- 规则三：精准手术式修改。只动必须动的地方，遵循现有风格。

## 1. 技术栈与常用命令
- **栈**: Go 1.25.0, Bubble Tea v1.3, Bubbles, Lipgloss, cobra, viper, gorilla/websocket, ntcharts, bubblezone, selfupdate.
- **环境**：执行命令前必须判断系统类型（Windows或Linux），使用兼容的命令执行。（win必须使用pwsh.exe前缀执行命令）
- **构建**：项目使用了mise工具。
- **构建/测试资源控制（尤其 Linux）**：执行前必须检查可用内存、CPU 负载及已有构建/测试进程；高负载检查默认串行，避免构建、测试和类型检查同时运行。根据实际资源限制并发、worker 数量及进程内存，并确认限制对子进程生效；内存不足或 CPU 持续过载时先停止并调整。完成或中断后清理本次启动的进程，并恢复验证期间临时修改的文件。
- **命令**: 
  > win下禁止使用类似`2>nul`命令，以防生成nul等保留设备名的文件
  - `mise exec go -- go test ./...` (涉及渲染及逻辑时必须跑测试，不依赖真实服务端环境)
  - `mise exec go -- go build -o mihosh .`（强制，构建后会自动杀死进程并重新运行）
- **构建后**：
  - win：执行`.\deploy.ps1`
  - linux：打包对应的可执行的mihosh文件，使用rsync上传到测试机的家目录下

## 2. 架构约定
- **CLI (`internal/cli`)**: 新增命令须支持 `--output plain|table|json`，以便脚本化。
- **TUI (`internal/ui/tui`)**: Feature-Sliced 结构；更新拆分为 `update.go`(路由分发)、`update_keys.go`(键盘)、`update_mouse.go`(鼠标)、`update_ws.go`(WebSocket 状态)。
- **主题 (`internal/ui/theme`)**: 多主题引擎（tokyo-night / catppuccin / gruvbox / nord / dracula）；颜色统一通过 `theme.Current()` 获取，禁止硬编码色值。
- **服务 (`internal/app/service`)**: 业务逻辑与 API 调用封装（ProxyService / ConnectionService / ConfigService / ProfileService / IPResolver / Updater）。
- **订阅 (`internal/infrastructure/profile`)**: 订阅的拉取、转换、合并、备份逻辑；`profile.Profile` 为订阅元数据结构。
- **配置 (`internal/infrastructure/config/types.go`)**: `language` 切换需刷新 i18n/快捷键；连接信息由 mihomo 配置文件解析；Mihomo 配置修改后会热重载。
- **国际化 (`pkg/i18n/locales/*.json`)**: 修改 UI 文案必须同时更新 `zh-CN` 与 `en-US`。

## 3. TUI 状态与通信规范 (严格遵守)
- **主 Model (`tui/model.go`)**: **仅限**存放基础设施、全局路由(PageType/PageCount)、共享数据(`ChartData`)、全局错误与 WS 生命周期。
- **页面 State (`features/<page>/state.go`)**: **绝对禁止**将页面局部状态放入主 Model。各页面状态严格封装于自身目录。复杂页面已拆分为 `state_mouse.go`、`state_filter.go`、`state_form.go` 等子文件。
- **消息定义 (`tui/messages/events.go`)**: 所有 TUI 消息必须集中在此文件定义，载荷极简。跨页通信必须依赖 `tea.Msg`，禁止直接篡改内部字段。
- **渲染交互**:
  - 布局尺寸：宽窄屏通过 `width` 参数动态计算面板比例（≥100 列宽屏双面板 43%/57%，＜100 列窄屏垂直堆叠，卡片除外）。
  - 鼠标坐标：新增鼠标交互优先复用 `resolveMainPageMouseHit`（位于 `update_mouse.go`）。
  - 弹窗拦截：弹窗（如详情）激活时应拦截底层按键，退出时需清理状态。

## 4. 并发、容量与生命周期
- **并发控制**: 批量网络操作**必须**限制并发。复用 `model.TestConcurrency = 20` 并结合 `WaitGroup`/`Mutex`/Semaphore 汇总结果。禁止创建无上限的 goroutine。
- **定长缓存 (Ring Buffer)**: 历史数据（连接、日志、图表点）**必须**使用 Ring Buffer，如 `LogsCap=1000`。禁止使用无界切片导致内存泄漏。
- **WebSocket (`infrastructure/api/websocket.go`)**: 必须复用现有的 `WSClient.Start` 和 `connectStream[T]` 泛型封装。禁止自行实现断线重连或重新挂载流。


## 5. 固定工作流
- **自动提交**：每完成一个任务，都在本地进行原子化提交
- **英文提交与 PR 文案**：git commit message、PR 标题与正文**一律使用英文**（代码注释与界面文案仍遵循既有中文规范）。
- **分支规范（`main` vs `dev`）**：`main` 为稳定发布分支（承载正式版标签 `vX.Y.Z`、`install.sh` 与 `Formula/mihosh.rb`，仅从已验收的 `dev` 同步）；`dev` 为开发分支（承载日常提交、功能/修复 PR、Dependabot 更新与预发布标签 `vX.Y.Z-*`）。

## 6. 第三方工具
- **噪声简化**：若当前开发环境为linux，使用rtk，@RTK.md
- **文档搜索**：使用`npx ctx7`，具体用法详见：https://context7.com/docs/clients/cli

<!-- CODEGRAPH_START -->
## CodeGraph

In repositories indexed by CodeGraph (a `.codegraph/` directory exists at the repo root), reach for it BEFORE grep/find or reading files when you need to understand or locate code:

- **MCP tool** (when available): `codegraph_explore` answers most code questions in one call — the relevant symbols' verbatim source plus the call paths between them, including dynamic-dispatch hops grep can't follow. Name a file or symbol in the query to read its current line-numbered source. If it's listed but deferred, load it by name via tool search.
- **Shell** (always works): `codegraph explore "<symbol names or question>"` prints the same output.

If there is no `.codegraph/` directory, skip CodeGraph entirely — indexing is the user's decision.
<!-- CODEGRAPH_END -->
