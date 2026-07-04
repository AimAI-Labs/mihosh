package tui

import (
	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleGlobalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		return m.dispatchToPage(msg)
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

	case msg.String() == "ctrl+u" && m.hasUpdate:
		m.showUpdateDialog = true
		return m, nil
	}

	// 分发到页面子状态
	return m.dispatchToPage(msg)

	// ── 数据消息：分发到子状态 ──

	return m, nil
}

// dispatchToPage 将消息分发到当前页面子状态
func (m Model) dispatchToPage(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.currentPage {
	case layout.PageNodes:
		m.nodesState, cmd = m.nodesState.Update(msg)

	case layout.PageConnections:
		m.connsState, cmd = m.connsState.Update(msg)

	case layout.PageLogs:
		m.logsState, cmd = m.logsState.Update(msg)

	case layout.PageRules:
		m.rulesState, cmd = m.rulesState.Update(msg)

	case layout.PageSub:
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
		pw, ph := m.getPageSize()
		m.settingsState = m.settingsState.SyncSysStatus(pw, ph, newCfg, m.logsState.GetSysLogs())
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
