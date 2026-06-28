package tui

import (
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/nodes"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/rules"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/settings"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/sub"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// connRefreshInterval 连接刷新间隔
const connRefreshInterval = 1 * time.Second

const (
	autoRefreshNoticeTicks = 3
	autoRefreshSyncedTicks = 1
)

// connTick 创建连接页面定时器
func connTick() tea.Cmd {
	return tea.Tick(connRefreshInterval, func(t time.Time) tea.Msg {
		return messages.ConnTickMsg(t)
	})
}

// logsTick 创建日志页面定时器
func logsTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return messages.LogsTickMsg(t)
	})
}

func autoRefreshTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return messages.AutoRefreshTickMsg(t)
	})
}

// reloadConfigCmd 通知 mihomo 核心重新加载配置文件。
// 失败回退为 ErrMsg（沿用现有全局错误显示），不回滚已写入的文件。
func reloadConfigCmd(client interface {
	ReloadConfig(path string) error
}) tea.Cmd {
	return func() tea.Msg {
		path, err := config.GetMihomoConfigPath()
		if err != nil {
			return messages.ConfigReloadedMsg{Err: err}
		}
		if err := client.ReloadConfig(path); err != nil {
			return messages.ConfigReloadedMsg{Err: err}
		}
		return messages.ConfigReloadedMsg{}
	}
}

// Init 初始化
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		nodes.FetchGroups(m.client),
		nodes.FetchProxies(m.client),
		nodes.FetchConfigMode(m.client),
		autoRefreshTick(),
		startWSStreams(m.wsClient, m.wsMsgChan),
		listenWSMessages(m.wsCtx, m.wsMsgChan),
		// 首次启动时自动将本地 mihomo 配置导入为本地订阅（幂等，已有则跳过）
		func() tea.Msg {
			p, err := m.profileSvc.AutoImportLocalSub()
			return messages.LocalSubImportedMsg{Profile: p, Err: err}
		},
	)
}

