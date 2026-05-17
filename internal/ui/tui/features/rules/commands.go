package rules

import (
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
)

// FetchRules 获取规则列表
func FetchRules(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		rules, err := client.GetRules()
		if err != nil {
			return messages.ErrMsg{Err: err}
		}
		return messages.RulesMsg(rules.Rules)
	}
}
