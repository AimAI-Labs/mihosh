package connections

import (
	"context"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
)

func (s State) startSiteTests(indices []int, timeout int) (State, tea.Cmd) {
	if len(indices) == 0 {
		return s, nil
	}
	s.CancelTest()
	names := make([]string, len(indices))
	for i, index := range indices {
		names[i] = s.siteTests[index].Name
		s.siteTests[index].Testing = true
	}
	s.siteTestIndices = indices
	s.siteBatch = service.NewTestBatch(names)
	s.testCtx, s.testCancel = context.WithCancel(context.Background())
	return s.launchSiteTests(timeout)
}

func (s State) launchSiteTests(timeout int) (State, tea.Cmd) {
	var cmds []tea.Cmd
	for _, task := range s.siteBatch.Take() {
		site := s.siteTests[s.siteTestIndices[task.Index]]
		ctx, proxyAddr := s.testCtx, s.proxyAddr
		cmds = append(cmds, messages.CoreRequest(s.client, func(_ *api.Client) tea.Msg {
			result := TestSiteDelay(ctx, proxyAddr, site.Name, site.URL, timeout)().(messages.SiteTestMsg)
			result.BatchID, result.TaskID = task.BatchID, task.Index
			return result
		}))
	}
	return s, tea.Batch(cmds...)
}

func (s State) CompleteSiteTest(msg messages.SiteTestMsg) (State, tea.Cmd) {
	if !s.siteBatch.Complete(msg.BatchID, msg.TaskID) {
		return s, nil
	}
	index := s.siteTestIndices[msg.TaskID]
	s.siteTests[index].Testing = false
	if msg.Err != nil {
		s.siteTests[index].Delay = 0
		s.siteTests[index].Error = "timeout"
	} else {
		s.siteTests[index].Delay = msg.Delay
		s.siteTests[index].Error = ""
	}
	if !s.siteBatch.Active() {
		s.CancelTest()
		return s, nil
	}
	return s.launchSiteTests(s.timeout)
}