// Update 消息路由器：全局消息自处理，页面消息分发到子状态
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// ── 全局：窗口大小 ──
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.logsState = m.logsState.UpdateMaxHScrollOffset(m.width, m.height)
		m.settingsState = m.settingsState.SyncSysStatus(m.width, m.height, m.config, m.logsState.GetSysLogs())
		return m, tea.ClearScreen

	case tea.MouseMsg:
		// 帮助弹窗打开时吞掉所有鼠标事件，防止穿透到底层
		if m.showHelp {
			// 左键点击任意位置关闭弹窗
			if isMouseLeftPress(msg) {
				m.showHelp = false
			}
			return m, nil
		}
		// 报错弹窗打开时吞掉所有鼠标事件
		if m.showErrorPopup {
			if isMouseLeftPress(msg) {
				m.showErrorPopup = false
			}
			return m, nil
		}
		switch {
		case isMouseLeftPress(msg):
			statusBarHeight := common.StatusBarHeight
			contentHeight := m.height - statusBarHeight
			if contentHeight < common.MinContentHeight {
				contentHeight = common.MinContentHeight
			}

			// 检查是否点击了底栏 (Status Bar)
			if msg.Y >= contentHeight {
				if m.err != nil {
					// 粗略判断点击区域：假设状态栏左侧显示错误信息
					if msg.X >= 0 && msg.X < m.width/2 {
						m.showErrorPopup = true
						return m, nil
					}
				}

				activeProxy, _, exists := m.nodesState.GetActiveProxyAndDelay()
				if exists && activeProxy != "" && !m.nodesState.Testing {
					// 粗略判断点击区域：假设状态栏左侧宽约 50 个字符 (包含运行状态和节点名称及延时)
					if msg.X > 8 && msg.X < 50 {
						m.nodesState = m.nodesState.StartSingleTest(activeProxy)
						return m, nodes.TestProxy(m.client, activeProxy, m.testURL, m.timeout)
					}
				}
				// 消费掉底栏的点击事件，防止误触发主页面内容
				return m, nil
			}

			if msg.Y >= 0 && msg.Y < layout.TopNavHeight {
				clickedPage := layout.GetClickedTopNavPage(msg.X, msg.Y, m.width)
				if clickedPage >= 0 && clickedPage < layout.PageCount {
					m.currentPage = clickedPage
					return m, m.onPageChange()
				}
			}

			if m.currentPage == layout.PageNodes {
				return m.handleNodesMouseLeft(msg.X, msg.Y)
			}
			if m.currentPage == layout.PageConnections {
				return m.handleConnectionsMouseLeft(msg.X, msg.Y)
			}
			if m.currentPage == layout.PageRules {
				return m.handleRulesMouseLeft(msg.X, msg.Y)
			}
			if m.currentPage == layout.PageSub {
				return m.handleSubMouseLeft(msg.X, msg.Y)
			}
			if m.currentPage == layout.PageSettings {
				return m.handleSettingsMouseLeft(msg.X, msg.Y)
			}
			if m.currentPage == layout.PageLogs {
				return m.handleLogsMouseLeft(msg.X, msg.Y)
			}
		case isMouseWheelUp(msg):
			return m.handleMouseScroll(true, msg.X, msg.Y)
		case isMouseWheelDown(msg):
			return m.handleMouseScroll(false, msg.X, msg.Y)
		}
		return m, nil

	// ── 全局：键盘 ──
	case tea.KeyMsg:
		// 帮助弹窗拦截
		if m.showHelp {
			switch msg.String() {
			case "esc", "q", "?":
				m.showHelp = false
				return m, nil
			}
			if key.Matches(msg, common.Keys.Quit) {
				return m, tea.Quit
			}
			return m, nil
		}

		// 报错弹窗拦截
		if m.showErrorPopup {
			switch msg.String() {
			case "esc", "enter", "q":
				m.showErrorPopup = false
				return m, nil
			}
			if key.Matches(msg, common.Keys.Quit) {
				return m, tea.Quit
			}
			return m, nil
		}

		// 全局帮助
		if msg.String() == "?" {
			m.showHelp = true
			return m, nil
		}

		// 输入捕获模式：当前页面正在编辑/过滤时，优先分发按键到页面，
		// 避免全局快捷键（如 q/r/a/t/s/c）拦截输入字符。
		if m.isInputCapturing() {
			return m.dispatchKeyToPage(msg)
		}

		// 全局快捷键
		switch {
		case key.Matches(msg, common.Keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, common.Keys.NextPage):
			m.currentPage = (m.currentPage + 1) % layout.PageCount
			return m, m.onPageChange()

		case key.Matches(msg, common.Keys.PrevPage):
			m.currentPage = (m.currentPage + layout.PageCount - 1) % layout.PageCount
			return m, m.onPageChange()

		case key.Matches(msg, common.Keys.Refresh):
			return m, m.refreshCurrentPage()
		}

		// 分发到页面子状态
		return m.dispatchKeyToPage(msg)

	// ── 数据消息：分发到子状态 ──

	case messages.GroupsMsg:
		if m.noticeTicks <= 0 {
			m.notice = ""
		}
		m.nodesState = m.nodesState.ApplyGroups(msg.Groups, msg.OrderedNames)

	case messages.ProxiesMsg:
		if m.noticeTicks <= 0 {
			m.notice = ""
		}
		m.nodesState = m.nodesState.ApplyProxies(msg)

	case messages.ConfigModeMsg:
		if m.noticeTicks <= 0 {
			m.notice = ""
		}
		m.nodesState = m.nodesState.ApplyConfigMode(msg.Mode)

	case messages.ConnectionsMsg:
		m.connsState = m.connsState.ApplyConnections(msg.Resp)
		if msg.Resp != nil && m.chartData != nil {
			m.chartData.AddConnCountData(len(msg.Resp.Connections))
		}

	case messages.MemoryWSMsg:
		if m.chartData != nil {
			m.chartData.AddMemoryData(msg.Memory)
		}
		if m.wsMsgChan != nil {
			return m, listenWSMessages(m.wsCtx, m.wsMsgChan)
		}

	case messages.TrafficWSMsg:
		if m.chartData != nil {
			m.chartData.AddSpeedData(msg.Up, msg.Down)
		}
		if m.wsMsgChan != nil {
			return m, listenWSMessages(m.wsCtx, m.wsMsgChan)
		}

	case messages.ConnectionsWSMsg:
		m.connsState = m.connsState.ApplyWSConnections(msg.Data)
		if m.chartData != nil {
			m.chartData.AddConnCountData(len(msg.Data.Connections))
		}
		if m.wsMsgChan != nil {
			return m, listenWSMessages(m.wsCtx, m.wsMsgChan)
		}

	case messages.LogsWSMsg:
		m.logsState = m.logsState.AppendLog(msg.LogType, msg.Payload)
		if m.currentPage == layout.PageSettings && m.settingsState.ActiveTab() == 1 {
			m.settingsState = m.settingsState.SyncSysStatus(m.width, m.height, m.config, m.logsState.GetSysLogs())
		}
		if m.wsMsgChan != nil {
			return m, listenWSMessages(m.wsCtx, m.wsMsgChan)
		}

	case messages.LogIPResolvedMsg:
		m.logsState = m.logsState.ApplyIPResolved(msg.IP, msg.Resolved)

	case messages.RulesMsg:
		m.rulesState = m.rulesState.ApplyRules(msg)

	case messages.RuleAddedMsg:
		// 规则已写入配置文件：热重载核心 + 刷新规则列表 + 显示成功提示
		m.notice = i18n.T("rules.added_toast")
		m.noticeTicks = autoRefreshNoticeTicks
		return m, reloadConfigCmd(m.client)

	case messages.ConfigReloadedMsg:
		if msg.Err != nil {
			m.err = messages.ErrMsg{Err: msg.Err}
			return m, nil
		}
		if m.currentPage == layout.PageRules {
			return m, rules.FetchRules(m.client)
		}

	case messages.RuleAddErrorMsg:
		// 写入失败：沿用全局错误显示
		m.err = msg
		m.notice = ""
		m.noticeTicks = 0

	case messages.RuleDeletedMsg:
		// 规则已从配置文件删除：热重载核心 + 刷新规则列表 + 显示成功提示
		m.notice = i18n.T("rules.deleted_toast")
		m.noticeTicks = autoRefreshNoticeTicks
		return m, reloadConfigCmd(m.client)

	case messages.RuleDeleteErrorMsg:
		// 删除失败：沿用全局错误显示
		m.err = msg
		m.notice = ""
		m.noticeTicks = 0

	case messages.RuleEditedMsg:
		// 规则已修改：热重载核心 + 刷新规则列表 + 显示成功提示
		m.notice = i18n.T("rules.edit_toast")
		m.noticeTicks = autoRefreshNoticeTicks
		return m, reloadConfigCmd(m.client)

	case messages.RuleEditErrorMsg:
		// 修改失败：沿用全局错误显示
		m.err = msg
		m.notice = ""
		m.noticeTicks = 0

	case messages.ConfigEditFinishedMsg:
		// 外部编辑器结束：失败沿用全局错误显示；成功热重载核心（随后刷新规则列表）
		// tea.ExecProcess 期间会 ReleaseTerminal（禁用鼠标），但 RestoreTerminal
		// 不恢复鼠标模式（Bubble Tea v1.3.10 缺陷），需显式重新启用，否则退出编辑器后鼠标失效。
		reenableMouse := func() tea.Msg { return tea.EnableMouseCellMotion() }
		if msg.Err != nil {
			m.err = messages.ErrMsg{Err: msg.Err}
			m.notice = ""
			m.noticeTicks = 0
			return m, reenableMouse
		}
		m.notice = i18n.T("rules.edited_toast")
		m.noticeTicks = autoRefreshNoticeTicks
		return m, tea.Batch(reenableMouse, reloadConfigCmd(m.client))

	case messages.SiteTestMsg:
		m.connsState = m.connsState.ApplySiteTestResult(msg.Name, msg.Delay, msg.Err)

	case messages.TestDoneMsg:
		m.nodesState = m.nodesState.ApplyTestDone(msg.Name, msg.Delay, msg.Err, msg.TestURL)
		// 如果是批量测速，需要补位
		if m.nodesState.TestAllActive {
			var batchCmd tea.Cmd
			m.nodesState, batchCmd = m.nodesState.LaunchBatchTests(m.client, m.testURL, m.timeout)
			return m, batchCmd
		}
		return m, nodes.FetchProxies(m.client)

	case messages.TestAllDoneMsg:
		m.nodesState = m.nodesState.ApplyTestAllDone(msg.Results)
		return m, nodes.FetchProxies(m.client)

	case messages.IPInfoMsg:
		if msg.Info != nil {
			m.connsState = m.connsState.ApplyIPInfo(msg.Info)
		}

	case messages.ConnectionClosedMsg:
		m.connsState = m.connsState.ApplyConnectionClosed()

	case messages.AllConnectionsClosedMsg:
		m.connsState = m.connsState.ApplyAllConnectionsClosed()

	case messages.ConnTickMsg:
		if m.currentPage == layout.PageConnections {
			return m, connTick()
		}

	case messages.LogsTickMsg:
		if m.currentPage == layout.PageLogs {
			return m, logsTick()
		}

	case messages.SysStatusTickMsg, messages.SysStatusResultMsg:
		if m.currentPage == layout.PageSettings {
			var cmd tea.Cmd
			m.settingsState, cmd = m.settingsState.HandleMsg(msg, m.width, m.height, m.config, m.logsState.GetSysLogs())
			return m, cmd
		}

	case messages.AutoRefreshTickMsg:
		interval := m.autoRefreshInterval()
		m.advanceAutoRefreshTransientState()
		if interval <= 0 {
			m.autoRefreshRemaining = 0
			m.autoRefreshSynced = false
			m.autoRefreshSyncedTTL = 0
			m.notice = ""
			m.noticeTicks = 0
			return m, autoRefreshTick()
		}
		if m.autoRefreshRemaining <= 0 || m.autoRefreshRemaining > interval {
			m.autoRefreshRemaining = interval
		}
		m.autoRefreshRemaining--
		if m.autoRefreshRemaining <= 0 {
			m.autoRefreshRemaining = interval
			return m, tea.Batch(autoRefreshTick(), fetchAutoRefresh(m.client, m.nodesState))
		}
		return m, autoRefreshTick()

	case autoRefreshMsg:
		m.nodesState = m.nodesState.ApplyGroups(msg.Result.Groups, msg.Result.OrderedNames)
		m.nodesState = m.nodesState.ApplyProxies(msg.Result.Proxies)
		m.nodesState = m.nodesState.ApplyConfigMode(msg.Result.Mode)
		m.autoRefreshSynced = true
		m.autoRefreshSyncedTTL = autoRefreshSyncedTicks
		if msg.Changed {
			m.notice = i18n.T("status.auto_refreshed")
			m.noticeTicks = autoRefreshNoticeTicks
		}

	case messages.ErrMsg:
		m.err = msg
		m.showErrorPopup = false // 收到新错误时不要自动弹出，除非用户主动点击，如果当前开着弹窗则关掉。或者也可以保持不管。保险起见设为 false。
		m.notice = ""
		m.noticeTicks = 0
		m.nodesState.Testing = false
		m.nodesState.TestingTarget = ""
		m.nodesState.TestAllActive = false
		m.nodesState.TestAllPending = nil
		m.nodesState.TestAllRunning = nil
		m.nodesState.TestAllTotal = 0
		m.nodesState.TestAllDone = 0
		m.nodesState.TestPending = 0

	
	case messages.CoreActionDoneMsg:
		m.settingsState = m.settingsState.ClearActionStates()
		m.notice = msg.Action + " " + i18n.T("settings.action.success")
		m.noticeTicks = autoRefreshNoticeTicks
		if msg.NeedReloadAll {
			cmds := []tea.Cmd{
				nodes.FetchGroups(m.client),
				nodes.FetchProxies(m.client),
				nodes.FetchConfigMode(m.client),
				rules.FetchRules(m.client),
				settings.FetchMihomoVersion(m.client),
				m.configSvc.FetchMihomoConfig(m.client),
			}
			if msg.DelayMs > 0 {
				return m, tea.Sequence(
					tea.Tick(time.Duration(msg.DelayMs)*time.Millisecond, func(_ time.Time) tea.Msg { return nil }),
					tea.Batch(cmds...),
				)
			}
			return m, tea.Batch(cmds...)
		}

	case messages.CoreActionErrorMsg:
		m.settingsState = m.settingsState.ClearActionStates()
		m.err = messages.ErrMsg{Err: msg.Err}
		m.notice = ""
		m.noticeTicks = 0

	case messages.MihomoVersionMsg:
		m.settingsState = m.settingsState.ApplyMihomoVersion(msg.Version)

	case messages.MihomoConfigMsg:
		// Mihomo 标签页配置加载完成（成功或失败）。
		// 同步热刷新 client/wsClient 端点（external-controller/secret 变更），
		// 并把 mixed-port 派生的代理地址同步到 connections。
		m.settingsState = m.settingsState.ApplyMihomoConfig(&msg)
		if msg.Config != nil {
			m.reloadClients(msg.Config.ExternalController, msg.Config.Secret)
			m.connsState = m.connsState.UpdateProxyAddr(config.MixedPortToProxyURL(msg.Config.MixedPort))
			m.err = nil
		}

	case messages.MihomoConfigSavedMsg:
		// Mihomo 配置项已保存（写 YAML + 热重载）：toast 提示 + 重新拉取运行时配置。
		// external-controller/secret/mixed-port 变更后重新解析 endpoint 热刷新 client/wsClient。
		if msg.Err != nil && !msg.WriteOK {
			// YAML 写入就失败了（如路径不存在），显示硬错误
			m.err = messages.ErrMsg{Err: msg.Err}
		} else {
			// YAML 写入成功：即使 reload 失败也刷新端点（修改 external-controller 后旧端口不可达是预期行为）
			endpoint := config.ResolveMihomoEndpoint()
			m.reloadClients(endpoint.ExternalController, endpoint.Secret)
			m.connsState = m.connsState.UpdateProxyAddr(config.MixedPortToProxyURL(endpoint.MixedPort))
			if msg.Err != nil {
				// reload 失败但 YAML 已保存，提示用户配置已保存但需重启内核生效
				m.notice = i18n.T("settings.toast.mihomo_save_reload_warn")
			} else {
				m.notice = i18n.T("settings.toast.mihomo_save_success")
			}
			m.noticeTicks = autoRefreshNoticeTicks
			m.err = nil
		}
		return m, tea.Batch(m.fetchNodes(), m.configSvc.FetchMihomoConfig(m.client))

	case messages.ThemeChangedMsg:
		// 主题已切换，触发重绘（View 会读取新主题色）
		return m, tea.ClearScreen

	case messages.NoticeMsg:
		// 页面通过 tea.Cmd 推送的底栏通知
		m.notice = msg.Text
		m.noticeTicks = autoRefreshNoticeTicks
		return m, nil

	// ── 订阅管理 (Sub) 消息 ──
	case messages.SubsLoadedMsg:
		m.subState = m.subState.ApplySubs(msg.Subs, msg.Active)

	case messages.SubAddDoneMsg:
		// 新增成功：刷新列表 + 提示，并自动触发首次更新
		m.notice = i18n.T("sub.added_toast")
		m.noticeTicks = autoRefreshNoticeTicks
		m.subState = m.subState.SetUpdating(msg.UID)
		return m, tea.Sequence(
			sub.FetchSubs(m.profileSvc),
			sub.UpdateSubCmd(m.profileSvc, msg.UID),
		)

	case messages.SubAddErrorMsg:
		m.err = msg
		m.notice = ""
		m.noticeTicks = 0

	case messages.SubDeletedMsg:
		m.notice = i18n.T("sub.deleted_toast")
		m.noticeTicks = autoRefreshNoticeTicks
		return m, sub.FetchSubs(m.profileSvc)

	case messages.SubDeleteErrorMsg:
		m.err = msg
		m.notice = ""
		m.noticeTicks = 0

	case messages.SubFetchDoneMsg:
		m.subState = m.subState.ClearUpdating()
		m.notice = i18n.T("sub.updated_toast")
		m.noticeTicks = autoRefreshNoticeTicks
		return m, sub.FetchSubs(m.profileSvc)

	case messages.SubFetchErrorMsg:
		m.err = msg
		m.notice = ""
		m.noticeTicks = 0
		m.subState = m.subState.ClearUpdating()

	case messages.SubActivatedMsg:
		if msg.Err != nil {
			m.err = messages.ErrMsg{Err: msg.Err}
			m.notice = ""
			m.noticeTicks = 0
			return m, nil
		}
		notice := i18n.T("sub.activated_toast")
		if msg.MergeErr != nil {
			notice = i18n.T("sub.activated_merge_warn_toast")
		}
		m.notice = notice
		m.noticeTicks = autoRefreshNoticeTicks
		// 刷新订阅列表（更新激活态）+ 刷新节点信息（配置已重载）
		return m, tea.Batch(
			sub.FetchSubs(m.profileSvc),
			nodes.FetchGroups(m.client),
			nodes.FetchProxies(m.client),
		)

	case messages.SubMergeSavedMsg:
		m.notice = i18n.T("sub.merge_saved_toast")
		m.noticeTicks = autoRefreshNoticeTicks

	case messages.SubMergeSaveErrorMsg:
		m.err = msg
		m.notice = ""
		m.noticeTicks = 0

	case messages.SubEditDoneMsg:
		m.notice = i18n.T("sub.edited_toast")
		m.noticeTicks = autoRefreshNoticeTicks
		return m, sub.FetchSubs(m.profileSvc)

	case messages.SubEditErrorMsg:
		m.err = msg
		m.notice = ""
		m.noticeTicks = 0

	case messages.SubRawEditFinishedMsg:
		reenableMouse := func() tea.Msg { return tea.EnableMouseCellMotion() }
		if msg.Err != nil {
			m.err = messages.ErrMsg{Err: msg.Err}
		}
		m.notice = ""
		m.noticeTicks = 0
		return m, reenableMouse

	case messages.MergeEditFinishedMsg:
		// 外部编辑器结束：tea.ExecProcess 期间会 ReleaseTerminal（禁用鼠标），
		// 但 RestoreTerminal 不恢复鼠标模式（Bubble Tea v1.3.10 缺陷），需显式重新启用。
		reenableMouse := func() tea.Msg { return tea.EnableMouseCellMotion() }
		if msg.Err != nil {
			m.err = messages.ErrMsg{Err: msg.Err}
			m.notice = ""
			m.noticeTicks = 0
			return m, reenableMouse
		}
		// 如果有激活的订阅，则立即应用这份全局覆写并热重载
		if m.subState.ActiveUID() != "" {
			if msg.HasUnsupportedManagedFields {
				m.notice = i18n.T("sub.merge_applied_unsupported_toast")
			} else {
				m.notice = i18n.T("sub.merge_applied_toast")
			}
			m.noticeTicks = autoRefreshNoticeTicks
			return m, tea.Batch(reenableMouse, sub.ApplyActiveMergeCmd(m.profileSvc, m.subState.ActiveUID(), msg.HasUnsupportedManagedFields))
		}
		if msg.HasUnsupportedManagedFields {
			m.notice = i18n.T("sub.merge_saved_unsupported_toast")
		} else {
			m.notice = i18n.T("sub.merge_saved_toast")
		}
		m.noticeTicks = autoRefreshNoticeTicks
		return m, reenableMouse

	case messages.SubMergeAppliedMsg:
		// 覆写编辑后触发的重载流程结束（仅激活订阅会走到此分支）。
		if msg.Err != nil {
			m.err = messages.ErrMsg{Err: msg.Err}
			m.notice = ""
			m.noticeTicks = 0
			return m, nil
		}
		notice := i18n.T("sub.merge_applied_toast")
		if msg.HasUnsupportedManagedFields {
			notice = i18n.T("sub.merge_applied_unsupported_toast")
		} else if msg.MergeErr != nil {
			notice = i18n.T("sub.merge_applied_warn_toast")
		}
		m.notice = notice
		m.noticeTicks = autoRefreshNoticeTicks
		// 刷新订阅列表 + 刷新节点信息（核心已重载）
		return m, tea.Batch(
			sub.FetchSubs(m.profileSvc),
			nodes.FetchGroups(m.client),
			nodes.FetchProxies(m.client),
		)

	case messages.LocalSubImportedMsg:
		// 首次启动自动导入本地订阅完成（成功时刷新列表，失败时静默忽略）
		if msg.Profile != nil && msg.Err == nil {
			return m, sub.FetchSubs(m.profileSvc)
		}
	}

	return m, nil
}

