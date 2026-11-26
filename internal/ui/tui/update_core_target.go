package tui

import (
	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/bootstrap"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/preferences"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
)

type coreTargetAction uint8

const (
	coreTargetSync coreTargetAction = iota
	coreTargetReconnect
	coreTargetStopped
)

type coreTargetOptions struct {
	keepIdentity  bool
	probe         bool
	refresh       bool
	refreshConfig bool
}

type coreTargetRequest struct {
	action coreTargetAction
	coreTargetOptions
}

func (m *Model) requestCoreTarget(action coreTargetAction, opts coreTargetOptions) tea.Cmd {
	if action == coreTargetSync && m.targetPending != nil && m.targetPending.action != coreTargetSync {
		// A newer snapshot still has to invalidate work from the previous runtime.
		action = m.targetPending.action
		if action == coreTargetStopped && opts.probe {
			action = coreTargetReconnect
		}
	}
	m.targetRequest++
	m.targetPending = &coreTargetRequest{action: action, coreTargetOptions: opts}
	request := m.targetRequest
	return func() tea.Msg {
		cfg, err, target := bootstrap.DiscoverTarget()
		if err != nil {
			cfg = nil
		}
		return messages.CoreTargetResolvedMsg{Request: request, Target: target, Config: cfg}
	}
}

func (m *Model) applyResolvedCoreTarget(msg messages.CoreTargetResolvedMsg) tea.Cmd {
	if msg.Request != m.targetRequest || m.targetPending == nil {
		return nil
	}
	request := m.targetPending
	m.targetPending = nil
	if msg.Config != nil {
		m.applyStoredConfig(msg.Config)
	}
	if request.action == coreTargetSync && request.keepIdentity && m.client != nil {
		// A pending manual restart leaves the live controller on its old credentials.
		current := m.client.EndpointSnapshot()
		msg.Target.Endpoint.ExternalController, msg.Target.Endpoint.Secret = current.ExternalController, current.Secret
	}
	cmd := m.updateCoreTarget(msg.Target, request.action)
	if cmd == nil {
		switch {
		case request.probe:
			cmd = m.checkCoreConnection(false)
		case request.refresh:
			cmd = m.refreshOnConnect()
		case request.refreshConfig:
			cmd = m.fetchNodes()
		}
	}
	if request.refreshConfig {
		cmd = tea.Batch(cmd, preferences.FetchMihomoConfig(m.configSvc, m.client))
	}
	return cmd
}

// updateCoreTarget owns endpoint synchronization and invalidation for every entry point.
// Page-local cleanup stays behind CoreTargetChangedMsg; existing WS streams reconnect themselves.
func (m *Model) updateCoreTarget(target service.MihomoTarget, update coreTargetAction) tea.Cmd {
	endpoint := target.Endpoint
	reset := false
	if m.client != nil {
		old := m.client.EndpointSnapshot()
		reset = old.ExternalController != endpoint.ExternalController || old.Secret != endpoint.Secret
	}
	invalidate := reset || update != coreTargetSync
	if invalidate {
		m.connectionEpoch++
		m.connectionReady = false
		m.connectionChecking = false
		if m.client != nil {
			m.client.UpdateEndpoint(endpoint.ExternalController, endpoint.Secret)
		}
		if m.wsClient != nil {
			m.wsClient.UpdateEndpoint(endpoint.ExternalController, endpoint.Secret)
			if m.wsInbox != nil {
				m.wsInbox.reset(m.wsClient.Generation())
			}
		}
	}
	changed := messages.CoreTargetChangedMsg{Reset: reset, Invalidate: invalidate, ConfigPath: target.ConfigPath, ProxyURL: service.MixedPortToProxyURL(endpoint.MixedPort)}
	m.rulesState, _ = m.rulesState.Update(changed)
	m.connsState, _ = m.connsState.Update(changed)
	m.nodesState, _ = m.nodesState.Update(changed)
	m.logsState, _ = m.logsState.Update(changed)
	if reset && m.chartData != nil {
		*m.chartData = *model.NewChartData(common.ChartPoints)
	}
	if update == coreTargetStopped {
		m.connectionChecked = true
		return nil
	}
	if reset || update == coreTargetReconnect {
		return m.checkCoreConnection(false)
	}
	return nil
}

func (m *Model) syncAppliedCore(restarted, pendingRestart, refresh bool) tea.Cmd {
	if restarted {
		return m.requestCoreTarget(coreTargetReconnect, coreTargetOptions{})
	}
	return m.requestCoreTarget(coreTargetSync, coreTargetOptions{keepIdentity: pendingRestart, refresh: refresh})
}

func (m *Model) coreOperationCompleted(action string) tea.Cmd {
	switch action {
	case "stop", "service-stop", "service-uninstall":
		return m.requestCoreTarget(coreTargetStopped, coreTargetOptions{})
	case "configure", "basic-config", "start", "setup", "connect", "external", "ports", "activate", "rollback", "restart", "upgrade", "profile", "service-install", "service-start", "service-restart", "tun":
		return m.requestCoreTarget(coreTargetReconnect, coreTargetOptions{})
	default:
		m.reloadStoredConfig()
		return nil
	}
}
