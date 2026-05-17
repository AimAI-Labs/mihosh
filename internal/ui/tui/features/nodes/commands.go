package nodes

import (
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/internal/app/service" // Need for testAllProxies
	tea "github.com/charmbracelet/bubbletea"
)

func FetchGroups(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		groups, orderedNames, err := client.GetGroups()
		if err != nil {
			return messages.ErrMsg{Err: err}
		}
		return messages.GroupsMsg{Groups: groups, OrderedNames: orderedNames}
	}
}

func FetchProxies(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		proxies, err := client.GetProxies()
		if err != nil {
			return messages.ErrMsg{Err: err}
		}
		return messages.ProxiesMsg(proxies)
	}
}

func SelectProxy(client *api.Client, group, proxy string) tea.Cmd {
	return func() tea.Msg {
		if err := client.SelectProxy(group, proxy); err != nil {
			return messages.ErrMsg{Err: err}
		}
		return tea.Batch(FetchGroups(client), FetchProxies(client))()
	}
}

func TestProxy(client *api.Client, name, testURL string, timeout int) tea.Cmd {
	return func() tea.Msg {
		delay, err := client.TestProxyDelay(name, testURL, timeout)
		if err != nil {
			return messages.TestDoneMsg{Name: name, Delay: -1, Err: err}
		}
		return messages.TestDoneMsg{Name: name, Delay: delay, Err: nil}
	}
}

func TestGroup(client *api.Client, group, testURL string, timeout int) tea.Cmd {
	return func() tea.Msg {
		if err := client.TestGroupDelay(group, testURL, timeout); err != nil {
			return messages.ErrMsg{Err: err}
		}
		return messages.TestDoneMsg{}
	}
}

func TestAllProxies(proxySvc *service.ProxyService, proxies []string) tea.Cmd {
	return func() tea.Msg {
		results := proxySvc.TestAllProxies(proxies)
		return messages.TestAllDoneMsg{Results: results}
	}
}

func LaunchBatchTests(client *api.Client, testURL string, timeout int, pending []string) tea.Cmd {
	if len(pending) == 0 {
		return nil
	}
	cmds := make([]tea.Cmd, 0, len(pending))
	for _, name := range pending {
		cmds = append(cmds, TestProxy(client, name, testURL, timeout))
	}
	return tea.Batch(cmds...)
}

func FetchConfigMode(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		configs, err := client.GetConfigs()
		if err != nil {
			return messages.ErrMsg{Err: err}
		}
		return messages.ConfigModeMsg{Mode: configs.Mode}
	}
}

func UpdateConfigMode(client *api.Client, mode string) tea.Cmd {
	return func() tea.Msg {
		err := client.UpdateConfig(model.UpdateConfigRequest{Mode: mode})
		if err != nil {
			return messages.ErrMsg{Err: err}
		}
		return messages.ConfigModeMsg{Mode: mode}
	}
}