// dispatchKeyToPage 将按键分发到当前页面子状态
func (m Model) dispatchKeyToPage(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.currentPage {
	case layout.PageNodes:
		m.nodesState, cmd = m.nodesState.Update(msg, m.client, m.proxySvc, m.testURL, m.timeout)

	case layout.PageConnections:
		m.connsState, cmd = m.connsState.Update(msg, m.client, m.timeout)

	case layout.PageLogs:
		m.logsState, cmd = m.logsState.Update(msg, m.ipResolver)

	case layout.PageRules:
		m.rulesState, cmd = m.rulesState.Update(msg, m.client)

	case layout.PageSub:
		m.subState, cmd = m.subState.Update(msg, m.profileSvc)

	case layout.PageSettings:
		var newCfg = m.config
		oldLanguage := ""
		if m.config != nil {
			oldLanguage = m.config.Language
		}
		m.settingsState, newCfg, cmd = m.settingsState.Update(msg, m.config, m.configSvc, m.client)
		m.settingsState = m.settingsState.SyncSysStatus(m.width, m.height, newCfg, m.logsState.GetSysLogs())
		m.config = newCfg
		if newCfg != nil && newCfg.Language != oldLanguage {
			i18n.SetLanguageOverride(newCfg.Language)
			common.InitKeyBindings()
			cmd = tea.Batch(cmd, tea.ClearScreen)
		}
		// 连接信息热刷新改由 MihomoConfigSavedMsg 驱动（Mihosh 配置不再含连接信息）。
		if newCfg != nil {
			m.autoRefreshRemaining = newCfg.AutoRefreshInterval
			m.autoRefreshSynced = false
			m.autoRefreshSyncedTTL = 0
			m.notice = ""
			m.noticeTicks = 0
		}
	}
	return m, cmd
}

