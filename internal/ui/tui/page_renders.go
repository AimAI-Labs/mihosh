package tui

import (
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/logs"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/nodes"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/rules"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/settings"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/sub"
)

// getPageSize 计算页面内容的可用宽度和高度
func (m Model) getPageSize() (pageWidth, pageHeight int) {
	mainWidth := m.width
	if mainWidth < common.MinMainWidth {
		mainWidth = common.MinMainWidth
	}
	pageWidth = mainWidth
	if pageWidth < common.MinMainWidth {
		pageWidth = common.MinMainWidth
	}
	pageHeight = m.height - layout.TopNavHeight - common.StatusBarHeight
	if pageHeight < common.MinContentHeight {
		pageHeight = common.MinContentHeight
	}
	return pageWidth, pageHeight
}

// renderNodesPage 渲染节点管理页面
func (m Model) renderNodesPage() string {
	pageWidth, pageHeight := m.getPageSize()
	state := m.nodesState.ToPageState(pageWidth, pageHeight)
	return nodes.RenderNodesPage(state)
}

func (m Model) renderConnectionsPage() string {
	pageWidth, pageHeight := m.getPageSize()
	state := m.connsState.ToPageState(m.chartData, pageWidth, pageHeight)
	return connections.RenderConnectionsPage(state)
}

// renderSettingsPage 渲染设置页面
func (m Model) renderSettingsPage() string {
	pageWidth, pageHeight := m.getPageSize()
	state := m.settingsState.ToPageState(m.config)
	return settings.RenderSettingsPage(state, pageWidth, pageHeight)
}

// renderLogsPage 渲染日志页面
func (m Model) renderLogsPage() string {
	pageWidth, pageHeight := m.getPageSize()
	state := m.logsState.ToPageState(pageWidth, pageHeight)
	return logs.RenderLogsPage(state)
}

// renderRulesPage 渲染规则页面
func (m Model) renderRulesPage() string {
	pageWidth, pageHeight := m.getPageSize()
	state := m.rulesState.ToPageState(pageWidth, pageHeight)
	return rules.RenderRulesPage(state)
}

// renderSubPage 渲染订阅管理页面
func (m Model) renderSubPage() string {
	pageWidth, pageHeight := m.getPageSize()
	state := m.subState.ToPageState(pageWidth, pageHeight)
	return sub.RenderSubPage(state)
}
