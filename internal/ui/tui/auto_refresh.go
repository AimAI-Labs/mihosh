package tui

import (
	"reflect"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/nodes"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
)

type autoRefreshResult struct {
	Groups       map[string]model.Group
	OrderedNames []string
	Proxies      map[string]model.Proxy
	Mode         string
}

type autoRefreshMsg struct {
	Result  autoRefreshResult
	Changed bool
}

func fetchAutoRefresh(client *api.Client, current nodes.State) tea.Cmd {
	return func() tea.Msg {
		proxies, err := client.GetProxies()
		if err != nil {
			return messages.ErrMsg{Err: err}
		}
		configs, err := client.GetConfigs()
		if err != nil {
			return messages.ErrMsg{Err: err}
		}
		groups, orderedNames := groupsFromProxies(proxies)
		result := autoRefreshResult{
			Groups:       groups,
			OrderedNames: orderedNames,
			Proxies:      proxies,
			Mode:         configs.Mode,
		}
		return autoRefreshMsg{Result: result, Changed: detectAutoRefreshChange(current, result)}
	}
}

func groupsFromProxies(proxies map[string]model.Proxy) (map[string]model.Group, []string) {
	groups := make(map[string]model.Group)
	for name, proxy := range proxies {
		if len(proxy.All) > 0 {
			groups[name] = model.Group{
				Name:    name,
				Type:    proxy.Type,
				Now:     proxy.Now,
				All:     proxy.All,
				History: proxy.History,
			}
		}
	}

	var orderedNames []string
	if global, ok := proxies["GLOBAL"]; ok {
		for _, name := range global.All {
			if _, isGroup := groups[name]; isGroup {
				orderedNames = append(orderedNames, name)
			}
		}
	}
	if len(orderedNames) != len(groups) {
		existing := make(map[string]bool, len(orderedNames))
		for _, name := range orderedNames {
			existing[name] = true
		}
		for name := range groups {
			if !existing[name] {
				orderedNames = append(orderedNames, name)
			}
		}
	}

	return groups, orderedNames
}

func detectAutoRefreshChange(current nodes.State, result autoRefreshResult) bool {
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