// onPageChange 页面切换处理
func (m *Model) onPageChange() tea.Cmd {
	m.err = nil
	switch m.currentPage {
	case layout.PageConnections:
		m.connsState = m.connsState.ResetPrevConnIDs()
		return tea.Batch(connections.FetchConnections(m.client), connTick())
	case layout.PageLogs:
		return logsTick()
	case layout.PageRules:
		return rules.FetchRules(m.client)
	case layout.PageSub:
		return sub.FetchSubs(m.profileSvc)
	case layout.PageSettings:
		cmds := []tea.Cmd{settings.FetchMihomoVersion(m.client), m.configSvc.FetchMihomoConfig(m.client)}
		if m.settingsState.SysStatus.Supported && m.settingsState.ActiveTab() == 1 {
			cmds = append(cmds, settings.FetchSysStatusCmd())
		}
		m.settingsState = m.settingsState.SyncSysStatus(m.width, m.height, m.config, m.logsState.GetSysLogs())
		return tea.Batch(cmds...)
	}
	return nil
}

// refreshCurrentPage 刷新当前页面
func (m *Model) refreshCurrentPage() tea.Cmd {
	switch m.currentPage {
	case layout.PageNodes:
		return tea.Batch(nodes.FetchGroups(m.client), nodes.FetchProxies(m.client))
	case layout.PageRules:
		return rules.FetchRules(m.client)
	case layout.PageSub:
		return sub.FetchSubs(m.profileSvc)
	case layout.PageSettings:
		cfg, _ := m.configSvc.LoadConfig()
		m.config = cfg
		return tea.Batch(
			settings.FetchMihomoVersion(m.client),
			m.configSvc.FetchMihomoConfig(m.client),
		)
	}
	return nil
}

