# AGENTS.md

> Mihosh (基于 Go 1.24 的 Mihomo TUI/CLI 客户端)
> 第一原则 (Fact-Based)：行动前必须阅读当前实际代码与测试，绝不依赖过期文档或臆测。

## 0. 项目规约
- 规则一：先想清楚再动手。不许偷偷做假设，歧义必问，提倡最简方案。
- 规则二：简洁至上。不写投机性功能，不做过度抽象。
- 规则三：精准手术式修改。只动必须动的地方，遵循现有风格。

## 1. 技术栈与常用命令
- **栈**: Go 1.24.0, Bubble Tea v1.3, cobra, viper, gorilla/websocket.
- **环境**：执行命令前必须判断系统类型（Windows或Linux），使用兼容的命令执行。（win优先执行pwsh.exe）
- **构建**：项目使用了mise工具。
- **命令**: 
  > win下禁止使用类似`2>nul`命令，以防生成nul等保留设备名的文件
  - `mise exec go -- go test ./...` (涉及渲染及逻辑时必须跑测试，不依赖真实服务端环境)
  - `mise exec go -- go build -o mihosh .`（强制，构建后会自动杀死进程并重新运行）
- **构建后（本地为Win环境时执行）**：执行`.\deploy.ps1`

## 2. 架构约定
- **CLI (`internal/cli`)**: 新增命令须支持 `--output plain|table|json`，以便脚本化。
- **TUI (`internal/ui/tui`)**: Feature-Sliced 结构。
- **服务 (`internal/app/service`)**: 业务逻辑与 API 调用封装。
- **配置 (`internal/infrastructure/config/types.go`)**: `language` 切换需刷新 i18n/快捷键；`proxy_address` 供测速用，需同步至 Connections；Mihomo 配置修改后会热重载。
- **国际化 (`pkg/i18n/locales/*.json`)**: 修改 UI 文案必须同时更新 `zh-CN` 与 `en-US`。

## 3. TUI 状态与通信规范 (严格遵守)
- **主 Model (`tui/model.go`)**: **仅限**存放基础设施、全局路由(PageType/PageCount)、共享数据(`ChartData`)、全局错误与 WS 生命周期。
- **页面 State (`features/<page>/state.go`)**: **绝对禁止**将页面局部状态放入主 Model。各页面状态严格封装于自身目录。
- **消息定义 (`tui/messages/events.go`)**: 所有 TUI 消息必须集中在此文件定义，载荷极简。跨页通信必须依赖 `tea.Msg`，禁止直接篡改内部字段。
- **渲染交互**:
  - 布局尺寸：必须使用 `layout.SidebarWidth()` 计算动态侧边栏。
  - 鼠标坐标：新增鼠标交互优先复用 `resolveMainPageMouseHit`。
  - 弹窗拦截：弹窗（如详情）激活时应拦截底层按键，退出时需清理状态。

## 4. 并发、容量与生命周期
- **并发控制**: 批量网络操作**必须**限制并发。复用 `model.TestConcurrency = 20` 并结合 `WaitGroup`/`Mutex`/Semaphore 汇总结果。禁止创建无上限的 goroutine。
- **定长缓存 (Ring Buffer)**: 历史数据（连接、日志、图表点）**必须**使用 Ring Buffer，如 `LogsCap=1000`。禁止使用无界切片导致内存泄漏。
- **WebSocket (`infrastructure/api/websocket.go`)**: 必须复用现有的 `WSClient.Start` 和 `connectStream[T]` 泛型封装。禁止自行实现断线重连或重新挂载流。

@RTK.md
