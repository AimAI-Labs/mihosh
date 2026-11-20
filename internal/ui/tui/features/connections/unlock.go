package connections

import (
	"context"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type unlockItem struct {
	name     string
	result   model.UnlockResult
	checking bool
}

type unlockView struct {
	open   bool
	cursor int
	items  []unlockItem
}

type unlockState struct {
	unlockView
	width   int
	batch   service.TestBatch
	indices []int
	ctx     context.Context
	cancel  context.CancelFunc
}

func (s *State) openUnlock() {
	if len(s.unlock.items) == 0 {
		results := config.LoadUnlockResults(s.unlockScope())
		for _, name := range service.UnlockServiceNames() {
			s.unlock.items = append(s.unlock.items, unlockItem{name: name, result: results[name]})
		}
	}
	s.unlock.open = true
}

func (u *unlockState) cancelChecks() {
	if u.cancel != nil {
		u.cancel()
		u.cancel = nil
	}
	u.batch.Cancel()
	u.indices = nil
	u.ctx = nil
	for i := range u.items {
		u.items[i].checking = false
	}
}

func (s *State) closeUnlock() {
	s.unlock.cancelChecks()
	s.unlock.open = false
}

func (s State) handleUnlockKey(msg tea.KeyMsg) (State, tea.Cmd) {
	columns := unlockColumns(s.unlock.width)
	switch {
	case key.Matches(msg, common.Keys.Escape), msg.String() == "q":
		s.closeUnlock()
	case key.Matches(msg, common.Keys.Up):
		s.unlock.move(-columns)
	case key.Matches(msg, common.Keys.Down):
		s.unlock.move(columns)
	case key.Matches(msg, common.Keys.Left):
		if s.unlock.cursor%columns > 0 {
			s.unlock.move(-1)
		}
	case key.Matches(msg, common.Keys.Right):
		if s.unlock.cursor%columns < columns-1 {
			s.unlock.move(1)
		}
	case key.Matches(msg, common.Keys.Enter):
		return s.startUnlock([]int{s.unlock.cursor})
	case msg.String() == "S":
		return s.startAllUnlockChecks()
	}
	return s, nil
}

func (u *unlockState) move(delta int) {
	u.cursor = max(0, min(u.cursor+delta, len(u.items)-1))
}

func (s State) clickUnlock(x, y, width, height int) (State, tea.Cmd) {
	s.unlock.width = width
	popup, layout := s.unlock.popup(width, height)
	left, top, right, bottom := popup.Bounds()
	if x < left || x >= right || y < top || y >= bottom {
		s.closeUnlock()
		return s, nil
	}
	x, y = x-left, y-top
	if y == 1 && x >= layout.allButtonX && x < layout.allButtonX+layout.allButtonW {
		return s.startAllUnlockChecks()
	}
	x, y = x-unlockGridInset, y-unlockGridTop
	if index := layout.grid.ItemAt(x, y); index >= 0 {
		s.unlock.cursor = index
		cardX, cardY, cardWidth := layout.grid.CardOrigin(index)
		if layout.grid.CardHeight >= 3 && x == cardX+cardWidth-3 && y == cardY+1 {
			return s.startUnlock([]int{index})
		}
	}
	return s, nil
}

func (s State) startAllUnlockChecks() (State, tea.Cmd) {
	indices := make([]int, len(s.unlock.items))
	for i := range indices {
		indices[i] = i
	}
	return s.startUnlock(indices)
}

func (s State) startUnlock(indices []int) (State, tea.Cmd) {
	if s.unlock.batch.Active() || len(indices) == 0 {
		return s, nil
	}
	if s.proxyAddr == "" {
		return s, func() tea.Msg {
			return messages.NoticeMsg{Text: i18n.T("conns.unlock.no_proxy"), Level: messages.NoticeWarning}
		}
	}
	s.CancelTest()
	names := make([]string, len(indices))
	for i, index := range indices {
		names[i] = s.unlock.items[index].name
		s.unlock.items[index].checking = true
	}
	s.unlock.indices = indices
	s.unlock.ctx, s.unlock.cancel = context.WithCancel(context.Background())
	s.unlock.batch = service.NewTestBatch(names)
	return s.launchUnlock()
}

func (s State) launchUnlock() (State, tea.Cmd) {
	var cmds []tea.Cmd
	for _, task := range s.unlock.batch.Take() {
		index := s.unlock.indices[task.Index]
		ctx, proxy := s.unlock.ctx, s.proxyAddr
		cmds = append(cmds, messages.CoreRequest(s.client, func(_ *api.Client) tea.Msg {
			return messages.UnlockTestMsg{
				BatchID: task.BatchID, TaskID: task.Index, Result: service.CheckUnlock(ctx, proxy, index),
			}
		}))
	}
	return s, tea.Batch(cmds...)
}

func (s State) completeUnlock(msg messages.UnlockTestMsg) (State, tea.Cmd) {
	if !s.unlock.batch.Complete(msg.BatchID, msg.TaskID) {
		return s, nil
	}
	item := &s.unlock.items[s.unlock.indices[msg.TaskID]]
	item.checking, item.result = false, msg.Result
	if !s.unlock.batch.Active() {
		s.unlock.cancelChecks()
		scope := s.unlockScope()
		snapshot := make(map[string]model.UnlockResult, len(s.unlock.items))
		for _, item := range s.unlock.items {
			snapshot[item.name] = item.result
		}
		return s, func() tea.Msg {
			if err := config.SaveUnlockResults(scope, snapshot); err != nil {
				return messages.NoticeMsg{Text: i18n.T("conns.unlock.cache_failed"), Detail: err.Error(), Level: messages.NoticeWarning}
			}
			return nil
		}
	}
	return s.launchUnlock()
}

func (s State) unlockScope() string {
	var endpoint config.MihomoEndpoint
	if s.client != nil {
		endpoint = s.client.EndpointSnapshot()
	}
	return config.UnlockResultsScope(s.proxyAddr, endpoint)
}