// handleMouseScroll 处理鼠标滚轮滚动
func (m Model) handleMouseScroll(up bool, x, y int) (tea.Model, tea.Cmd) {
	if y >= 0 && y < layout.TopNavHeight {
		if up {
			m.currentPage = (m.currentPage + layout.PageCount - 1) % layout.PageCount
		} else {
			m.currentPage = (m.currentPage + 1) % layout.PageCount
		}
		return m, m.onPageChange()
	}

	mainWidth := m.width
	if mainWidth < common.MinMainWidth {
		mainWidth = common.MinMainWidth
	}
	mainX := x
	mainY := y - layout.TopNavHeight
	mainHeight := m.height - layout.TopNavHeight

	switch m.currentPage {
	case layout.PageNodes:
		pageX, pageY, pageWidth, pageHeight, ok := m.resolveMainPageMouseHit(x, y)
		if ok {
			m.nodesState = m.nodesState.HandleMouseScroll(up, pageX, pageY, pageWidth, pageHeight)
		} else {
			// 如果无法解析，则回退到原始调用（不传坐标，虽然 nodesState 也会变）
			m.nodesState = m.nodesState.HandleMouseScroll(up, -1, -1, m.width, m.height)
		}
	case layout.PageConnections:
		var cmd tea.Cmd
		m.connsState, cmd = m.connsState.HandleMouseScroll(up, mainX, mainY, mainWidth, mainHeight)
		if cmd != nil {
			return m, cmd
		}
	case layout.PageLogs:
		m.logsState = m.logsState.HandleMouseScroll(up, mainHeight)
	case layout.PageRules:
		m.rulesState = m.rulesState.HandleMouseScroll(up)
	case layout.PageSub:
		m.subState = m.subState.HandleMouseScroll(up)
	case layout.PageSettings:
		var cmd tea.Cmd
		_, pageY, _, pageHeight, ok := m.resolveMainPageMouseHit(x, y)
		// actionsPanelBottomY is roughly the height used by Tab bar (3) + Spacer (1) + Config Panel (8) + Spacer (1) + Actions Panel (4) + Desc/Warning (1)
		actionsPanelBottomY := 18
		if ok {
			m.settingsState, cmd = m.settingsState.HandleMouseScroll(up, pageY, pageHeight, actionsPanelBottomY)
		} else {
			m.settingsState, cmd = m.settingsState.HandleMouseScroll(up, -1, pageHeight, actionsPanelBottomY)
		}
		if cmd != nil {
			return m, cmd
		}
	}

	return m, nil
}

