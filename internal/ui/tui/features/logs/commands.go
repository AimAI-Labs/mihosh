package logs

import (
	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
)

// ResolveLogSourceIP 异步解析内网IP来源
func ResolveLogSourceIP(resolver *service.IPResolver, ip string) tea.Cmd {
	return func() tea.Msg {
		resolved := resolver.Resolve(ip)
		return messages.LogIPResolvedMsg{
			IP:       ip,
			Resolved: resolved,
		}
	}
}
