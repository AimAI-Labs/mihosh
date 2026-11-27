package tui

import (
	"time"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/preferences"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/nodes"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/rules"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) applyCoreEvent(msg tea.Msg) (Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case messages.CoreTargetResolvedMsg:
		cmd := m.applyResolvedCoreTarget(msg)
		return m, cmd, true

	case messages.CoreOwnerChangedMsg:
		update := coreTargetReconnect
		if msg.Action == "stop" {
			update = coreTargetStopped
		}
		cmd := m.requestCoreTarget(update, coreTargetOptions{})
		return m, tea.Batch(m.listenCoreOwner(), m.coreState.Refresh(), cmd), true

	case messages.ManagedCoreStartedMsg:
		if msg.Epoch != m.connectionEpoch {
			return m, nil, true
		}
		cmd := m.requestCoreTarget(coreTargetReconnect, coreTargetOptions{})
		return m, cmd, true

	case messages.CoreConnectionMsg:
		if msg.Epoch != m.connectionEpoch {
			return m, nil, true
		}
		m.coreState, _ = m.coreState.Update(msg)
		m.connectionChecked = true
		m.connectionChecking = false
		m.connectionReady = msg.Err == nil
		m.syncProcessLogs()
		if msg.Err != nil {
			if !msg.KeepPage {
				m.currentPage = layout.PageCore
			}
			return m, m.placeholderTickCmd(), true
		}
		var stream tea.Cmd
		if !m.wsStarted {
			m.wsStarted = true
			stream = startWSStreams(m.wsCtx, m.wsClient, m.wsInbox)
		}
		if m.autoRefreshInterval() > 0 {
			// Run one follow-up auto-refresh on the next tick so delayed core startup data appears automatically.
			m.autoRefreshRemaining = 1
		}
		return m, tea.Batch(stream, m.refreshOnConnect()), true

	case messages.CoreStatusMsg, messages.CoreServiceLogMsg, messages.CoreGeoDataMsg, messages.CoreProgressTickMsg, messages.CoreOperationMsg, messages.SysStatusResultMsg:
		if result, ok := msg.(messages.CoreOperationMsg); ok && result.ID != m.coreState.OperationID() {
			return m, nil, true
		}
		var cmd tea.Cmd
		m.coreState, cmd = m.coreState.Update(msg)
		if result, ok := msg.(messages.CoreOperationMsg); ok {
			if n, ok := m.coreState.OperationNotice(result); ok {
				if n.Level != messages.NoticeError {
					m.clearErrorNotice()
				}
				m.notice.Publish(n)
			}
			m.coreState = m.coreState.ClearTaskNotice()
			var targetCmd tea.Cmd
			if result.Err == nil {
				targetCmd = m.coreOperationCompleted(result.Action)
			}
			m.syncProcessLogs()
			return m, tea.Batch(cmd, targetCmd), true
		}
		if statusMsg, ok := msg.(messages.CoreStatusMsg); ok {
			if statusMsg.Err != nil {
				m.publishError(statusMsg.Err)
				m.coreState = m.coreState.ClearTaskNotice()
			} else {
				m.syncProcessLogs()
				if statusMsg.Status.Mode == "managed" && !statusMsg.Status.Running && m.connectionReady {
					resolve := m.requestCoreTarget(coreTargetStopped, coreTargetOptions{})
					return m, tea.Batch(cmd, resolve), true
				}
			}
		}
		return m, cmd, true

	case messages.ShowGuideModalMsg:
		m.guideState = m.guideState.Activate(msg.Endpoint, msg.SecretMasked, msg.ErrMessage)
		return m, nil, true

	case messages.HideGuideModalMsg:
		if m.guideState.Active {
			m.publishNotice(i18n.T("status.normal"), messages.NoticeInfo)
		}
		m.guideState = m.guideState.Dismiss()
		return m, nil, true

	case messages.UpdateCheckedMsg:
		if msg.Info != nil {
			m.updateInfo = msg.Info
			m.hasUpdate = true
		}
		return m, nil, true

	case messages.UpdateAppliedMsg:
		if msg.Err != nil {
			m.updateError = msg.Err
			m.isUpdating = false
			if !m.showUpdateDialog {
				m.publishError(msg.Err)
			}
			return m, nil, true
		}
		if msg.ServiceBinary != "" {
			m.exitNotice = i18n.Tf("update.service_updated", msg.ServiceBinary)
		}
		text := i18n.T("status.update_applied")
		if text == "status.update_applied" {
			text = "Update applied, please restart."
		}
		m.publishNotice(text, messages.NoticeSuccess)
		m.onAppQuit()
		return m, tea.Quit, true

	case messages.CoreActionDoneMsg:
		m.coreState, _ = m.coreState.Update(msg)
		m.publishNotice(msg.Action+" "+i18n.T("settings.action.success"), messages.NoticeSuccess)
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
				), true
			}
			return m, tea.Batch(cmds...), true
		}
		return m, nil, true

	case messages.CoreActionErrorMsg:
		m.coreState, _ = m.coreState.Update(msg)
		m.publishError(msg.Err)
		return m, nil, true
	}

	return m, nil, false
}