func (m Model) handleNodesMouseLeft(x, y int) (tea.Model, tea.Cmd) {
	pageX, pageY, pageWidth, pageHeight, ok := m.resolveMainPageMouseHit(x, y)
	if !ok {
		return m, nil
	}

	var cmd tea.Cmd
	m.nodesState, cmd = m.nodesState.HandleMouseLeft(pageX, pageY, pageWidth, pageHeight, m.client)
	return m, cmd
}

func (m Model) handleConnectionsMouseLeft(x, y int) (tea.Model, tea.Cmd) {
	pageX, pageY, pageWidth, pageHeight, ok := m.resolveMainPageMouseHit(x, y)
	if !ok {
		return m, nil
	}

	var cmd tea.Cmd
	m.connsState, cmd = m.connsState.HandleMouseLeft(pageX, pageY, pageWidth, pageHeight, m.chartData, m.timeout)
	return m, cmd
}

func (m Model) handleRulesMouseLeft(x, y int) (tea.Model, tea.Cmd) {
	pageX, pageY, pageWidth, pageHeight, ok := m.resolveMainPageMouseHit(x, y)
	if !ok {
		return m, nil
	}

	var cmd tea.Cmd
	m.rulesState, cmd = m.rulesState.HandleMouseLeft(pageX, pageY, pageWidth, pageHeight, m.client)
	return m, cmd
}

