package tui

import (
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/preferences"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/sub"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) applySubEvent(msg tea.Msg) (Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case messages.SubsLoadedMsg:
		m.subState = m.subState.ApplySubs(msg.Subs, msg.Active)
		return m, nil, true

	case messages.SubAddDoneMsg:
		// 新增成功：刷新列表 + 提示，并自动触发首次更新
		m.publishNotice(i18n.T("sub.added_toast"), messages.NoticeSuccess)
		m.subState = m.subState.SetUpdating(msg.UID)
		return m, tea.Sequence(
			sub.FetchSubs(m.profileSvc),
			sub.UpdateSubCmd(m.profileSvc, msg.UID),
		), true

	case messages.SubAddErrorMsg:
		m.publishError(msg.Err)
		return m, nil, true

	case messages.SubDeletedMsg:
		m.publishNotice(i18n.T("sub.deleted_toast"), messages.NoticeSuccess)
		return m, sub.FetchSubs(m.profileSvc), true

	case messages.SubDeleteErrorMsg:
		m.publishError(msg.Err)
		return m, nil, true

	case messages.SubFetchDoneMsg:
		m.subState = m.subState.ClearUpdating()
		m.publishNotice(i18n.T("sub.updated_toast"), messages.NoticeSuccess)
		return m, sub.FetchSubs(m.profileSvc), true

	case messages.SubFetchErrorMsg:
		m.publishError(msg.Err)
		m.subState = m.subState.ClearUpdating()
		return m, nil, true

	case messages.SubAutoUpdateTickMsg:
		if m.subState.BatchUpdating() {
			return m, subAutoUpdateTick(), true
		}
		m.subState = m.subState.SetBatchUpdating()
		return m, tea.Batch(subAutoUpdateTick(), sub.AutoUpdateDueSubsCmd(m.profileSvc)), true

	case messages.SubBatchFetchDoneMsg:
		m.subState = m.subState.ClearBatchUpdating()
		if msg.Err != nil {
			if !msg.Auto {
				m.publishError(msg.Err)
			}
			return m, nil, true
		}
		if msg.Total == 0 {
			if !msg.Auto {
				m.publishNotice(i18n.T("sub.batch_empty"), messages.NoticeInfo)
			}
			return m, nil, true
		}
		if msg.Auto {
			if msg.Failed > 0 {
				m.publishNotice(i18n.Tf("sub.auto_partial", msg.Updated, msg.Total, msg.Failed), messages.NoticeWarning)
			} else if msg.Updated > 0 {
				m.publishNotice(i18n.Tf("sub.auto_updated", msg.Updated), messages.NoticeSuccess)
			}
		} else {
			if msg.Failed > 0 {
				m.publishNotice(i18n.Tf("sub.batch_partial", msg.Updated, msg.Total, msg.Failed), messages.NoticeWarning)
			} else {
				m.publishNotice(i18n.Tf("sub.batch_done", msg.Updated), messages.NoticeSuccess)
			}
		}
		return m, sub.FetchSubs(m.profileSvc), true

	case messages.SubActivatedMsg:
		if msg.Err != nil {
			m.publishError(msg.Err)
			return m, nil, true
		}
		cmd := m.finishSubConfigApply(msg.MergeErr, msg.Restarted, msg.NeedsRestart, msg.Remote,
			"sub.activated_toast", "sub.activated_merge_warn_toast")
		return m, cmd, true

	case messages.SubMergeSavedMsg:
		m.publishNotice(i18n.T("sub.merge_saved_toast"), messages.NoticeSuccess)
		return m, nil, true

	case messages.SubMergeSaveErrorMsg:
		m.publishError(msg.Err)
		return m, nil, true

	case messages.SubEditDoneMsg:
		m.publishNotice(i18n.T("sub.edited_toast"), messages.NoticeSuccess)
		return m, sub.FetchSubs(m.profileSvc), true

	case messages.SubEditErrorMsg:
		m.publishError(msg.Err)
		return m, nil, true

	case messages.SubSourceCopiedMsg:
		if msg.Success {
			m.publishNotice(i18n.T("sub.copy_source_done"), messages.NoticeSuccess)
		} else {
			m.publishNotice(i18n.T("sub.copy_source_error"), messages.NoticeError)
		}
		return m, nil, true

	case messages.SubFileLoadedMsg:
		m.subState, _ = m.subState.Update(msg)
		return m, nil, true

	case messages.SubFileViewerClosedMsg:
		m.subState, _ = m.subState.Update(msg)
		return m, func() tea.Msg { return tea.EnableMouseCellMotion() }, true

	case messages.SubHistoryLoadedMsg, messages.SubHistoryDiffMsg:
		m.subState, _ = m.subState.Update(msg)
		return m, nil, true

	case messages.SubRollbackDoneMsg:
		if msg.Err != nil {
			m.publishError(msg.Err)
			return m, nil, true
		}
		if msg.MergeErr != nil {
			m.publishNotice(i18n.Tf("sub.history.rollback_warn", msg.SnapshotID), messages.NoticeWarning)
		} else {
			m.publishNotice(i18n.Tf("sub.history.rollback_done", msg.SnapshotID), messages.NoticeSuccess)
		}
		cmds := []tea.Cmd{sub.FetchSubs(m.profileSvc), preferences.FetchMihomoOverrides(m.configSvc)}
		if msg.Restarted {
			m.reloadStoredConfig()
			cmds = append(cmds, m.updateCoreTarget(config.ResolveMihomoEndpoint(), coreTargetReconnect))
		} else if m.connectionReady {
			cmds = append(cmds, m.refreshOnConnect())
		}
		return m, tea.Batch(cmds...), true

	case messages.MergeEditFinishedMsg:
		// 外部编辑器结束：tea.ExecProcess 期间会 ReleaseTerminal（禁用鼠标），
		// 但 RestoreTerminal 不恢复鼠标模式（Bubble Tea v1.3.10 缺陷），需显式重新启用。
		reenableMouse := func() tea.Msg { return tea.EnableMouseCellMotion() }
		if msg.Err != nil {
			m.publishError(msg.Err)
			return m, tea.Batch(reenableMouse, preferences.FetchMihomoOverrides(m.configSvc)), true
		}
		m.clearErrorNotice()
		// 如果有激活的订阅，则立即应用这份全局覆写并热重载
		if m.subState.ActiveUID() != "" {
			m.publishNotice(i18n.T("settings.toast.mihomo_override_applying"), messages.NoticeInfo)
			return m, tea.Batch(reenableMouse, sub.ApplyActiveMergeCmd(m.profileSvc, m.subState.ActiveUID(), msg.ClearedOverrides)), true
		}
		m.publishNotice(i18n.T("sub.merge_saved_toast"), messages.NoticeSuccess)
		return m, tea.Batch(reenableMouse, preferences.FetchMihomoOverrides(m.configSvc)), true

	case messages.SubMergeAppliedMsg:
		// 覆写编辑后触发的重载流程结束（仅激活订阅会走到此分支）。
		if msg.Err != nil {
			m.publishError(msg.Err)
			return m, preferences.FetchMihomoOverrides(m.configSvc), true
		}
		cmd := m.finishSubConfigApply(msg.MergeErr, msg.Restarted, msg.NeedsRestart, msg.Remote,
			"sub.merge_applied_toast", "sub.merge_applied_warn_toast")
		return m, tea.Batch(cmd, preferences.FetchMihomoOverrides(m.configSvc)), true

	case messages.LocalSubImportedMsg:
		// 首次启动自动导入本地订阅完成（成功时刷新列表，失败时静默忽略）
		if msg.Profile != nil && msg.Err == nil {
			return m, sub.FetchSubs(m.profileSvc), true
		}
		return m, nil, true
	}

	return m, nil, false
}

func (m *Model) finishSubConfigApply(mergeErr error, restarted, needsRestart, remote bool, successKey, warningKey string) tea.Cmd {
	m.clearErrorNotice()
	if mergeErr != nil {
		m.publishNotice(i18n.T(warningKey), messages.NoticeWarning)
	} else if restarted || needsRestart || remote {
		result := messages.MihomoOverridesSavedMsg{
			Applied: true, Restarted: restarted, NeedsRestart: needsRestart, Remote: remote,
		}
		m.publishNotice(i18n.T(mihomoOverrideNotice(result)), overrideNoticeLevel(result.Applied, result.NeedsRestart))
	} else {
		m.publishNotice(i18n.T(successKey), messages.NoticeSuccess)
	}
	if !remote {
		if cmd := m.syncAppliedCore(restarted, needsRestart); cmd != nil {
			return tea.Batch(cmd, sub.FetchSubs(m.profileSvc), m.coreState.Refresh())
		}
	}
	return m.refreshOnConnect()
}
