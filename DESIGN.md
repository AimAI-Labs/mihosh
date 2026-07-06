# Mihosh 设计规范 — Tokyo Night

## 概述

Mihosh 是一款基于 Mihomo 的终端代理管理工具。项目内置多主题引擎（默认 **Tokyo Night** 暗色主题，另支持 Catppuccin、Gruvbox、Nord、Dracula）。本文档以 Tokyo Night 为示例定义颜色体系、组件样式、布局规则和交互模式，作为 UI 开发的统一参考。

> 注意：所有颜色均通过 `theme.Current()` 获取语义色，禁止硬编码具体色值。下表中的具体色值仅为 Tokyo Night 主题示例。

## 颜色体系

### 基础调色板

Tokyo Night 源自 [Tokyo Night VS Code 主题](https://github.com/enkia/tokyo-night-vscode-theme)，以深蓝灰为底，辅以高饱和度的冷色系强调色。

| 角色 | 名称 | 色值 | 用途 |
|------|------|------|------|
| 背景 | `TokyoPanel` | `#1A1B26` | 面板/页面底色 |
| 选中背景 | `TokyoSelected` | `#292E42` | 列表选中行背景 |
| 正文 | `TokyoForeground` | `#C0CAF5` | 主要文字 |
| 次要文字 | `TokyoMuted` | `#565F89` | 灰色/禁用/辅助信息 |
| 蓝色 | `TokyoBlue` | `#7AA2F7` | 主强调色/边框/标题 |
| 青色 | `TokyoCyan` | `#7DCFFF` | 选中态指示器/高亮 |
| 绿色 | `TokyoGreen` | `#9ECE6A` | 成功/低延迟（<100ms） |
| 黄色 | `TokyoYellow` | `#E0AF68` | 警告/中延迟（100-300ms） |
| 红色 | `TokyoRed` | `#F7768E` | 错误/高延迟（>300ms） |
| 紫色 | `TokyoPurple` | `#BB9AF7` | 图表下载/强调 |

### 语义色映射

| 语义 | 颜色 | 说明 |
|------|------|------|
| 成功/在线 | `TokyoGreen` | 测试通过、连接正常、低延迟 |
| 警告/测试中 | `TokyoYellow` | 正在测速、中等延迟 |
| 错误/离线 | `TokyoRed` | 超时、连接失败、REJECT 节点 |
| 信息/选中 | `TokyoCyan` | 当前选中项指示器（`┃`） |
| 主操作 | `TokyoBlue` | 边框、标题、模式切换激活态 |
| 禁用/静音 | `TokyoMuted` | 未选中文字、分隔线、滚动轨道 |

### 延迟色阶梯

```go
func GetDelayColor(delay int) lipgloss.Color {
    t := theme.Current()
    switch {
    case delay == 0:   return t.Muted     // 未测试
    case delay < 100:  return t.Success   // 快
    case delay < 300:  return t.Warning   // 中
    default:           return t.Danger    // 慢
    }
}
```

### 多主题支持

通过 `internal/ui/theme` 包实现多主题切换。每个主题定义了统一的语义色集合：

| 字段 | 类型 | 说明 |
|------|------|------|
| `Foreground` | 前景 | 主要文字 |
| `Muted` | 前景 | 灰色/禁用/辅助信息 |
| `Dim` | 前景 | 比 Muted 更暗的文字 |
| `Bright` | 前景 | 最亮文字 |
| `Background` | 背景 | 面板/页面底色 |
| `Surface` | 背景 | 浮层、卡片底色 |
| `Overlay` | 背景 | 叠加层 |
| `Selected` | 背景 | 列表选中行背景 |
| `Primary` | 强调 | 主强调色/边框/标题 |
| `Secondary` | 强调 | 辅助强调色 |
| `Success` | 语义 | 成功/在线/低延迟 |
| `Warning` | 语义 | 警告/测试中/中等延迟 |
| `Danger` | 语义 | 错误/离线/高延迟 |
| `Info` | 语义 | 选中态指示器/高亮 |
| `Border` | 扩展 | 边框/分隔线 |
| `Active` | 扩展 | 活跃状态指示 |
| `Orange` | 扩展 | 橙色强调 |
| `Purple` | 扩展 | 紫色强调/图表下载 |

可用主题：`tokyo-night`（默认）、`catppuccin`、`gruvbox`、`nord`、`dracula`。

切换 API：
```go
theme.SetTheme("catppuccin") // 切换主题
t := theme.Current()          // 获取当前主题
theme.Names()                 // 所有可用主题名
```

## 组件规范

### 1. Tokyo 面板 (`RenderTokyoPanel`)

所有内容区域统一使用带圆角边框的 Tokyo 面板包裹。

```
╭─ 面板标题 ────────────────────╮
│ 内容区域                        │
│                                │
╰────────────────────────────────╯
```

- **边框颜色**：`TokyoBlue`
- **标题**：嵌入顶部边框，格式 `╭─ {title} ──╮`
- **内边距**：左右各 1 字符
- **最小宽度**：24 字符
- **边框字符**：`╭╮╰╯│─`（圆角 Unicode）

### 2. 模式切换栏

与导航栏风格一致的带边框按钮组。

```
╭────────────────────────────╮
│ 规则 │ 全局 │ 直连          │
╰────────────────────────────╯
```

- **激活态**：背景 `TokyoSelected`，前景 `TokyoCyan`，加粗
- **非激活态**：前景 `TokyoBlue`
- **分隔符**：`│`，颜色 `TokyoMuted`
- **边框**：同 Tokyo 面板

### 3. 列表项

#### 选中行

- **左侧指示器**：`┃`（`TokyoCyan`，加粗）
- **行背景**：`TokyoSelected`（`#292E42`）
- **文字**：`TokyoCyan`，加粗

#### 未选中行

- **左侧指示器**：空格（`  `）
- **文字**：`TokyoForeground`（`#C0CAF5`）

#### 状态灯

- `●` 字符，颜色根据状态变化
- 特殊节点：REJECT → `TokyoRed`，DIRECT → `TokyoMuted`
- 普通节点：使用延迟色阶梯

### 4. 滚动条

```
┃  ← 滚动条拇指（TokyoMuted）
│  ← 滚动条轨道（TokyoMuted）
```

- 仅当列表项总数超过可视区域时显示
- 拇指位置根据滚动偏移计算

### 5. 顶部导航栏

```
╭──────────────────────────────────────────────────╮
│ 节点管理 │ 连接监控 │ 日志 │ 规则 │ 订阅 │ 设置 │
╰──────────────────────────────────────────────────╯
```

- **激活标签**：背景 `#292E42`，前景 `TokyoBlue`，加粗
- **非激活标签**：前景 `TokyoGray`
- **自动刷新状态**：右侧显示，`TokyoGreen`

### 6. 底部状态栏

```
────────────────────────────────────────────────────
✔ 运行正常                    ↑1.2MB/s ↓3.4MB/s │ MEM 128MB
```

- **分隔线**：`─` 字符，颜色 `ColorBorder`（`#414868`）
- **错误状态**：`✗` + 红色文字（`ColorDanger`）
- **正常状态**：`✔` + 灰色文字（`ColorGray`）
- **测试中**：黄色文字（`ColorWarning`）
- **上传速度**：绿色（`ColorSuccess`）
- **下载速度**：蓝色（`ColorPrimary`）

### 7. 弹窗/模态框

```
╭────────────────────────────╮
│ 标题                        │
│ 副标题                      │
│ ────────────────────────── │
│ 内容...                     │
╰────────────────────────────╯
```

- **边框**：`lipgloss.RoundedBorder()`
- **错误弹窗边框色**：`#E74C3C`
- **叠加方式**：居中覆盖在当前页面上

### 8. 页脚提示

- **样式**：`FooterStyle`（灰色文字 `ColorGray`，左右内边距 1）
- **位置**：固定在页面底部（通过空行填充实现）
- **自适应**：根据终端高度动态调整填充行数

## 布局规则

### 整体结构

```
┌─────────────────────────────────┐
│          顶部导航栏 (3行)        │
├─────────────────────────────────┤
│                                 │
│          页面内容区              │
│                                 │
├─────────────────────────────────┤
│          底部状态栏 (2行)        │
└─────────────────────────────────┘
```

- **顶部导航高度**：3 行（上边框 + 内容 + 下边框）
- **底部状态栏高度**：2 行（分隔线 + 信息行）
- **最小内容高度**：5 行

### 宽窄屏适配

| 断点 | 模式 | 说明 |
|------|------|------|
| ≥ 100 列 | 宽屏 | 节点页面双面板并排（策略组 43% + 节点列表 57%） |
| < 100 列 | 窄屏 | 节点页面双面板垂直堆叠 |

### 面板布局（节点页面）

#### 宽屏模式

```
╭─ 模式切换 ────────────────────╮
│ 规则 │ 全局 │ 直连              │
╰───────────────────────────────╯

╭─ 策略组 ──────╮  ╭─ 节点列表 ─────────╮
│ 名称 │ 类型 │ 当前 │  │ 名称 │ 延迟 │ 状态 │
│ ...  │ ...  │ ...  │  │ ...  │ ...  │ ...  │
╰────────────────╯  ╰────────────────────╯
```

- 策略组面板宽度：内容区的 43%
- 节点列表面板宽度：内容区的 57% - 间距
- 面板间距：2 字符

#### 窄屏模式

```
╭─ 模式切换 ────────────────────╮
│ 规则 │ 全局 │ 直连              │
╰───────────────────────────────╯

╭─ 策略组 ──────────────────────╮
│ 名称 │ 类型 │ 当前              │
╰───────────────────────────────╯

╭─ 节点列表 ────────────────────╮
│ 名称 │ 延迟 │ 状态              │
╰───────────────────────────────╯
```

## 文字排版

### 截断规则

- 超出宽度时末尾加 `~` 替代省略号
- 精确处理中文等宽字符（`runewidth` 库）

### 对齐

- 列表表头与数据列左对齐
- 列宽根据内容动态计算，设置最小/最大约束
- 右侧状态指标右对齐

## 边框字符集

| 字符 | 用途 | Unicode |
|------|------|---------|
| `╭` | 左上角（圆角） | U+256D |
| `╮` | 右上角（圆角） | U+256E |
| `╰` | 左下角（圆角） | U+2570 |
| `╯` | 右下角（圆角） | U+256F |
| `│` | 垂直线 | U+2502 |
| `─` | 水平线 | U+2500 |
| `┃` | 粗垂直线（选中指示/滚动拇指） | U+2503 |
| `●` | 圆点（状态灯） | U+25CF |
| `✓` | 勾号（选中/完成） | U+2713 |
| `✗` | 叉号（错误） | U+2717 |
| `►` | 三角（选择指示） | U+25BA |
| `█` | 全块字符（图表柱状） | U+2588 |

## 字体与终端兼容性

- 依赖终端对 Unicode Box Drawing 字符的支持
- 推荐使用 Nerd Font 或支持 CJK 的等宽字体
- 最小终端宽度：20 列（强制约束）
- 最小终端高度：通过 `MinContentHeight` 保证内容可读

## 文件结构

```
internal/ui/
├── styles/
│   └── styles.go                # 全局调色板与通用样式
├── theme/
│   ├── theme.go                 # 主题引擎（Current/SetTheme/Names）
│   └── builtin.go               # 内置主题定义（tokyo-night/catppuccin/gruvbox/nord/dracula）
└── tui/
    ├── model.go                 # 主 Model 定义
    ├── view.go                  # 主视图组装
    ├── update.go                # Update 路由分发
    ├── update_keys.go           # 键盘事件处理
    ├── update_mouse.go          # 鼠标事件处理（含 resolveMainPageMouseHit）
    ├── update_ws.go             # WebSocket 状态派发
    ├── commands.go              # tea.Cmd 工厂函数
    ├── auto_refresh.go          # 自动刷新逻辑
    ├── page_renders.go          # 页面渲染入口
    ├── components/
    │   ├── common/
    │   │   ├── colors.go            # 基础颜色常量
    │   │   ├── styles.go            # 通用样式预设
    │   │   ├── constants.go         # 布局常量与符号
    │   │   ├── keys.go              # 快捷键绑定
    │   │   ├── tokyo_panel.go       # Tokyo 面板组件
    │   │   ├── footer.go            # 页脚组件
    │   │   ├── sparkline.go         # 图表组件
    │   │   ├── scrollbar.go         # 滚动条组件
    │   │   ├── filter.go            # 过滤器工具函数
    │   │   ├── filter_list.go       # 可复用过滤列表组件
    │   │   ├── double_click.go      # 双击检测器
    │   │   ├── clamp_scroll.go      # 滚动边界限制
    │   │   ├── inline_help.go       # 内联帮助提示
    │   │   └── toast.go             # Toast 通知组件
    │   └── layout/
    │       ├── top_nav.go           # 顶部导航栏（含 PageType 定义）
    │       └── statusbar.go         # 底部状态栏
    ├── messages/
    │   └── events.go                # 集中定义所有 TUI 消息
    └── features/
        ├── nodes/                   # 节点管理页面
        │   ├── state.go
        │   ├── commands.go
        │   ├── view.go
        │   └── view_components.go
        ├── connections/             # 连接监控页面
        │   ├── state.go / state_apply.go / state_detail.go / state_filter.go / state_mouse.go / state_topn.go
        │   ├── commands.go
        │   ├── view.go / view_components.go
        │   └── components/              # 连接页子组件（charts / detail / topn / site_card）
        ├── logs/                    # 日志页面
        │   ├── state.go
        │   ├── commands.go
        │   ├── view.go
        │   ├── detail.go
        │   └── parse.go
        ├── rules/                   # 规则页面（已拆分）
        │   ├── state.go / state_confirm.go / state_filter.go / state_form.go / state_mouse.go
        │   ├── commands.go / editor.go
        │   ├── view.go / view_list.go / view_filter.go / view_add_form.go / view_proxy_picker.go
        │   ├── view_colors.go / view_confirm.go / edit_form_view.go
        │   └── add_form.go
        ├── sub/                     # 订阅管理页面
        │   ├── state.go
        │   ├── commands.go
        │   ├── view.go
        │   ├── add_form.go
        │   ├── merge_editor.go / raw_editor.go
        │   └── *_test.go
        ├── settings/                # 设置页面
        │   ├── state.go
        │   ├── view.go
        │   ├── actions_layout.go
        │   └── sys_status.go
        └── help/                    # 帮助页面（快捷键参考）
            └── view.go
```