func (m Model) handleSubMouseLeft(x, y int) (tea.Model, tea.Cmd) {
	pageX, pageY, pageWidth, pageHeight, ok := m.resolveMainPageMouseHit(x, y)
	if !ok {
		return m, nil
	}

	var cmd tea.Cmd
	m.subState, cmd = m.subState.HandleMouseLeft(pageX, pageY, pageWidth, pageHeight, m.profileSvc)
	return m, cmd
}

func (m Model) handleSettingsMouseLeft(x, y int) (tea.Model, tea.Cmd) {
	pageX, pageY, pageWidth, pageHeight, ok := m.resolveMainPageMouseHit(x, y)
	if !ok {
		return m, nil
	}

	oldLanguage := ""
	oldTheme := ""
	if m.config != nil {
		oldLanguage = m.config.Language
		oldTheme = m.config.Theme
	}

	var cmd tea.Cmd
	m.settingsState, m.config, cmd = m.settingsState.HandleMouseLeft(pageX, pageY, pageWidth, pageHeight, m.config, m.configSvc, m.client)
	m.settingsState = m.settingsState.SyncSysStatus(m.width, m.height, m.config, m.logsState.GetSysLogs())
	if m.config != nil && m.config.Language != oldLanguage {
		i18n.SetLanguageOverride(m.config.Language)
		common.InitKeyBindings()
		return m, tea.Batch(cmd, tea.ClearScreen)
	}
	if m.config != nil && m.config.Theme != oldTheme {
		return m, tea.Batch(cmd, tea.ClearScreen)
	}
	// 连接信息热刷新改由 MihomoConfigSavedMsg 驱动（Mihosh 配置不再含连接信息）。
	if m.config != nil {
		m.autoRefreshRemaining = m.config.AutoRefreshInterval
		m.autoRefreshSynced = false
		m.autoRefreshSyncedTTL = 0
		m.notice = ""
		m.noticeTicks = 0
	}
	return m, cmd
}

