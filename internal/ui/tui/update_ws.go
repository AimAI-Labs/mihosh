package tui

import (
	"time"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/preferences"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/nodes"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/rules"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/sub"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
)

// connRefreshInterval 连接刷新间隔
const connRefreshInterval = 1 * time.Second

const autoRefreshSyncedTicks = 1

// connTick 创建连接页面定时器
func connTick() tea.Cmd {
	return tea.Tick(connRefreshInterval, func(t time.Time) tea.Msg {
		return messages.ConnTickMsg(t)
	})
}

func (m *Model) ensureConnTick() tea.Cmd {
	if m.connTickActive {
		return nil
	}
	m.connTickActive = true
	return connTick()
}

// logsTick 创建日志页面定时器
func logsTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return messages.LogsTickMsg(t)
	})
}

func (m *Model) ensureLogsTick() tea.Cmd {
	if m.logsTickActive {
		return nil
	}
	m.logsTickActive = true
	return logsTick()
}

func autoRefreshTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return messages.AutoRefreshTickMsg(t)
	})
}

func subAutoUpdateTick() tea.Cmd {
	return tea.Tick(time.Minute, func(t time.Time) tea.Msg {
		return messages.SubAutoUpdateTickMsg(t)
	})
}

func (m Model) handleWSMessages(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.applyWSMessage(msg)
	if next.notice.ID != m.notice.ID {
		cmd = tea.Batch(cmd, next.notice.ExpiryCmd())
	}
	return next, cmd
}

func (m Model) applyWSMessage(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case messages.CoreHTTPMsg:
		if m.client == nil || msg.Generation != m.client.Generation() {
			return m, nil
		}
		return m.applyWSMessage(msg.Payload)

	case messages.CoreWSBatchMsg:
		if m.wsClient != nil && msg.Generation == m.wsClient.Generation() {
			m.applyStreamBatch(msg)
		}
		return m, listenWSMessages(m.wsCtx, m.wsInbox)
	}

	if next, cmd, handled := m.applyCoreEvent(msg); handled {
		return next, cmd
	}
	if next, cmd, handled := m.applyConfigEvent(msg); handled {
		return next, cmd
	}
	if next, cmd, handled := m.applySubEvent(msg); handled {
		return next, cmd
	}

	switch msg := msg.(type) {
	case messages.GroupsMsg:
		prevProxy, _, _ := m.nodesState.GetActiveProxyAndDelay()
		m.nodesState = m.nodesState.ApplyGroups(msg.Groups, msg.OrderedNames)
		nextProxy, _, _ := m.nodesState.GetActiveProxyAndDelay()
		if prevProxy != "" && nextProxy != "" && nextProxy != prevProxy {
			m.connsState = m.connsState.MarkCurrentIPStale()
		}

	case messages.ProxiesMsg:
		m.nodesState = m.nodesState.ApplyProxies(msg)

	case messages.ConfigModeMsg:
		prevMode := m.nodesState.Mode
		m.nodesState = m.nodesState.ApplyConfigMode(msg.Mode)
		if prevMode != "" && msg.Mode != "" && prevMode != msg.Mode {
			m.connsState = m.connsState.MarkCurrentIPStale()
		}

	case messages.ConnectionsMsg:
		m.connsState = m.connsState.ApplyConnections(msg.Resp)
		if msg.Resp != nil && m.chartData != nil {
			m.chartData.AddConnCountData(len(msg.Resp.Connections))
		}

	case messages.MemoryWSMsg:
		m.applyStreamBatch(messages.CoreWSBatchMsg{Memory: &msg})

	case messages.TrafficWSMsg:
		m.applyStreamBatch(messages.CoreWSBatchMsg{Traffic: &msg})

	case messages.ConnectionsWSMsg:
		m.applyStreamBatch(messages.CoreWSBatchMsg{Connections: &msg})

	case messages.LogsWSMsg:
		m.applyStreamLogs([]model.LogEntry{{Type: msg.LogType, Payload: msg.Payload, Timestamp: time.Now()}}, 0)

	case messages.LogIPResolvedMsg:
		m.logsState = m.logsState.ApplyIPResolved(msg.IP, msg.Resolved)

	case messages.RulesMsg:
		m.rulesState = m.rulesState.ApplyRules(msg)

	case messages.SiteTestMsg:
		var cmd tea.Cmd
		m.connsState, cmd = m.connsState.CompleteSiteTest(msg)
		return m, cmd

	case messages.UnlockTestMsg:
		var cmd tea.Cmd
		m.connsState, cmd = m.connsState.Update(msg)
		return m, cmd

	case messages.DNSQueryResultMsg:
		var cmd tea.Cmd
		m.connsState, cmd = m.connsState.Update(msg)
		return m, cmd

	case messages.TestDoneMsg:
		var cmd tea.Cmd
		m.nodesState, cmd = m.nodesState.CompleteTest(msg)
		return m, cmd

	case messages.IPInfoMsg:
		m.connsState = m.connsState.ApplyIPInfoMsg(msg)

	case messages.CurrentIPInfoMsg:
		var cmd tea.Cmd
		m.connsState, cmd = m.connsState.Update(msg)
		return m, cmd

	case messages.ConnectionClosedMsg:
		m.connsState = m.connsState.ApplyConnectionClosed()

	case messages.AllConnectionsClosedMsg:
		m.connsState = m.connsState.ApplyAllConnectionsClosed()

	case messages.FilteredConnectionsClosedMsg:
		m.connsState = m.connsState.ApplyAllConnectionsClosed()
		if msg.Failed > 0 {
			m.publishNotice(i18n.Tf("conns.close_matched_partial", msg.Closed, msg.Failed), messages.NoticeWarning)
		} else if msg.Closed > 0 {
			m.publishNotice(i18n.Tf("conns.close_matched_done", msg.Closed), messages.NoticeSuccess)
		}

	case messages.PlaceholderTickMsg:
		m.placeholderTickActive = false
		if !m.coreUnavailable() {
			return m, nil
		}
		m.placeholderState = m.placeholderState.Advance()
		return m, m.placeholderTickCmd()

	case messages.ConnTickMsg:
		m.connTickActive = false
		if m.currentPage == layout.PageConnections {
			var cmd tea.Cmd
			m.connsState, cmd = m.connsState.Update(msg)
			return m, tea.Batch(m.ensureConnTick(), cmd)
		}

	case messages.LogsTickMsg:
		m.logsTickActive = false
		if m.currentPage == layout.PageLogs {
			return m, m.ensureLogsTick()
		}

	case messages.AutoRefreshTickMsg:
		m.syncProcessLogs()
		interval := m.autoRefreshInterval()
		m.advanceAutoRefreshTransientState()
		if interval <= 0 {
			m.autoRefreshRemaining = 0
			m.autoRefreshSynced = false
			m.autoRefreshSyncedTTL = 0
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
		m.nodesState = m.nodesState.ApplyGroups(msg.Result.Groups, msg.Result.OrderedNames)
		m.nodesState = m.nodesState.ApplyProxies(msg.Result.Proxies)
		m.nodesState = m.nodesState.ApplyConfigMode(msg.Result.Mode)
		m.autoRefreshSynced = true
		m.autoRefreshSyncedTTL = autoRefreshSyncedTicks
		if msg.Changed {
			m.publishNotice(i18n.T("status.auto_refreshed"), messages.NoticeInfo)
			m.connsState = m.connsState.MarkCurrentIPStale()
			if m.currentPage == layout.PageConnections {
				var cmd tea.Cmd
				m.connsState, cmd = m.connsState.EnsureCurrentIP()
				return m, cmd
			}
		}

	case messages.ErrMsg:
		m.publishError(msg.Err)
		m.nodesState = m.nodesState.ResetTesting()

	case messages.NoticeMsg:
		m.notice.Publish(msg)

	case messages.NoticeExpiredMsg:
		m.notice.Expire(msg.ID)
	}
	return m, nil
}

