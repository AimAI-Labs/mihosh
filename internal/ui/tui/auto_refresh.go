package tui

import (
	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"reflect"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/nodes"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
)

func fetchAutoRefresh(client *service.Client, current nodes.State) tea.Cmd {
	return messages.CoreRequest(client, func(client *service.Client) tea.Msg {
		proxies, err := client.GetProxies()
		if err != nil {
			return messages.ErrMsg{Err: err}
		}
		configs, err := client.GetConfigs()
		if err != nil {
			return messages.ErrMsg{Err: err}
		}
		groups, orderedNames := model.ProxySnapshot(proxies).Groups()
		result := messages.AutoRefreshResult{
			Groups:       groups,
			OrderedNames: orderedNames,
			Proxies:      proxies,
			Mode:         configs.Mode,
		}
		return messages.AutoRefreshMsg{Result: result, Changed: detectAutoRefreshChange(current, result)}
	})
}

func detectAutoRefreshChange(current nodes.State, result messages.AutoRefreshResult) bool {
	if current.Mode != "" && result.Mode != "" && current.Mode != result.Mode {
		return true
	}
	if !reflect.DeepEqual(current.GroupNames, result.OrderedNames) {
		return true
	}
	if len(current.Groups) != len(result.Groups) {
		return true
	}
	for name, group := range result.Groups {
		currentGroup, ok := current.Groups[name]
		if !ok {
			return true
		}
		if currentGroup.Now != group.Now || !reflect.DeepEqual(currentGroup.All, group.All) {
			return true
		}
	}
	return false
}