func (m *Model) advanceAutoRefreshTransientState() {
	if m.autoRefreshSyncedTTL > 0 {
		m.autoRefreshSyncedTTL--
		if m.autoRefreshSyncedTTL == 0 {
			m.autoRefreshSynced = false
		}
	}
	if m.noticeTicks > 0 {
		m.noticeTicks--
		if m.noticeTicks == 0 {
			m.notice = ""
		}
	}
}

func (m Model) topNavActive() bool {
	return false
}

// isInputCapturing 判断当前页面是否处于会捕获按键的输入/过滤模式。
// 处于这些模式时，所有按键必须分发到页面用于文本输入或弹窗操作，
// 避免被全局快捷键（q/r/a/t/s/c 等）拦截。
func (m Model) isInputCapturing() bool {
	switch m.currentPage {
	case layout.PageNodes:
		return m.nodesState.NodeFilterMode
	case layout.PageConnections:
		return m.connsState.FilterMode()
	case layout.PageLogs:
		return m.logsState.FilterMode()
	case layout.PageRules:
		return m.rulesState.FilterMode() || m.rulesState.ShowTypeFilter() || m.rulesState.ShowAddForm() || m.rulesState.ShowDeleteConfirm()
	case layout.PageSub:
		return m.subState.Querying()
	case layout.PageSettings:
		return m.settingsState.IsEditing()
	}
	return false
}

func (m Model) autoRefreshInterval() int {
	if m.config == nil {
		return 0
	}
	return m.config.AutoRefreshInterval
}

// reloadClients 把 api.Client 与 WSClient 的端点更新为新值。
// api.Client 立即生效（后续 DoRequest 读新值）；
// WSClient 主动断开旧连接，由 connectStream 在下一轮用新端点重连。
// 不重建对象：所有引用 m.client / m.wsClient 的页面状态自动获得新端点。
func (m Model) reloadClients(apiAddress, secret string) {
	if m.client != nil {
		m.client.UpdateEndpoint(apiAddress, secret)
	}
	if m.wsClient != nil {
		m.wsClient.UpdateEndpoint(apiAddress, secret)
	}
}

// fetchNodes 批量拉取节点信息（用于端点切换后刷新）。
func (m Model) fetchNodes() tea.Cmd {
	return tea.Batch(
		nodes.FetchGroups(m.client),
		nodes.FetchProxies(m.client),
		nodes.FetchConfigMode(m.client),
	)
}

func (m Model) handleLogsMouseLeft(x, y int) (tea.Model, tea.Cmd) {
	pageX, pageY, pageWidth, _, ok := m.resolveMainPageMouseHit(x, y)
	if !ok {
		return m, nil
	}

	var cmd tea.Cmd
	m.logsState, cmd = m.logsState.HandleMouseLeft(pageY, pageX, pageWidth, m.ipResolver)
	return m, cmd
}

func (m Model) resolveMainPageMouseHit(x, y int) (pageX, pageY, pageWidth, pageHeight int, ok bool) {
	statusBarHeight := common.StatusBarHeight
	contentHeight := m.height - statusBarHeight - layout.TopNavHeight
	if contentHeight < common.MinContentHeight {
		contentHeight = common.MinContentHeight
	}
	contentYAbs := y - layout.TopNavHeight
	if contentYAbs < 0 || contentYAbs >= contentHeight {
		return 0, 0, 0, 0, false
	}

	mainWidth := m.width
	if mainWidth < common.MinMainWidth {
		mainWidth = common.MinMainWidth
	}

	// 移除外边框检查，内容直接从顶部开始
	pageY = contentYAbs
	pageHeight = m.height - layout.TopNavHeight - common.StatusBarHeight
	if pageHeight < common.MinContentHeight {
		pageHeight = common.MinContentHeight
	}
	pageWidth = mainWidth
	if pageWidth < common.MinMainWidth {
		pageWidth = common.MinMainWidth
	}
	pageX = x
	if pageX < 0 || pageX >= pageWidth {
		return 0, 0, 0, 0, false
	}

	return pageX, pageY, pageWidth, pageHeight, true
}

func isMouseLeftPress(msg tea.MouseMsg) bool {
	if msg.Button == tea.MouseButtonLeft {
		return msg.Action == tea.MouseActionPress
	}
	return msg.Type == tea.MouseLeft
}

func isMouseWheelUp(msg tea.MouseMsg) bool {
	if msg.Button == tea.MouseButtonWheelUp {
		return msg.Action == tea.MouseActionPress
	}
	return msg.Type == tea.MouseWheelUp
}

func isMouseWheelDown(msg tea.MouseMsg) bool {
	if msg.Button == tea.MouseButtonWheelDown {
		return msg.Action == tea.MouseActionPress
	}
	return msg.Type == tea.MouseWheelDown
}
