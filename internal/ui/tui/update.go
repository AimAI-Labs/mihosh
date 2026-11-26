package tui

import (
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/preferences"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/rules"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/sub"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"

	tea "github.com/charmbracelet/bubbletea"
)

// checkInitialConnectionCmd 发起初始 Mihomo API 健康检测
func checkInitialConnectionCmd(client *service.Client) tea.Cmd {
	if client == nil {
		return nil
	}
	return messages.CoreRequest(client, func(client *service.Client) tea.Msg {
		if _, err := client.GetVersion(); err != nil {
			endpoint := client.EndpointSnapshot()
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
	})
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
		subAutoUpdateTick(),
		listenWSMessages(m.wsCtx, m.wsInbox),
		func() tea.Msg {
			info, err := service.CheckUpdate(model.Version)
			if err != nil || info == nil {
				return messages.UpdateCheckedMsg{}
			}
			return messages.UpdateCheckedMsg{Info: info}
		},
		tea.Sequence(func() tea.Msg {
			if err := service.PreserveSubscriptionUsage(); err != nil {
				return messages.ErrMsg{Err: err}
			}
			return nil
		}, sub.RefreshSubs(m.profileSvc), func() tea.Msg {
			p, err := m.profileSvc.AutoImportLocalSub()
			return messages.LocalSubImportedMsg{Profile: p, Err: err}
		}, sub.AutoUpdateDueSubsCmd(m.profileSvc)),
	)
}

// Close also covers terminal errors and signal-driven exits from Bubble Tea.
func (m Model) Close() {
	if m.wsCancel != nil {
		m.wsCancel()
	}
	if m.wsClient != nil {
		m.wsClient.Stop()
	}
	m.connsState.CancelTest()
	m.coreState.Close()
	if m.coreSvc != nil {
		m.coreSvc.Close()
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
		width, height := m.getPageSize()
		m.connsState, _ = m.connsState.Update(messages.PageResizeMsg{Width: width, Height: height})
		var ipCmd tea.Cmd
		m.connsState, ipCmd = m.connsState.EnsureCurrentIP()
		if cmd != nil {
			m.connsState = m.connsState.ResetPrevConnIDs()
			cmd = tea.Batch(cmd, connTick())
		} else if ipCmd != nil {
			cmd = connTick()
		}
		cmd = tea.Batch(cmd, ipCmd)
	case layout.PageLogs:
		cmd = tea.Batch(cmd, logsTick())
	case layout.PageSub:
		width, height := m.getPageSize()
		m.subState, _ = m.subState.Update(messages.PageResizeMsg{Width: width, Height: height})
	}
	return tea.Batch(cmd, m.placeholderTickCmd())
}

func (m *Model) refreshMenu() tea.Cmd {
	if m.connectionChecked && !m.connectionReady && m.currentPage != layout.PageCore && m.currentPage != layout.PageSub && m.currentPage != layout.PageSettings && m.currentPage != layout.PageLogs {
		return m.checkCoreConnection(true)
	}
	switch m.currentPage {
	case layout.PageCore:
		if m.connectionChecked && !m.connectionReady {
			return tea.Batch(m.coreState.Refresh(), m.checkCoreConnection(true))
		}
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
	m.Close()
}

// refreshCurrentPage 刷新当前页面
func (m *Model) refreshCurrentPage() tea.Cmd {
	if m.coreUnavailable() {
		// 占位屏上刷新即重新检测内核，连接成功后占位屏会自动消失。
		return tea.Batch(m.checkCoreConnection(true), m.placeholderTickCmd())
	}
	switch m.currentPage {
	case layout.PageCore:
		var cmd tea.Cmd
		m.coreState, cmd = m.coreState.RefreshCurrent()
		return tea.Batch(cmd, m.requestCoreTarget(coreTargetSync, coreTargetOptions{probe: true}))
	case layout.PageConnections:
		cmd := m.refreshMenu()
		if m.connsState.ViewMode() == connections.ConnViewTraffic {
			var ipCmd tea.Cmd
			m.connsState, ipCmd = m.connsState.RefreshCurrentIP()
			return tea.Batch(cmd, ipCmd)
		}
		return cmd
	}
	return m.refreshMenu()
}

func (m Model) topNavActive() bool {
	return false
}

func (m *Model) checkCoreConnection(keepPage bool) tea.Cmd {
	if m.client == nil || m.connectionChecking {
		return nil
	}
	m.connectionChecking = true
	epoch := m.connectionEpoch
	client := m.client.Snapshot()
	return func() tea.Msg {
		version, err := client.GetVersion()
		if err != nil {
			return messages.CoreConnectionMsg{Epoch: epoch, Err: err, KeepPage: keepPage}
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
