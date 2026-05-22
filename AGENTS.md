# AGENTS.md

本文档为参与 Mihosh 项目开发的 AI 智能体及代码助手提供当前项目上下文、开发规范和决策指南。内容以当前仓库代码为准；过期或不存在的外部文档引用不要作为依据。

---

## 1. 项目定位

- **项目名称**：Mihosh
- **仓库模块**：`github.com/AimAI-Labs/mihosh`
- **定义**：面向 Mihomo / Clash Meta 的终端管理客户端，包含交互式 TUI 与可脚本化 CLI。
- **核心能力**：
  - 节点与策略组管理：查看、切换、单节点测速、策略组测速、批量测速。
  - 实时连接监控：活跃连接、历史连接、流量与内存图表、连接关闭、Top N 统计、站点连通性测试。
  - 实时日志查看：WebSocket 日志流、级别过滤、关键词过滤、详情解析、内网源 IP 解析。
  - 规则查看：规则列表与搜索。
  - 设置编辑：TUI 内编辑配置，CLI 中初始化、展示、修改、编辑配置。
  - 国际化：内置 `zh-CN` 与 `en-US` 资源，支持配置语言。

## 2. 技术栈

| 层级 | 技术选型 | 当前版本 / 位置 |
|---|---|---|
| 语言 | Go | `go 1.24.0`，toolchain `go1.24.11` |
| CLI | cobra + viper | cobra `v1.10.1`，viper `v1.21.0` |
| TUI | Bubble Tea | `v1.3.10` |
| UI 组件 | bubbles + lipgloss | bubbles `v0.21.0`，lipgloss `v1.1.0` |
| WebSocket | gorilla/websocket | `v1.5.3` |
| 测试 | testify + Go test | `v1.11.1` |
| 国际化 | 本地 JSON 资源 | `pkg/i18n/locales/*.json` |

---

## 3. 当前目录结构

```text
mihosh
├── main.go
├── go.mod
├── Makefile
├── README.md
├── internal
│   ├── cli                         # cobra CLI 命令
│   ├── app/service                 # 业务服务：proxy/config/connection/ip_resolver
│   ├── domain/model                # 数据模型与跨层常量
│   ├── infrastructure
│   │   ├── api                     # Mihomo REST + WebSocket 客户端
│   │   └── config                  # 配置加载、保存、默认路径
│   └── ui
│       ├── styles                  # 全局颜色与样式
│       └── tui
│           ├── model.go            # Bubble Tea 主 Model，仅保留全局共享状态
│           ├── update.go           # 全局消息路由与页面分发
│           ├── view.go             # 顶层布局
│           ├── commands.go         # WebSocket 启动与监听命令
│           ├── messages/events.go  # TUI 消息定义
│           ├── components          # layout/common 组件
│           └── features            # 页面特性：nodes/connections/logs/rules/settings/help
├── pkg
│   ├── i18n
│   └── utils
└── docs
```

---

## 4. 架构事实

Mihosh 采用分层结构：

```text
CLI 入口层         internal/cli
TUI 交互层         internal/ui/tui
业务服务层         internal/app/service
基础设施层         internal/infrastructure
数据模型层         internal/domain/model
工具与资源层       pkg
```

TUI 当前使用 Feature-Sliced 组织方式：

- 主 `Model` 只保存基础设施、路由布局、共享图表、全局错误、WebSocket 生命周期和各页面 State。
- 页面状态位于 `internal/ui/tui/features/<page>/state.go`。
- 页面渲染位于 `internal/ui/tui/features/<page>/view.go` 或其 `components/` 子目录。
- TUI 消息集中定义在 `internal/ui/tui/messages/events.go`，不要在多个文件散落新增同类消息。
- 页面切换通过 `layout.PageType` 和 `layout.PageCount` 管理；当前侧边栏页面为 Nodes、Connections、Logs、Rules、Settings。Help 是弹窗/特性视图，不是侧边栏页。

---

## 5. 开发原则

1. **KISS**
   - 只解决当前需求，不为了假想未来添加通用层。
   - 小范围重复优于无意义抽象。

2. **Fact-Based**
   - 修改前必须读取相关代码。
   - 以当前实现、测试和 `go.mod` 为事实来源，不沿用过期文档。

3. **First Principles**
   - 遇到性能、并发、布局、WebSocket 生命周期问题时，从数据流、goroutine、channel、状态更新与渲染约束分析。

---

## 6. 常用命令

```bash
# 安装依赖
go mod download

# 格式化
go fmt ./...

# 静态检查
go vet ./...

# 测试
go test ./...

# 编译
go build -o mihosh .

# 一次性检查
make check
```

Linux 环境如果缺少 `make`，直接运行等价命令：

```bash
go fmt ./...
go vet ./...
go test ./...
go build .
```

---

## 7. CLI 现状

默认运行 `mihosh` 会加载配置并启动 TUI；首次缺少配置时会引导初始化。

当前主要命令：

