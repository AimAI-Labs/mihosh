package tui

import (
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/nodes"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/rules"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/settings"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/sub"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"

	tea "github.com/charmbracelet/bubbletea"
)

// Init 初始化
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		nodes.FetchGroups(m.client),
		nodes.FetchProxies(m.client),
		nodes.FetchConfigMode(m.client),
		autoRefreshTick(),
		startWSStreams(m.wsClient, m.wsMsgChan),
		listenWSMessages(m.wsCtx, m.wsMsgChan),
		// 自动更新检测
		func() tea.Msg {
			info, err := service.CheckUpdate(model.Version)
			if err != nil || info == nil {
				return messages.UpdateCheckedMsg{Info: nil}
			}
			return messages.UpdateCheckedMsg{Info: info}
		},
		// 首次启动时自动将本地 mihomo 配置导入为本地订阅（幂等，已有则跳过）
		func() tea.Msg {
			p, err := m.profileSvc.AutoImportLocalSub()
			return messages.LocalSubImportedMsg{Profile: p, Err: err}
		},
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		pw, ph := m.getPageSize()
		m.logsState = m.logsState.UpdateMaxHScrollOffset(pw, ph)
		m.settingsState = m.settingsState.SyncSysStatus(pw, ph, m.config, m.logsState.GetSysLogs())
		return m, tea.ClearScreen

	case tea.MouseMsg:
		return m.handleGlobalMouse(msg)

	case tea.KeyMsg:
		return m.handleGlobalKeys(msg)

	default:
		return m.handleWSMessages(msg)
	}
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
		pw, ph := m.getPageSize()
		m.settingsState = m.settingsState.SyncSysStatus(pw, ph, m.config, m.logsState.GetSysLogs())
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

func (m Model) topNavActive() bool {
	return false
}
