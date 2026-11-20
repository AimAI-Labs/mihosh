package tui

import (
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/preferences"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/nodes"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/rules"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/sub"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"

	tea "github.com/charmbracelet/bubbletea"
)

// checkInitialConnectionCmd 发起初始 Mihomo API 健康检测
func checkInitialConnectionCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		if _, err := client.GetVersion(); err != nil {
			endpoint := config.ResolveMihomoEndpoint()
			secretMasked := ""
			if endpoint.Secret != "" {
				secretMasked = "******"
			}
			return messages.ShowGuideModalMsg{
				Endpoint:     endpoint.ExternalController,
				SecretMasked: secretMasked,
				ErrMessage:   err.Error(),
			}
		}
		return messages.HideGuideModalMsg{}
	}
}

// Init 初始化
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.coreState.Refresh(),
		m.listenCoreOwner(),
		func() tea.Msg {
			if m.config.Core.Mode == "managed" {
				if err := m.coreSvc.Start(m.wsCtx); err != nil {
					return messages.CoreConnectionMsg{Epoch: m.connectionEpoch, Err: err}
				}
				return messages.ManagedCoreStartedMsg{Epoch: m.connectionEpoch}
			}
			version, err := m.client.GetVersion()
			if err != nil {
				return messages.CoreConnectionMsg{Epoch: m.connectionEpoch, Err: err}
			}
			return messages.CoreConnectionMsg{Epoch: m.connectionEpoch, Version: version.Version}
		},
		autoRefreshTick(),
		listenWSMessages(m.wsCtx, m.wsMsgChan),
		func() tea.Msg {
			info, err := service.CheckUpdate(model.Version)
			if err != nil || info == nil {
				return messages.UpdateCheckedMsg{}
			}
			return messages.UpdateCheckedMsg{Info: info}
		},
		tea.Sequence(sub.RefreshSubs(m.profileSvc), func() tea.Msg {
			p, err := m.profileSvc.AutoImportLocalSub()
			return messages.LocalSubImportedMsg{Profile: p, Err: err}
		}),
	)
}

// Close also covers terminal errors and signal-driven exits from Bubble Tea.
func (m Model) Close() {
	m.coreState.Close()
	if m.coreSvc != nil {
		m.coreSvc.Close()
	}
	if m.wsCancel != nil {
		m.wsCancel()
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		pw, ph := m.getPageSize()
		m.coreState, _ = m.coreState.HandleEditorMsg(nil, pw, ph, m.logsState.GetSysLogs())
		resizeMsg := messages.PageResizeMsg{Width: pw, Height: ph}
		if m.currentPage != layout.PageCore {
			m.coreState, _ = m.coreState.Update(resizeMsg)
		}
		newModel, cmd := m.dispatchToPage(resizeMsg)
		m = newModel.(Model)
		return m, tea.Batch(tea.ClearScreen, cmd)

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
	cmd := m.refreshMenu()
	switch m.currentPage {
	case layout.PageConnections:
		if cmd != nil {
			m.connsState = m.connsState.ResetPrevConnIDs()
			return tea.Batch(cmd, connTick())
		}
	case layout.PageLogs:
		return logsTick()
	}
	return cmd
}

func (m *Model) refreshMenu() tea.Cmd {
	if m.connectionChecked && !m.connectionReady && m.currentPage != layout.PageCore && m.currentPage != layout.PageSub && m.currentPage != layout.PageSettings && m.currentPage != layout.PageLogs {
		return nil
	}
	switch m.currentPage {
	case layout.PageCore:
		return m.coreState.Refresh()
	case layout.PageNodes:
		return m.fetchNodes()
	case layout.PageConnections:
		return connections.FetchConnections(m.client)
	case layout.PageLogs:
		m.syncProcessLogs()
	case layout.PageRules:
		return rules.FetchRules(m.client)
	case layout.PageSub:
		return sub.RefreshSubs(m.profileSvc)
	case layout.PageSettings:
		if m.configSvc != nil {
			m.reloadStoredConfig()
		}
		return preferences.FetchMihomoVersion(m.client)
	}
	return nil
}

// onPageLeave 页面离开处理
func (m *Model) onPageLeave() {
	switch m.currentPage {
	case layout.PageCore:
		m.coreState = m.coreState.Leave()
	case layout.PageConnections:
		m.connsState.CancelTest()
	}
}

// onAppQuit 应用退出前清理
func (m *Model) onAppQuit() {
	m.coreState.Close()
	if m.coreSvc != nil {
		m.coreSvc.Close()
	}
	m.connsState.CancelTest()
	if m.wsCancel != nil {
		m.wsCancel()
	}
}

// refreshCurrentPage 刷新当前页面
func (m *Model) refreshCurrentPage() tea.Cmd {
	switch m.currentPage {
	case layout.PageCore:
		var cmd tea.Cmd
		m.coreState, cmd = m.coreState.RefreshCurrent()
		check := m.updateCoreTarget(config.ResolveMihomoEndpoint(), coreTargetSync)
		if check == nil {
			check = m.checkCoreConnection()
		}
		return tea.Batch(cmd, check)
	case layout.PageNodes:
		return tea.Batch(nodes.FetchGroups(m.client), nodes.FetchProxies(m.client))
	case layout.PageRules:
		return rules.FetchRules(m.client)
	case layout.PageSub:
		return sub.RefreshSubs(m.profileSvc)
	case layout.PageSettings:
		cfg, _ := m.configSvc.LoadConfig()
		m.config = cfg
		return preferences.FetchMihomoVersion(m.client)
	}
	return nil
}

func (m Model) topNavActive() bool {
	return false
}

func (m *Model) checkCoreConnection() tea.Cmd {
	epoch := m.connectionEpoch
	client := m.client
	if client != nil {
		client = client.Snapshot()
	}
	return func() tea.Msg {
		version, err := client.GetVersion()
		if err != nil {
			return messages.CoreConnectionMsg{Epoch: epoch, Err: err}
		}
		return messages.CoreConnectionMsg{Epoch: epoch, Version: version.Version}
	}
}

func (m *Model) listenCoreOwner() tea.Cmd {
	ctx := m.wsCtx
	events := m.coreSvc.Events()
	return func() tea.Msg {
		select {
		case <-ctx.Done():
			return nil
		case action := <-events:
			return messages.CoreOwnerChangedMsg{Action: action}
		}
	}
}
