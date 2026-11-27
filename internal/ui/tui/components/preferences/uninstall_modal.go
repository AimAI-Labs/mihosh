package preferences

import (
	"context"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	uninstallFocusKeepConfig = 0
	uninstallFocusConfirm    = 1
	uninstallFocusCancel     = 2
	uninstallFocusCount      = 3
)

// UninstallModalState holds page-local state for Settings -> One-Click Uninstall modal.
type UninstallModalState struct {
	Open       bool
	Loading    bool
	Busy       bool
	KeepConfig bool
	Focus      int
	Plan       *service.UninstallPlan
	ErrText    string
}

func (s State) uninstallService() *service.UninstallService {
	if s.uninstallSvc != nil {
		return s.uninstallSvc
	}
	return service.NewUninstallService(s.coreSvc)
}

// SetCoreService wires the shared CoreService used by uninstall operations.
func (s State) SetCoreService(coreSvc *service.CoreService) State {
	s.coreSvc = coreSvc
	return s
}

// SetUninstallService injects a custom UninstallService (primarily for unit tests).
func (s State) SetUninstallService(svc *service.UninstallService) State {
	s.uninstallSvc = svc
	return s
}

// OpenUninstallModal opens the uninstall confirmation modal and triggers async plan inspection.
func (s State) OpenUninstallModal() (State, tea.Cmd) {
	if s.uninstallModal.Busy {
		s.uninstallModal.Open = true
		return s, nil
	}
	s.uninstallModal = UninstallModalState{
		Open:       true,
		Loading:    true,
		KeepConfig: true,
		Focus:      uninstallFocusKeepConfig,
	}
	return s, loadUninstallPlanCmd(s.uninstallService())
}

// ApplyUninstallPlan stores the inspected uninstall plan into the modal state.
func (s State) ApplyUninstallPlan(msg messages.UninstallPlanMsg) State {
	if !s.uninstallModal.Open {
		return s
	}
	s.uninstallModal.Loading = false
	if msg.Err != nil {
		s.uninstallModal.ErrText = msg.Err.Error()
		return s
	}
	plan := msg.Plan
	s.uninstallModal.Plan = &plan
	s.uninstallModal.ErrText = ""
	return s
}

// CompleteUninstall records a failed uninstall attempt in the modal state.
func (s State) CompleteUninstall(msg messages.UninstallDoneMsg) State {
	s.uninstallModal.Busy = false
	if !s.uninstallModal.Open {
		return s
	}
	if msg.Err != nil {
		s.uninstallModal.ErrText = msg.Err.Error()
	}
	return s
}

func (s State) handleUninstallModalKey(msg tea.KeyMsg) (State, tea.Cmd) {
	m := &s.uninstallModal
	if m.Busy {
		return s, nil
	}
	switch {
	case key.Matches(msg, common.Keys.Escape), msg.String() == "q":
		s.uninstallModal = UninstallModalState{}
		return s, nil
	case msg.Type == tea.KeySpace, msg.String() == " ", msg.String() == "k":
		if !m.Loading {
			m.KeepConfig = !m.KeepConfig
		}
		return s, nil
	case key.Matches(msg, common.Keys.Up), msg.String() == "shift+tab", msg.String() == "left":
		m.Focus = (m.Focus + uninstallFocusCount - 1) % uninstallFocusCount
		return s, nil
	case key.Matches(msg, common.Keys.Down), msg.String() == "tab", msg.String() == "right":
		m.Focus = (m.Focus + 1) % uninstallFocusCount
		return s, nil
	case key.Matches(msg, common.Keys.Enter):
		switch m.Focus {
		case uninstallFocusKeepConfig:
			if !m.Loading {
				m.KeepConfig = !m.KeepConfig
			}
			return s, nil
		case uninstallFocusConfirm:
			if m.Loading {
				return s, nil
			}
			m.Busy = true
			m.ErrText = ""
			return s, runUninstallCmd(s.uninstallService(), m.KeepConfig)
		case uninstallFocusCancel:
			s.uninstallModal = UninstallModalState{}
			return s, nil
		}
	}
	return s, nil
}

func (s State) handleUninstallModalClick(pageX, pageY, pageWidth, pageHeight int) (State, tea.Cmd) {
	modal := buildUninstallModal(s.ToPageState(s.Config, nil), pageWidth, pageHeight)
	left, top, right, bottom := modal.Bounds()
	if pageY < top || pageY >= bottom || pageX < left || pageX >= right {
		if s.uninstallModal.Busy {
			return s, nil
		}
		s.uninstallModal = UninstallModalState{}
		return s, nil
	}
	x, y := pageX-left-2, pageY-top
	if x < 0 || x >= modal.innerWidth {
		return s, nil
	}
	if row := y - modal.hintTop; row >= 0 && row < len(modal.hints) {
		if hint, ok := common.InlineHintAt(modal.hints[row], x, modal.innerWidth); ok {
			keyMsg, _ := hint.KeyMsg()
			return s.handleUninstallModalKey(keyMsg)
		}
	}
	if s.uninstallModal.Busy || s.uninstallModal.Loading {
		return s, nil
	}
	m := &s.uninstallModal
	if y == modal.toggleRowY {
		m.Focus = uninstallFocusKeepConfig
		m.KeepConfig = !m.KeepConfig
		return s, nil
	}
	if y == modal.confirmRowY && x >= modal.confirmStart && x < modal.confirmEnd {
		m.Focus = uninstallFocusConfirm
		m.Busy = true
		m.ErrText = ""
		return s, runUninstallCmd(s.uninstallService(), m.KeepConfig)
	}
	if y == modal.cancelRowY && x >= modal.cancelStart && x < modal.cancelEnd {
		s.uninstallModal = UninstallModalState{}
		return s, nil
	}
	return s, nil
}

func loadUninstallPlanCmd(svc *service.UninstallService) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		plan, err := svc.Plan(ctx, true)
		return messages.UninstallPlanMsg{Plan: plan, Err: err}
	}
}

func runUninstallCmd(svc *service.UninstallService, keepConfig bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		res, err := svc.Uninstall(ctx, service.UninstallOptions{
			KeepConfig:           keepConfig,
			AllowInteractiveSudo: false,
		})
		return messages.UninstallDoneMsg{Result: res, Err: err}
	}
}
