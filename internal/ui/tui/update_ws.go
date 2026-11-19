package tui

import (
	"fmt"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/preferences"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/nodes"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/rules"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/sub"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"

	"github.com/AimAI-Labs/mihosh/pkg/i18n"
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

func (m Model) handleWSMessages(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case messages.CoreHTTPMsg:
		if m.client == nil || msg.Generation != m.client.Generation() {
			return m, nil
		}
		return m.handleWSMessages(msg.Payload)
	case messages.CoreOwnerChangedMsg:
		m.reloadStoredConfig()
		update := coreTargetReconnect
		if msg.Action == "stop" {
			update = coreTargetStopped
		}
		cmd := m.updateCoreTarget(config.ResolveMihomoEndpoint(), update)
		return m, tea.Batch(m.listenCoreOwner(), m.coreState.Refresh(), cmd)
	case messages.CoreWSMsg:
		if m.wsClient == nil || msg.Generation != m.wsClient.Generation() {
			return m, listenWSMessages(m.wsCtx, m.wsMsgChan)
		}
		return m.handleWSMessages(msg.Payload)

	case messages.ManagedCoreStartedMsg:
		if msg.Epoch != m.connectionEpoch {
			return m, nil
		}
		m.reloadStoredConfig()
		return m, m.updateCoreTarget(config.ResolveMihomoEndpoint(), coreTargetReconnect)

	case messages.CoreConnectionMsg:
		if msg.Epoch != m.connectionEpoch {
			return m, nil
		}
		m.coreState, _ = m.coreState.Update(msg)
		m.connectionChecked = true
		m.connectionReady = msg.Err == nil
		m.syncProcessLogs()
		if msg.Err != nil {
			m.currentPage = layout.PageCore
			return m, nil
		}
		var stream tea.Cmd
		if !m.wsStarted {
			m.wsStarted = true
			stream = startWSStreams(m.wsCtx, m.wsClient, m.wsMsgChan)
		}
		if m.autoRefreshInterval() > 0 {
			// Run one follow-up auto-refresh on the next tick so delayed core startup data appears automatically.
			m.autoRefreshRemaining = 1
		}
		return m, tea.Batch(stream, m.refreshOnConnect())

	case messages.CoreStatusMsg, messages.CoreGeoDataMsg, messages.CoreProgressTickMsg, messages.CoreOperationMsg, messages.SysStatusResultMsg:
		if result, ok := msg.(messages.CoreOperationMsg); ok && result.ID != m.coreState.OperationID() {
			return m, nil
		}
		var cmd tea.Cmd
		m.coreState, cmd = m.coreState.Update(msg)
		if result, ok := msg.(messages.CoreOperationMsg); ok {
			var targetCmd tea.Cmd
			if result.Err == nil {
				targetCmd = m.coreOperationCompleted(result.Action)
			}
			m.syncProcessLogs()
			return m, tea.Batch(cmd, targetCmd)
		}
		if statusMsg, ok := msg.(messages.CoreStatusMsg); ok && statusMsg.Err == nil {
			m.syncProcessLogs()
			if statusMsg.Status.Mode == "managed" && !statusMsg.Status.Running && m.connectionReady {
				m.reloadStoredConfig()
				return m, tea.Batch(cmd, m.updateCoreTarget(config.ResolveMihomoEndpoint(), coreTargetStopped))
			}
		}
		return m, cmd

	case messages.ShowGuideModalMsg:
		m.guideState = m.guideState.Activate(msg.Endpoint, msg.SecretMasked, msg.ErrMessage)
		return m, nil

	case messages.HideGuideModalMsg:
		if m.guideState.Active {
			m.notice = i18n.T("status.normal")
			m.noticeTicks = autoRefreshNoticeTicks
		}
		m.guideState = m.guideState.Dismiss()
		return m, nil

	case messages.UpdateCheckedMsg:
		if msg.Info != nil {
			m.updateInfo = msg.Info
			m.hasUpdate = true
		}

	case messages.UpdateAppliedMsg:
		if msg.Err != nil {
			m.updateError = msg.Err
			m.isUpdating = false
			return m, nil
		}
		m.notice = i18n.T("status.update_applied")
		if m.notice == "status.update_applied" {
			m.notice = "Update applied, please restart."
		}
		m.noticeTicks = autoRefreshNoticeTicks
		return m, tea.Quit

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
		if !m.usesProcessLogs() {
			m.logsState = m.logsState.AppendLog(msg.LogType, msg.Payload)
		}
		if m.currentPage == layout.PageCore && m.coreState.IsMihomoPage() {
			pw, ph := m.getPageSize()
			m.coreState, _ = m.coreState.HandleEditorMsg(nil, pw, ph, m.logsState.GetSysLogs())
		}
		if m.wsMsgChan != nil {
			return m, listenWSMessages(m.wsCtx, m.wsMsgChan)
		}

	case messages.LogIPResolvedMsg:
		m.logsState = m.logsState.ApplyIPResolved(msg.IP, msg.Resolved)

	case messages.RulesMsg:
		m.rulesState = m.rulesState.ApplyRules(msg)

	case messages.RuleEditAppliedMsg:
		if msg.Err != nil {
			m.err = messages.ErrMsg{Err: msg.Err}
			m.notice, m.noticeTicks = "", 0
			return m, nil
		}
		m.err = nil
		notice := msg.NoticeKey
		if msg.Remote || msg.NeedsRestart || msg.Restarted {
			notice = mihomoOverrideNotice(messages.MihomoOverridesSavedMsg{
				Applied: msg.Applied, Remote: msg.Remote, NeedsRestart: msg.NeedsRestart, Restarted: msg.Restarted,
			})
		} else if !msg.Applied {
			notice = "rules.write.saved"
		}
		m.notice = i18n.T(notice)
		m.noticeTicks = autoRefreshNoticeTicks
		if !msg.Remote && (msg.Restarted || msg.EndpointChanged) {
			if cmd := m.syncAppliedCore(msg.Restarted, msg.NeedsRestart && msg.Applied); cmd != nil {
				return m, cmd
			}
		}
		if msg.Applied && m.client != nil {
			return m, rules.FetchRules(m.client)
		}
		return m, nil

	case messages.ConfigReloadedMsg:
		if msg.Err != nil {
			m.err = messages.ErrMsg(msg)
			return m, nil
		}
		if m.currentPage == layout.PageRules {
			return m, rules.FetchRules(m.client)
		}

	case messages.ConfigEditFinishedMsg:
		// 外部编辑器结束：失败沿用全局错误显示；成功热重载核心（随后刷新规则列表）
		// tea.ExecProcess 期间会 ReleaseTerminal（禁用鼠标），但 RestoreTerminal
		// 不恢复鼠标模式（Bubble Tea v1.3.10 缺陷），需显式重新启用，否则退出编辑器后鼠标失效。
		reenableMouse := func() tea.Msg { return tea.EnableMouseCellMotion() }
		if msg.Err != nil {
			m.err = messages.ErrMsg(msg)
			m.notice = ""
			m.noticeTicks = 0
			return m, reenableMouse
		}
		m.notice = i18n.T("rules.edited_toast")
		m.noticeTicks = autoRefreshNoticeTicks
		return m, tea.Batch(reenableMouse, reloadConfigCmd(m.client))

	case messages.SiteTestMsg:
		var cmd tea.Cmd
		m.connsState, cmd = m.connsState.CompleteSiteTest(msg)
		return m, cmd

	case messages.TestDoneMsg:
		var cmd tea.Cmd
		m.nodesState, cmd = m.nodesState.CompleteTest(msg)
		return m, cmd

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

	case messages.AutoRefreshTickMsg:
		m.syncProcessLogs()
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
			if !m.connectionReady {
				return m, autoRefreshTick()
			}
			return m, tea.Batch(autoRefreshTick(), fetchAutoRefresh(m.client, m.nodesState))
		}
		return m, autoRefreshTick()

	case messages.AutoRefreshMsg:
		m.err = nil
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
		m.nodesState = m.nodesState.ResetTesting()

	case messages.CoreActionDoneMsg:
		m.coreState, _ = m.coreState.Update(msg)
		m.notice = msg.Action + " " + i18n.T("settings.action.success")
		m.noticeTicks = autoRefreshNoticeTicks
		if msg.NeedReloadAll {
			cmds := []tea.Cmd{
				nodes.FetchGroups(m.client),
				nodes.FetchProxies(m.client),
				nodes.FetchConfigMode(m.client),
				rules.FetchRules(m.client),
				preferences.FetchMihomoVersion(m.client),
				preferences.FetchMihomoOverrides(m.configSvc),
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
		m.coreState, _ = m.coreState.Update(msg)
		m.err = messages.ErrMsg{Err: msg.Err}
		m.notice = ""
		m.noticeTicks = 0

	case messages.MihomoVersionMsg:
		m.settingsState = m.settingsState.ApplyMihomoVersion(msg.Version)
		m.coreState, _ = m.coreState.Update(msg)

	case messages.MihomoOverridesMsg:
		m.coreState, _ = m.coreState.Update(msg)

	case messages.MihomoOverridesSavedMsg:
		m.coreState, _ = m.coreState.Update(msg)
		if !msg.WriteOK {
			if msg.Err != nil {
				err := msg.Err
				if msg.Restored {
					err = fmt.Errorf("%s (%w)", i18n.T("settings.toast.mihomo_override_restored"), err)
				}
				m.err = messages.ErrMsg{Err: err}
			}
			m.notice, m.noticeTicks = "", 0
			return m, nil
		}
		m.err = nil
		m.notice = i18n.T(mihomoOverrideNotice(msg))
		m.noticeTicks = autoRefreshNoticeTicks
		var targetCmd tea.Cmd
		if !msg.Remote && (msg.Restarted || msg.EndpointChanged || msg.NeedsRestart && !msg.Applied) {
			targetCmd = m.syncAppliedCore(msg.Restarted, msg.NeedsRestart && msg.Applied)
		}
		if targetCmd == nil && msg.Applied && m.connectionReady {
			return m, m.refreshOnConnect()
		}
		return m, tea.Batch(targetCmd, m.coreState.Refresh())

	case messages.MihomoConfigMsg:
		// Mihomo 标签页配置加载完成（成功或失败）。
		// 同步热刷新 client/wsClient 端点（external-controller/secret 变更），
		// 并把 mixed-port 派生的代理地址同步到 connections。
		m.coreState, _ = m.coreState.Update(msg)
		if msg.Config != nil {
			cmd := m.updateCoreTarget(config.MihomoEndpoint{ExternalController: msg.Config.ExternalController, Secret: msg.Config.Secret, MixedPort: msg.Config.MixedPort}, coreTargetSync)
			m.err = nil
			return m, cmd
		}

	case messages.MihomoConfigSavedMsg:
		var targetCmd tea.Cmd
		// Mihomo 配置项已保存（写 YAML + 热重载）：toast 提示 + 重新拉取运行时配置。
		// external-controller/secret/mixed-port 变更后重新解析 endpoint 热刷新 client/wsClient。
		if msg.Err != nil && !msg.WriteOK {
			// YAML 写入就失败了（如路径不存在），显示硬错误
			m.err = messages.ErrMsg{Err: msg.Err}
		} else {
			// YAML 写入成功：即使 reload 失败也刷新端点（修改 external-controller 后旧端口不可达是预期行为）
			endpoint := config.ResolveMihomoEndpoint()
			targetCmd = m.updateCoreTarget(endpoint, coreTargetSync)
			if msg.Err != nil {
				// reload 失败但 YAML 已保存，提示用户配置已保存但需重启内核生效
				m.notice = i18n.T("settings.toast.mihomo_save_reload_warn")
			} else {
				m.notice = i18n.T("settings.toast.mihomo_save_success")
			}
			m.noticeTicks = autoRefreshNoticeTicks
			m.err = nil
		}
		return m, tea.Batch(targetCmd, m.fetchNodes(), preferences.FetchMihomoConfig(m.configSvc, m.client))

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
		cmd := m.finishSubConfigApply(msg.MergeErr, msg.Restarted, msg.NeedsRestart, msg.Remote,
			"sub.activated_toast", "sub.activated_merge_warn_toast")
		return m, cmd

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

	case messages.SubSourceCopiedMsg:
		m.notice = i18n.T("sub.copy_source_error")
		if msg.Success {
			m.notice = i18n.T("sub.copy_source_done")
		}
		m.noticeTicks = autoRefreshNoticeTicks

	case messages.SubFileLoadedMsg:
		m.subState, _ = m.subState.Update(msg)

	case messages.SubFileViewerClosedMsg:
		m.subState, _ = m.subState.Update(msg)
		return m, func() tea.Msg { return tea.EnableMouseCellMotion() }

	case messages.MergeEditFinishedMsg:
		// 外部编辑器结束：tea.ExecProcess 期间会 ReleaseTerminal（禁用鼠标），
		// 但 RestoreTerminal 不恢复鼠标模式（Bubble Tea v1.3.10 缺陷），需显式重新启用。
		reenableMouse := func() tea.Msg { return tea.EnableMouseCellMotion() }
		if msg.Err != nil {
			m.err = messages.ErrMsg{Err: msg.Err}
			m.notice = ""
			m.noticeTicks = 0
			return m, tea.Batch(reenableMouse, preferences.FetchMihomoOverrides(m.configSvc))
		}
		m.err = nil
		// 如果有激活的订阅，则立即应用这份全局覆写并热重载
		if m.subState.ActiveUID() != "" {
			m.notice = i18n.T("settings.toast.mihomo_override_applying")
			m.noticeTicks = autoRefreshNoticeTicks
			return m, tea.Batch(reenableMouse, sub.ApplyActiveMergeCmd(m.profileSvc, m.subState.ActiveUID(), msg.ClearedOverrides))
		}
		m.notice = i18n.T("sub.merge_saved_toast")
		m.noticeTicks = autoRefreshNoticeTicks
		return m, tea.Batch(reenableMouse, preferences.FetchMihomoOverrides(m.configSvc))

	case messages.SubMergeAppliedMsg:
		// 覆写编辑后触发的重载流程结束（仅激活订阅会走到此分支）。
		if msg.Err != nil {
			m.err = messages.ErrMsg{Err: msg.Err}
			m.notice = ""
			m.noticeTicks = 0
			return m, preferences.FetchMihomoOverrides(m.configSvc)
		}
		cmd := m.finishSubConfigApply(msg.MergeErr, msg.Restarted, msg.NeedsRestart, msg.Remote,
			"sub.merge_applied_toast", "sub.merge_applied_warn_toast")
		return m, tea.Batch(cmd, preferences.FetchMihomoOverrides(m.configSvc))

	case messages.LocalSubImportedMsg:
		// 首次启动自动导入本地订阅完成（成功时刷新列表，失败时静默忽略）
		if msg.Profile != nil && msg.Err == nil {
			return m, sub.FetchSubs(m.profileSvc)
		}
	default:
		return m, nil
	}
	return m, nil
}

func mihomoOverrideNotice(msg messages.MihomoOverridesSavedMsg) string {
	switch {
	case msg.Remote && !msg.Applied:
		return "settings.toast.mihomo_override_remote_saved_only"
	case msg.Remote && msg.NeedsRestart:
		return "settings.toast.mihomo_override_remote_restart"
	case msg.Remote:
		return "settings.toast.mihomo_override_remote_applied"
	case msg.Restarted:
		return "settings.toast.mihomo_override_restarted"
	case msg.NeedsRestart && msg.Applied:
		return "settings.toast.mihomo_override_needs_restart_partial"
	case msg.NeedsRestart:
		return "settings.toast.mihomo_override_needs_restart"
	case msg.Applied:
		return "settings.toast.mihomo_override_applied"
	default:
		return "settings.toast.mihomo_override_saved_offline"
	}
}

func (m *Model) finishSubConfigApply(mergeErr error, restarted, needsRestart, remote bool, successKey, warningKey string) tea.Cmd {
	m.err = nil
	m.noticeTicks = autoRefreshNoticeTicks
	if mergeErr != nil {
		m.notice = i18n.T(warningKey)
	} else if restarted || needsRestart || remote {
		m.notice = i18n.T(mihomoOverrideNotice(messages.MihomoOverridesSavedMsg{
			Applied: true, Restarted: restarted, NeedsRestart: needsRestart, Remote: remote,
		}))
	} else {
		m.notice = i18n.T(successKey)
	}
	if !remote {
		if cmd := m.syncAppliedCore(restarted, needsRestart); cmd != nil {
			return tea.Batch(cmd, sub.FetchSubs(m.profileSvc), m.coreState.Refresh())
		}
	}
	return m.refreshOnConnect()
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

func (m *Model) autoRefreshInterval() int {
	if m.config == nil {
		return 0
	}
	return m.config.AutoRefreshInterval
}

// fetchNodes 批量拉取节点信息（用于端点切换后刷新）。
func (m *Model) fetchNodes() tea.Cmd {
	return tea.Batch(
		nodes.FetchGroups(m.client),
		nodes.FetchProxies(m.client),
		nodes.FetchConfigMode(m.client),
	)
}

// refreshOnConnect 每次连接就绪进入界面时自动触发一次全量刷新，避免部分页面或状态栏信息未及时展示。
func (m *Model) refreshOnConnect() tea.Cmd {
	var cmds []tea.Cmd
	if m.client != nil {
		cmds = append(cmds,
			fetchAutoRefresh(m.client, m.nodesState),
			connections.FetchConnections(m.client),
			rules.FetchRules(m.client),
			preferences.FetchMihomoVersion(m.client),
		)
	}
	if m.profileSvc != nil {
		cmds = append(cmds, sub.FetchSubs(m.profileSvc))
	}
	if m.coreSvc != nil {
		cmds = append(cmds, m.coreState.Refresh())
	}
	return tea.Batch(cmds...)
}

func (m *Model) reloadStoredConfig() {
	cfg, err := config.Load()
	if err != nil {
		return
	}
	previousLanguage := ""
	previousTheme := ""
	if m.config != nil {
		previousLanguage = m.config.Language
		previousTheme = m.config.Theme
	}
	m.config = cfg
	m.settingsState.Config = cfg
	if cfg.Language != previousLanguage {
		i18n.SetLanguageOverride(cfg.Language)
		common.InitKeyBindings()
	}
	if cfg.Theme != "" && cfg.Theme != previousTheme {
		theme.SetTheme(cfg.Theme)
	}
	theme.SetBorderStyle(cfg.BorderStyle)
}
