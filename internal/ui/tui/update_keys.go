package tui

import (
	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/help"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleGlobalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.currentPage == layout.PageCore && msg.String() == "ctrl+c" {
		m.onAppQuit()
		return m, tea.Quit
	}
	// 引导弹窗拦截
	if m.guideState.Active {
		switch msg.String() {
		case "tab", "right", "down", "l", "j":
			m.guideState = m.guideState.NextButton()
			return m, nil
		case "shift+tab", "left", "up", "h", "k":
			m.guideState = m.guideState.PrevButton()
			return m, nil
		case "e":
			m.guideState = m.guideState.Dismiss()
			m.currentPage = layout.PageCore
			return m, m.onPageChange()
		case "r":
			return m, checkInitialConnectionCmd(m.client)
		case "esc", "q":
			m.guideState = m.guideState.Dismiss()
			return m, nil
		case "enter":
			switch m.guideState.FocusedButton {
			case 0:
				m.guideState = m.guideState.Dismiss()
				m.currentPage = layout.PageCore
				return m, m.onPageChange()
			case 1:
				return m, checkInitialConnectionCmd(m.client)
			case 2:
				m.guideState = m.guideState.Dismiss()
				return m, nil
			}
		}
		if key.Matches(msg, common.Keys.Quit) {
			m.onAppQuit()
			return m, tea.Quit
		}
		return m, nil
	}

	// 帮助弹窗拦截
	if m.showHelp {
		switch msg.String() {
		case "esc", "q", "?":
			m.showHelp = false
			m.helpState = help.State{}
			return m, nil
		case "up", "k", "down", "j", "pgup", "pgdown":
			delta := 1
			if msg.String() == "pgup" || msg.String() == "pgdown" {
				delta = max(1, m.height/2)
			}
			if msg.String() == "up" || msg.String() == "k" || msg.String() == "pgup" {
				delta = -delta
			}
			m.helpState = m.helpState.Scroll(delta, m.width, m.height, m.buildHelpContext())
			return m, nil
		}
		if key.Matches(msg, common.Keys.Quit) {
			m.onAppQuit()
			return m, tea.Quit
		}
		return m, nil
	}

	// 报错弹窗拦截
	if m.notice.DetailsOpen {
		switch msg.String() {
		case "esc", "enter", "q":
			m.notice.Clear()
			return m, nil
		}
		if key.Matches(msg, common.Keys.Quit) {
			m.onAppQuit()
			return m, tea.Quit
		}
		return m, nil
	}

	// 更新弹窗拦截
	if m.showUpdateDialog {
		switch msg.String() {
		case "esc", "q":
			m.showUpdateDialog = false
			m.updateError = nil
			return m, nil
		case "enter":
			if !m.isUpdating && m.updateInfo != nil {
				m.isUpdating = true
				m.updateError = nil
				url := m.updateInfo.DownloadURL
				return m, func() tea.Msg {
					err := service.DownloadAndApplyUpdate(url)
					return messages.UpdateAppliedMsg{Err: err}
				}
			}
			return m, nil
		}
		if key.Matches(msg, common.Keys.Quit) {
			m.onAppQuit()
			return m, tea.Quit
		}
		return m, nil
	}

	capture := m.pageInputCapture()
	if msg.String() == "?" && !capture.Help {
		m.helpState = help.State{}
		m.showHelp = true
		return m, nil
	}
	if capture.Active && (msg.String() != "ctrl+c" || capture.Interrupt) {
		return m.dispatchToPage(msg)
	}

	// 全局快捷键
	switch {
	case key.Matches(msg, common.Keys.Quit):
		m.onAppQuit()
		return m, tea.Quit

	case key.Matches(msg, common.Keys.NextPage):
		m.onPageLeave()
		m.currentPage = (m.currentPage + 1) % layout.PageCount
		return m, m.onPageChange()

	case key.Matches(msg, common.Keys.PrevPage):
		m.onPageLeave()
		m.currentPage = (m.currentPage + layout.PageCount - 1) % layout.PageCount
		return m, m.onPageChange()

	case key.Matches(msg, common.Keys.Refresh):
		cmd := m.refreshCurrentPage()
		return m, cmd

	case msg.String() == "ctrl+u" && m.hasUpdate:
		m.showUpdateDialog = true
		return m, nil
	}

	// 分发到页面子状态
	return m.dispatchToPage(msg)
}

// dispatchToPage 将消息分发到当前页面子状态
func (m Model) dispatchToPage(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.currentPage {
	case layout.PageCore:
		m.coreState, cmd = m.coreState.Update(msg)
	case layout.PageNodes:
		m.nodesState, cmd = m.nodesState.Update(msg)

	case layout.PageConnections:
		m.connsState, cmd = m.connsState.Update(msg)

	case layout.PageLogs:
		m.logsState, cmd = m.logsState.Update(msg)

	case layout.PageRules:
		m.rulesState, cmd = m.rulesState.Update(msg)

	case layout.PageSub:
		pw, ph := m.getPageSize()
		m.subState, _ = m.subState.Update(messages.PageResizeMsg{Width: pw, Height: ph})
		m.subState, cmd = m.subState.Update(msg)

	case layout.PageSettings:
		var newCfg = m.config
		oldLanguage := ""
		if m.config != nil {
			oldLanguage = m.config.Language
		}
		m.settingsState.Config = m.config
		m.settingsState, cmd = m.settingsState.Update(msg)
		newCfg = m.settingsState.Config
		m.config = newCfg
		if newCfg != nil && newCfg.Language != oldLanguage {
			i18n.SetLanguageOverride(newCfg.Language)
			common.InitKeyBindings()
			cmd = tea.Batch(cmd, tea.ClearScreen)
		}
		if newCfg != nil {
			m.autoRefreshRemaining = newCfg.AutoRefreshInterval
			m.autoRefreshSynced = false
			m.autoRefreshSyncedTTL = 0
		}
	}
	return m, cmd
}

// pageInputCapture asks the page to own its mode-specific input rules.
func (m *Model) pageInputCapture() common.InputCapture {
	switch m.currentPage {
	case layout.PageCore:
		return m.coreState.InputCapture()
	case layout.PageNodes:
		return m.nodesState.InputCapture()
	case layout.PageConnections:
		return m.connsState.InputCapture()
	case layout.PageLogs:
		return m.logsState.InputCapture()
	case layout.PageRules:
		return m.rulesState.InputCapture()
	case layout.PageSub:
		return m.subState.InputCapture()
	case layout.PageSettings:
		return m.settingsState.InputCapture()
	}
	return common.InputCapture{}
}
