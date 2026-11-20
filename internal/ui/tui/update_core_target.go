package tui

import (
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
)

type coreTargetUpdate uint8

const (
	coreTargetSync coreTargetUpdate = iota
	coreTargetReconnect
	coreTargetStopped
)

// updateCoreTarget owns endpoint synchronization and invalidation for every entry point.
// Page-local cleanup stays behind CoreTargetChangedMsg; existing WS streams reconnect themselves.
func (m *Model) updateCoreTarget(endpoint config.MihomoEndpoint, update coreTargetUpdate) tea.Cmd {
	reset := false
	if m.client != nil {
		old := m.client.EndpointSnapshot()
		reset = old.ExternalController != endpoint.ExternalController || old.Secret != endpoint.Secret
	}
	if reset || update != coreTargetSync {
		m.connectionEpoch++
		m.connectionReady = false
		m.connectionChecking = false
		if m.client != nil {
			m.client.UpdateEndpoint(endpoint.ExternalController, endpoint.Secret)
		}
	}
	if reset {
		if m.wsClient != nil {
			m.wsClient.UpdateEndpoint(endpoint.ExternalController, endpoint.Secret)
		}
	}
	path, _ := config.GetMihomoConfigPathForWrite()
	changed := messages.CoreTargetChangedMsg{Reset: reset, Invalidate: reset || update != coreTargetSync, ConfigPath: path, ProxyURL: config.MixedPortToProxyURL(endpoint.MixedPort)}
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

func (m *Model) syncAppliedCore(restarted, pendingRestart bool) tea.Cmd {
	endpoint := config.ResolveMihomoEndpoint()
	if restarted {
		return m.updateCoreTarget(endpoint, coreTargetReconnect)
	}
	if pendingRestart && m.client != nil {
		// The live controller still uses its old address and secret after hot reload.
		current := m.client.EndpointSnapshot()
		endpoint.ExternalController, endpoint.Secret = current.ExternalController, current.Secret
	}
	return m.updateCoreTarget(endpoint, coreTargetSync)
}

func (m *Model) coreOperationCompleted(action string) tea.Cmd {
	m.reloadStoredConfig()
	switch action {
	case "stop", "service-stop", "service-uninstall":
		return m.updateCoreTarget(config.ResolveMihomoEndpoint(), coreTargetStopped)
	case "configure", "basic-config", "start", "setup", "connect", "external", "ports", "activate", "rollback", "restart", "upgrade", "profile", "service-install", "service-start", "service-restart", "tun":
		return m.updateCoreTarget(config.ResolveMihomoEndpoint(), coreTargetReconnect)
	default:
		return nil
	}
}
