package connections

import (
	"context"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/dialog"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type unlockItem struct {
	name     string
	result   service.UnlockResult
	checking bool
}

type unlockView struct {
	open   bool
	cursor int
	items  []unlockItem
}

type unlockState struct {
	unlockView
	batch   service.TestBatch
	indices []int
	ctx     context.Context
	cancel  context.CancelFunc
}

func (s *State) openUnlock() {
	if len(s.unlock.items) == 0 {
		for _, name := range service.UnlockServiceNames() {
			s.unlock.items = append(s.unlock.items, unlockItem{name: name})
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
	switch {
	case key.Matches(msg, common.Keys.Escape), msg.String() == "q":
		s.closeUnlock()
	case key.Matches(msg, common.Keys.Up):
		s.unlock.move(-1)
	case key.Matches(msg, common.Keys.Down):
		s.unlock.move(1)
	case key.Matches(msg, common.Keys.Enter):
		return s.startUnlock([]int{s.unlock.cursor})
	case msg.String() == "S":
		indices := make([]int, len(s.unlock.items))
		for i := range indices {
			indices[i] = i
		}
		return s.startUnlock(indices)
	}
	return s, nil
}

func (u *unlockState) move(delta int) {
	u.cursor = max(0, min(u.cursor+delta, len(u.items)-1))
}

func (s State) clickUnlock(x, y, width, height int) (State, tea.Cmd) {
	popup, window := s.unlock.popup(width, height)
	left, top, right, bottom := popup.Bounds()
	if x < left || x >= right || y < top || y >= bottom {
		s.closeUnlock()
		return s, nil
	}
	index := window.Start + y - top - 1
	if x > left && x < right-1 && index >= window.Start && index < window.End {
		s.unlock.cursor = index
		return s.startUnlock([]int{index})
	}
	return s, nil
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
		return s, nil
	}
	return s.launchUnlock()
}

func (u unlockView) popup(width, height int) (dialog.Dialog, common.ListWindow) {
	panelWidth := max(24, min(62, width-2))
	window := common.ResolveListWindow(len(u.items), u.cursor, 0, height-3)
	inner := panelWidth - 4
	statusWidth := inner * 45 / 100
	nameWidth := inner - statusWidth - 1

	var rows []string
	for index := window.Start; index < window.End; index++ {
		item := u.items[index]
		status, color := "pending", common.Muted()
		switch item.result.Status {
		case service.UnlockAvailable:
			status, color = string(item.result.Status), common.Success()
		case service.UnlockRestricted, service.UnlockFailed:
			status, color = string(item.result.Status), common.Danger()
		case service.UnlockOriginals, service.UnlockComingSoon, service.UnlockInconclusive:
			status, color = string(item.result.Status), common.Warning()
		}
		result := i18n.T("conns.unlock." + status)
		if item.checking {
			result, color = i18n.T("conns.unlock.checking"), common.Active()
		} else {
			if item.result.Region != "" {
				result += " " + item.result.Region
			}
			if inner >= 52 && !item.result.CheckedAt.IsZero() {
				result += " " + item.result.CheckedAt.Local().Format("15:04")
			}
		}
		name := item.name
		if index == window.Selected {
			name = common.SymbolSelectActive + name
		} else {
			name = common.SymbolSelectInactive + name
		}
		left := lipgloss.NewStyle().Width(nameWidth).Foreground(common.Bright()).
			Render(ansi.Truncate(name, nameWidth, "…"))
		right := lipgloss.NewStyle().Foreground(color).
			Render(ansi.Truncate(result, statusWidth, "…"))
		rows = append(rows, left+" "+right)
	}
	panel := common.RenderBorderedPanel(i18n.T("conns.unlock.title"), strings.Join(rows, "\n"), panelWidth, common.Active(), common.Bright())
	return dialog.New(panel, width, max(1, height-1)).Clipped(), window
}
