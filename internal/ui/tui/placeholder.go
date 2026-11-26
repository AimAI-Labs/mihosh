package tui

import (
	"time"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/placeholder"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
)

// placeholderPage 判断页面是否依赖内核数据：未连接内核时这些页面统一展示占位屏。
func placeholderPage(page layout.PageType) bool {
	switch page {
	case layout.PageNodes, layout.PageConnections, layout.PageLogs, layout.PageRules, layout.PageSub:
		return true
	}
	return false
}

// coreUnavailable 表示已经检测过内核，且当前页面依赖的数据源未连接。
func (m *Model) coreUnavailable() bool {
	return m.connectionChecked && !m.connectionReady && placeholderPage(m.currentPage)
}

func placeholderTick() tea.Cmd {
	return tea.Tick(placeholder.FrameInterval, func(time.Time) tea.Msg {
		return messages.PlaceholderTickMsg{}
	})
}

// placeholderTickCmd 在占位屏可见时启动像素动画；连接成功或离开页面后自然停止，避免重复并发 tick 链。
func (m *Model) placeholderTickCmd() tea.Cmd {
	if !m.coreUnavailable() || m.placeholderTickActive {
		return nil
	}
	m.placeholderTickActive = true
	return placeholderTick()
}