func (m *Model) applyStreamBatch(msg messages.CoreWSBatchMsg) {
	if msg.Memory != nil && m.chartData != nil {
		m.chartData.AddMemoryData(msg.Memory.Memory)
	}
	if msg.Traffic != nil && m.chartData != nil {
		m.chartData.AddSpeedData(msg.Traffic.Up, msg.Traffic.Down)
	}
	if msg.Connections != nil {
		m.connsState = m.connsState.ApplyWSConnections(msg.Connections.Data)
		if m.chartData != nil {
			m.chartData.AddConnCountData(len(msg.Connections.Data.Connections))
		}
	}
	if len(msg.Logs) != 0 || msg.DroppedLogs != 0 {
		m.applyStreamLogs(msg.Logs, msg.DroppedLogs)
	}
}

func (m *Model) applyStreamLogs(entries []model.LogEntry, dropped uint64) {
	if !m.usesProcessLogs() {
		m.logsState = m.logsState.AppendLogs(entries, dropped)
	}
	if m.currentPage == layout.PageCore && m.coreState.IsMihomoPage() {
		pw, ph := m.getPageSize()
		m.coreState, _ = m.coreState.HandleEditorMsg(nil, pw, ph, m.logsState.GetSysLogs())
	}
}

func (m *Model) advanceAutoRefreshTransientState() {
	if m.autoRefreshSyncedTTL > 0 {
		m.autoRefreshSyncedTTL--
		if m.autoRefreshSyncedTTL == 0 {
			m.autoRefreshSynced = false
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
	cfg, err := service.Load()
	if err != nil {
		return
	}
	m.applyStoredConfig(cfg)
}

func (m *Model) applyStoredConfig(cfg *service.Config) {
	previousLanguage := ""
	previousTheme := ""
	if m.config != nil {
		previousLanguage = m.config.Language
		previousTheme = m.config.Theme
	}
	m.config = cfg
	m.settingsState.SetConfig(cfg)
	m.nodesState = m.nodesState.UpdateTestConfig(cfg.TestURL, cfg.Timeout)
	m.connsState = m.connsState.UpdateTimeout(cfg.Timeout)
	if cfg.Language != previousLanguage {
		i18n.SetLanguageOverride(cfg.Language)
		common.InitKeyBindings()
	}
	if cfg.Theme != "" && cfg.Theme != previousTheme {
		theme.SetTheme(cfg.Theme)
	}
	theme.SetBorderStyle(cfg.BorderStyle)
}
