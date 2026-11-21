package nodes

import (
	"fmt"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
)

func FetchGroups(client *service.Client) tea.Cmd {
	return messages.CoreRequest(client, func(client *service.Client) tea.Msg {
		groups, orderedNames, err := client.GetGroups()
		if err != nil {
			return messages.ErrMsg{Err: err}
		}
		return messages.GroupsMsg{Groups: groups, OrderedNames: orderedNames}
	})
}

func FetchProxies(client *service.Client) tea.Cmd {
	return messages.CoreRequest(client, func(client *service.Client) tea.Msg {
		proxies, err := client.GetProxies()
		if err != nil {
			return messages.ErrMsg{Err: err}
		}
		return messages.ProxiesMsg(proxies)
	})
}

func SelectProxy(client *service.Client, group, proxy string) tea.Cmd {
	return func() tea.Msg {
		if err := client.SelectProxy(group, proxy); err != nil {
			return messages.ErrMsg{Err: err}
		}
		return tea.Batch(FetchGroups(client), FetchProxies(client))()
	}
}

func TestProxy(client *service.Client, task service.TestTask, testURL string, timeout int) tea.Cmd {
	return messages.CoreRequest(client, func(client *service.Client) (msg tea.Msg) {
		result := messages.TestDoneMsg{Name: task.Name, Delay: -1, TestURL: testURL, BatchID: task.BatchID, TaskID: task.Index}
		defer func() {
			if r := recover(); r != nil {
				result.Err = fmt.Errorf("panic in TestProxy: %v", r)
				result.Delay = -1
			}
			msg = result
		}()
		if client == nil {
			result.Err = fmt.Errorf("client is nil")
			return
		}
		result.Delay, result.Err = client.TestProxyDelay(task.Name, testURL, timeout)
		if result.Err != nil {
			result.Delay = -1
		}
		return
	})
}

func FetchConfigMode(client *service.Client) tea.Cmd {
	return messages.CoreRequest(client, func(client *service.Client) tea.Msg {
		configs, err := client.GetConfigs()
		if err != nil {
			return messages.ErrMsg{Err: err}
		}
		return messages.ConfigModeMsg{Mode: configs.Mode}
	})
}

func UpdateConfigMode(client *service.Client, mode string) tea.Cmd {
	return func() tea.Msg {
		err := client.UpdateConfig(model.UpdateConfigRequest{Mode: mode})
		if err != nil {
			return messages.ErrMsg{Err: err}
		}
		return messages.ConfigModeMsg{Mode: mode}
	}
}