```text
mihosh
mihosh config init
mihosh config show [--output plain|table|json]
mihosh config set <key> <value>
mihosh config edit [--editor <editor>] [--path <mihomo-config>]
mihosh list [--output plain|table|json]
mihosh select <group> <node>
mihosh test [--output plain|table|json]
mihosh test node <node> [--output plain|table|json]
mihosh test group <group> [--output plain|table|json]
mihosh connections [--output plain|table|json]
mihosh mode [rule|global|direct]
mihosh version
```

输出格式解析集中在 `internal/cli/output_format.go`。新增 CLI 命令时：

- 放在 `internal/cli/<name>.go`。
- 在 `internal/cli/root.go` 注册。
- 网络、配置、参数错误使用现有 `wrap*Error` 分类。
- 可脚本化输出优先支持 `--output plain|table|json`。
- 为渲染函数写单元测试，不把测试绑定到真实 Mihomo 服务。

---

## 8. 配置规范

配置结构定义在 `internal/infrastructure/config/types.go`：

```yaml
api_address: http://127.0.0.1:9090
secret: ""
test_url: http://www.gstatic.com/generate_204
timeout: 5000
proxy_address: http://127.0.0.1:7890
language: auto
```

注意：

- `language` 支持 `auto` 及本地化资源中的语言。修改语言后需要刷新 i18n 与快捷键绑定。
- `proxy_address` 被 Connections 页面站点测试使用；Settings 页面保存后会同步到 `connections.State`。
- `config edit` 可以编辑 Mihosh 配置或 Mihomo 配置，Mihomo 配置修改后会尝试调用 API 热重载。

---

## 9. TUI 状态与消息规范

### 9.1 状态归属

新增页面内状态必须放在对应 `features/<page>/State` 中，不要直接塞进主 `Model`。

```go
// 正确：页面内状态进入页面 State
type State struct {
    selectedIndex int
    filterText    string
}

// 错误：页面内状态直接进入主 Model
type Model struct {
    selectedIndex int
    filterText    string
}
```

主 `Model` 仅允许保存这些类别：

- 基础设施对象：API client、service、config。
- 全局路由和布局：当前页面、宽高、帮助弹窗。
- 跨页面共享数据：`ChartData`、全局错误。
- WebSocket 生命周期：`WSClient`、消息 channel、context。
- 页面 State 引用。

### 9.2 消息定义

所有 TUI 自定义消息优先放在 `internal/ui/tui/messages/events.go`，使用 `XxxMsg` 命名。当前包含：

- API 数据：`GroupsMsg`、`ProxiesMsg`、`ConfigModeMsg`、`ConnectionsMsg`、`RulesMsg`。
- 测速：`TestDoneMsg`、`TestAllDoneMsg`、`SiteTestMsg`。
- 连接动作：`ConnectionClosedMsg`、`AllConnectionsClosedMsg`、`IPInfoMsg`。
- WebSocket：`MemoryWSMsg`、`TrafficWSMsg`、`ConnectionsWSMsg`、`LogsWSMsg`。
- 日志详情：`LogIPResolvedMsg`。
- 定时器：`ConnTickMsg`、`LogsTickMsg`。
- 错误：`ErrMsg`。

消息体保持轻量，不传递不必要的大对象。页面间通信通过 `tea.Msg` 或主 `Model` 的受控同步完成，不直接跨页面改内部字段。

### 9.3 更新流程

```text
Model.Update
├── 全局消息：窗口、鼠标、键盘、WebSocket、错误
├── 全局快捷键：退出、帮助、翻页、刷新
├── 数据消息：应用到对应页面 State
└── dispatchKeyToPage：按当前页面分发键盘输入
```

鼠标命中计算由 `resolveMainPageMouseHit` 统一提供页面相对坐标。新增鼠标交互时优先复用该坐标体系。

---

## 10. 页面职责

| 页面/特性 | 状态文件 | 重点职责 |
|---|---|---|
| Nodes | `features/nodes/state.go` | 策略组、节点列表、模式、单测、批量测速、选择节点 |
| Connections | `features/connections/state.go` | 活跃/历史连接、详情弹窗、IP 信息、Top N、站点测试、连接关闭 |
| Logs | `features/logs/state.go` | 日志 Ring Buffer、级别过滤、搜索、水平滚动、详情解析、内网源 IP 解析 |
| Rules | `features/rules/state.go` | 规则拉取、列表渲染、过滤 |
| Settings | `features/settings/state.go` | 配置编辑、保存、语言切换、代理地址同步 |
| Help | `features/help/view.go` | 全局帮助弹窗内容 |

新增页面时需要同时处理：

- `layout.PageType` / `PageCount` / 侧边栏文案。
- `Model` 中页面 State 初始化。
- `update.go` 页面切换、刷新、键盘与鼠标分发。
- `page_renders.go` 和视图渲染。
- i18n 文案与快捷键帮助。
- 聚焦的单元测试。

---

## 11. 并发、容量与生命周期

### 11.1 批量网络操作

