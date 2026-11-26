package tui

import (
	"fmt"
	"github.com/AimAI-Labs/mihosh/internal/app/service"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/preferences"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/nodes"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/rules"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
)

// reloadConfigCmd 通知 mihomo 核心重新加载配置文件。
// 失败回退为 ErrMsg（沿用现有全局错误显示），不回滚已写入的文件。
func reloadConfigCmd(client interface {
	ReloadConfig(path string) error
}) tea.Cmd {
	if client == nil {
		return nil
	}
	return func() tea.Msg {
		path, err := service.GetMihomoConfigPath()
		if err != nil {
			return messages.ConfigReloadedMsg{Err: err}
		}
		if err := client.ReloadConfig(path); err != nil {
			return messages.ConfigReloadedMsg{Err: err}
		}
		return messages.ConfigReloadedMsg{}
	}
}

func (m Model) applyConfigEvent(msg tea.Msg) (Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case messages.SyncDoneMsg:
		m.settingsState = m.settingsState.CompleteSync(msg)
		if msg.Err != nil {
			m.publishError(msg.Err)
			return m, nil, true
		}
		m.clearErrorNotice()
		if msg.Reloaded {
			if cfg, err := service.Load(); err == nil && cfg != nil {
				m.applyStoredConfig(cfg)
				m.autoRefreshRemaining = cfg.AutoRefreshInterval
				m.subState = m.subState.ApplySubs(cfg.Subs, cfg.ActiveSub)
			}
			m.nodesState = m.nodesState.ReloadChains()
		}
		if msg.Summary != "" {
			m.publishNotice(msg.Summary, messages.NoticeSuccess)
		}
		return m, nil, true

	case messages.ProxyChainAppliedMsg:
		m.nodesState = m.nodesState.ApplyChainResult(msg)
		if msg.Err != nil {
			m.publishError(msg.Err)
			return m, nil, true
		}
		m.clearErrorNotice()
		var noticeText string
		switch {
		case msg.Remote || msg.NeedsRestart || msg.Restarted:
			noticeText = i18n.T(mihomoOverrideNotice(messages.MihomoOverridesSavedMsg{
				Applied: msg.Applied, Remote: msg.Remote, NeedsRestart: msg.NeedsRestart, Restarted: msg.Restarted,
			}))
		case !msg.Applied:
			noticeText = i18n.T("nodes.chain.saved_pending")
		case msg.Cleared:
			noticeText = i18n.Tf("nodes.chain.cleared_toast", msg.Group)
		default:
			noticeText = i18n.Tf("nodes.chain.applied_toast", msg.Group, msg.ExitNode)
		}
		m.publishNotice(noticeText, overrideNoticeLevel(msg.Applied, msg.NeedsRestart))
		var targetCmd tea.Cmd
		if !msg.Remote && (msg.Restarted || msg.EndpointChanged) {
			targetCmd = m.syncAppliedCore(msg.Restarted, msg.NeedsRestart && msg.Applied, false)
		}
		if msg.Applied && !msg.Restarted && m.client != nil {
			return m, tea.Batch(targetCmd, nodes.FetchGroups(m.client), nodes.FetchProxies(m.client)), true
		}
		return m, targetCmd, true

	case messages.RuleEditAppliedMsg:
		if msg.Err != nil {
			m.publishError(msg.Err)
			return m, nil, true
		}
		m.clearErrorNotice()
		notice := msg.NoticeKey
		if msg.Remote || msg.NeedsRestart || msg.Restarted {
			notice = mihomoOverrideNotice(messages.MihomoOverridesSavedMsg{
				Applied: msg.Applied, Remote: msg.Remote, NeedsRestart: msg.NeedsRestart, Restarted: msg.Restarted,
			})
		} else if !msg.Applied {
			notice = "rules.write.saved"
		}
		m.publishNotice(i18n.T(notice), overrideNoticeLevel(msg.Applied, msg.NeedsRestart))
		var targetCmd tea.Cmd
		if !msg.Remote && (msg.Restarted || msg.EndpointChanged) {
			targetCmd = m.syncAppliedCore(msg.Restarted, msg.NeedsRestart && msg.Applied, false)
		}
		if msg.Applied && !msg.Restarted && m.client != nil {
			return m, tea.Batch(targetCmd, rules.FetchRules(m.client)), true
		}
		return m, targetCmd, true

	case messages.ConfigReloadedMsg:
		if msg.Err != nil {
			m.publishError(msg.Err)
			return m, nil, true
		}
		if m.currentPage == layout.PageRules {
			return m, rules.FetchRules(m.client), true
		}
		return m, nil, true

	case messages.ConfigEditFinishedMsg:
		// 外部编辑器结束：失败沿用全局错误显示；成功热重载核心（随后刷新规则列表）
		// tea.ExecProcess 期间会 ReleaseTerminal（禁用鼠标），但 RestoreTerminal
		// 不恢复鼠标模式（Bubble Tea v1.3.10 缺陷），需显式重新启用，否则退出编辑器后鼠标失效。
		reenableMouse := func() tea.Msg { return tea.EnableMouseCellMotion() }
		if msg.Err != nil {
			m.publishError(msg.Err)
			return m, reenableMouse, true
		}
		m.publishNotice(i18n.T("rules.edited_toast"), messages.NoticeSuccess)
		return m, tea.Batch(reenableMouse, reloadConfigCmd(m.client)), true

	case messages.MihomoVersionMsg:
		m.settingsState = m.settingsState.ApplyMihomoVersion(msg.Version)
		m.coreState, _ = m.coreState.Update(msg)
		return m, nil, true

	case messages.MihomoOverridesMsg:
		m.coreState, _ = m.coreState.Update(msg)
		return m, nil, true

	case messages.MihomoOverridesSavedMsg:
		m.coreState, _ = m.coreState.Update(msg)
		if !msg.WriteOK {
			if msg.Err != nil {
				err := msg.Err
				if msg.Restored {
					err = fmt.Errorf("%s (%w)", i18n.T("settings.toast.mihomo_override_restored"), err)
				}
				m.publishError(err)
			}
			return m, nil, true
		}
		m.clearErrorNotice()
		m.publishNotice(i18n.T(mihomoOverrideNotice(msg)), overrideNoticeLevel(msg.Applied, msg.NeedsRestart))
		var targetCmd tea.Cmd
		if !msg.Remote && (msg.Restarted || msg.EndpointChanged || msg.NeedsRestart && !msg.Applied) {
			targetCmd = m.syncAppliedCore(msg.Restarted, msg.NeedsRestart && msg.Applied, msg.Applied && m.connectionReady)
		}
		if targetCmd == nil && msg.Applied && m.connectionReady {
			return m, m.refreshOnConnect(), true
		}
		return m, tea.Batch(targetCmd, m.coreState.Refresh()), true

	case messages.MihomoConfigMsg:
		m.coreState, _ = m.coreState.Update(msg)
		if msg.Target != nil && m.targetPending == nil {
			cmd := m.updateCoreTarget(*msg.Target, coreTargetSync)
			return m, cmd, true
		}
		return m, nil, true

	case messages.MihomoConfigSavedMsg:
		if msg.Err != nil && !msg.WriteOK {
			m.publishError(msg.Err)
			return m, tea.Batch(m.fetchNodes(), preferences.FetchMihomoConfig(m.configSvc, m.client)), true
		}
		m.clearErrorNotice()
		if msg.Err != nil {
			m.publishNotice(i18n.T("settings.toast.mihomo_save_reload_warn"), messages.NoticeWarning)
		} else {
			m.publishNotice(i18n.T("settings.toast.mihomo_save_success"), messages.NoticeSuccess)
		}
		cmd := m.requestCoreTarget(coreTargetSync, coreTargetOptions{refreshConfig: true})
		return m, cmd, true

	case messages.ThemeChangedMsg:
		// 主题已切换，触发重绘（View 会读取新主题色）
		return m, tea.ClearScreen, true
	}

	return m, nil, false
}

func overrideNoticeLevel(applied, needsRestart bool) messages.NoticeLevel {
	if needsRestart || !applied {
		return messages.NoticeWarning
	}
	return messages.NoticeSuccess
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