批量网络操作必须限制并发。当前全局并发常量在 `internal/domain/model/constants.go`：

```go
const TestConcurrency = 20
```

`ProxyService.TestAllProxies` 使用 channel semaphore、`sync.WaitGroup` 和 `sync.Mutex` 汇总结果。新增批量测速、批量请求或批量解析时复用这个上限或给出明确理由。

### 11.2 Ring Buffer

固定长度历史数据必须使用 Ring Buffer，不要使用无限增长切片或反复头部插入。

当前容量常量：

```go
ClosedConnCap = 1000
LogsCap       = 1000
ChartPoints   = 60
```

当前使用位置：

- `connections.State.closedConns` / `closedTimes`：历史连接。
- `logs.State.logBuf`：日志缓存。
- `model.ChartData`：状态栏和连接页图表。

### 11.3 WebSocket

WebSocket 生命周期集中在 `internal/infrastructure/api/websocket.go`：

- `WSClient.Start` 启动 `memory`、`traffic`、`connections`、`logs?level=...` 四路流。
- `connectStream[T]` 负责统一连接、读取、JSON 解析、断线重连、停止信号响应。
- `WSClient.Stop` 关闭 stop channel 并关闭所有已记录连接。

新增实时数据源必须复用 `connectStream[T]`，不要复制一套重连循环。

---

## 12. UI 与交互注意事项

- 侧边栏宽度由当前语言文案动态计算，涉及布局时使用 `layout.SidebarWidth()`。
- 日志水平滚动由 `logs.State.UpdateMaxHScrollOffset` 约束，窗口大小变化时必须同步更新。
- 详情弹窗进入后应拦截当前页面按键，退出时清理 detail 状态与滚动状态。
- Connections 页面有两种视图：活跃连接与历史连接；筛选、详情、双击逻辑需要同时考虑。
- Connections 页面 Top N 弹窗和连接详情存在渲染优先级关系，改动前阅读 `features/connections/state.go` 与 `components/`。
- i18n 改动需要同时更新 `pkg/i18n/locales/zh-CN.json` 与 `pkg/i18n/locales/en-US.json`。

---

## 13. 测试策略

优先使用窄范围测试覆盖改动点，再运行全量验证。

常见测试位置：

- CLI 输出与错误：`internal/cli/*_test.go`
- TUI 设置语言：`internal/ui/tui/settings_language_test.go`
- 组件布局：`internal/ui/tui/components/layout/*_test.go`
- Nodes 页面：`internal/ui/tui/features/nodes/*_test.go`
- Connections 页面：`internal/ui/tui/features/connections/*_test.go`
- Logs 页面：`internal/ui/tui/features/logs/*_test.go`
- Config 基础设施：`internal/infrastructure/config/*_test.go`
- 工具函数：`pkg/utils/*_test.go`

完成代码改动前至少运行：

```bash
go test ./...
```

涉及编译、CLI 或跨包接口时再运行：

```bash
go build .
```

---

## 14. 常见陷阱

- 不要引用不存在的 `RTK.md` 作为项目规范；当前仓库没有该文件。
- 不要把页面局部状态加到主 `Model`。
- 不要新增不受控 goroutine 或无上限网络并发。
- 不要用无限切片保存日志、历史连接、统计点。
- 不要复制 WebSocket 重连逻辑。
- 不要在渲染层直接发起网络请求；网络动作应通过 `tea.Cmd` 或 service/API 层。
- 不要只改中文或只改英文 i18n 文案。
- 不要让 CLI 渲染函数只能写 `os.Stdout`；可测试渲染应接受 `io.Writer`。
- 不要在没有真实 Mihomo 服务的单元测试中依赖外部网络。

---

## 15. 提交规范

使用 Conventional Commits：

```text
<type>(<scope>): <subject>
```

常用 type：

- `feat`：新增功能
- `fix`：修复问题
- `refactor`：不改变行为的重构
- `perf`：性能优化
- `test`：测试
- `docs`：文档
- `chore`：构建、依赖、工具链

常用 scope：

- `cli`
- `tui`
- `service`
- `api`
- `config`
- `model`
- `i18n`
- `docs`

---

## 16. 开发工作流

复杂需求按以下顺序处理：

1. 读取相关代码和测试，确认当前事实。
2. 明确影响范围：CLI、TUI、service、API、config、model、i18n、测试。
3. 先写或更新能证明行为的测试。
4. 小步实现，遵循现有包边界。
5. 运行针对性测试。
6. 运行 `go test ./...`，必要时运行 `go build .` 或 `make check`。
7. 总结实际改动、验证结果和残留风险。

---

## 17. 参考资料

- Bubble Tea: https://github.com/charmbracelet/bubbletea
- Bubbles: https://github.com/charmbracelet/bubbles
- Lip Gloss: https://github.com/charmbracelet/lipgloss
- Cobra: https://cobra.dev/
- Viper: https://github.com/spf13/viper
- Gorilla WebSocket: https://pkg.go.dev/github.com/gorilla/websocket

@RTK.md
