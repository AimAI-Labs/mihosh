package tui

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/logs"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/nodes"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/settings"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/sub"
	tea "github.com/charmbracelet/bubbletea"
)

func capturedPage(t *testing.T, name string) Model {
	t.Helper()
	chart := model.NewChartData(10)
	m := Model{
		width: 120, height: 36, config: &config.Config{}, chartData: chart,
		connsState: connections.NewState("", nil, nil, 0, chart),
		logsState:  logs.NewState(nil).AppendLog("info", "first").AppendLog("info", "second"),
		nodesState: nodes.NewState(nil, "", 0), rulesState: newRulesStateWithRules(),
		subState:      sub.NewState(nil).ApplySubs([]profile.Profile{{UID: "a", Name: "first"}, {UID: "b", Name: "second"}}, ""),
		settingsState: settings.NewState(nil, nil),
	}
	m.connsState.Connections = &model.ConnectionsResponse{Connections: []model.Connection{
		{ID: "first", Metadata: model.Metadata{Host: "first.test"}},
		{ID: "second", Metadata: model.Metadata{Host: "second.test"}},
	}}
	m.nodesState.GroupNames = []string{"first", "second"}
	var keys []tea.KeyMsg
	switch name {
	case "connection detail", "connection filter":
		m.currentPage = layout.PageConnections
		keys = []tea.KeyMsg{keyMsg("h"), {Type: tea.KeyEnter}}
		if name == "connection filter" {
			keys[1] = keyMsg("/")
		}
	case "connection TopN":
		m.currentPage = layout.PageConnections
		for i := 0; i < 2; i++ {
			next, _ := m.Update(tea.MouseMsg{X: 1, Y: layout.TopNavHeight + 4, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
			m = next.(Model)
		}
		if !m.connsState.ToPageState(chart, 120, 32).TopNModalMode {
			t.Fatal("TopN did not open")
		}
	case "log detail", "log filter":
		m.currentPage = layout.PageLogs
		keys = []tea.KeyMsg{{Type: tea.KeyEnter}}
		if name == "log filter" {
			keys[0] = keyMsg("/")
		}
	case "node results", "node filter":
		m.currentPage = layout.PageNodes
		keys = []tea.KeyMsg{keyMsg("f")}
		if name == "node filter" {
			keys[0] = keyMsg("/")
		}
	case "rule add", "rule edit", "rule delete", "rule picker", "rule filter":
		m.currentPage = layout.PageRules
		keys = []tea.KeyMsg{keyMsg("n")}
		switch name {
		case "rule edit":
			keys[0] = tea.KeyMsg{Type: tea.KeyEnter}
		case "rule delete":
			keys[0] = keyMsg("d")
		case "rule filter":
			keys[0] = keyMsg("/")
		case "rule picker":
			keys = append(keys, tea.KeyMsg{Type: tea.KeyTab}, tea.KeyMsg{Type: tea.KeyTab}, tea.KeyMsg{Type: tea.KeyEnter})
		}
	case "subscription add", "subscription edit", "subscription delete", "subscription filter":
		m.currentPage = layout.PageSub
		keys = []tea.KeyMsg{keyMsg("n")}
		switch name {
		case "subscription edit":
			keys[0] = keyMsg("e")
		case "subscription delete":
			keys[0] = keyMsg("d")
		case "subscription filter":
			keys[0] = keyMsg("/")
		}
	case "Core edit":
		m.currentPage = layout.PageCore
		keys = []tea.KeyMsg{keyMsg("t")}
	case "settings edit":
		m.currentPage = layout.PageSettings
		keys = []tea.KeyMsg{{Type: tea.KeyEnter}}
	default:
		t.Fatalf("unknown mode %s", name)
	}
	for _, key := range keys {
		next, _ := m.Update(key)
		m = next.(Model)
	}
	return m
}

func TestPageInputCaptureBlocksGlobalNavigation(t *testing.T) {
	modes := []string{
		"connection detail", "connection TopN", "connection filter", "log detail", "log filter",
		"node results", "node filter", "rule add", "rule edit", "rule picker", "rule delete", "rule filter",
		"subscription add", "subscription edit", "subscription delete", "subscription filter", "Core edit", "settings edit",
	}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			for _, event := range []tea.Msg{
				tea.KeyMsg{Type: tea.KeyTab}, tea.KeyMsg{Type: tea.KeyShiftTab}, keyMsg("r"),
				tea.MouseMsg{X: 2, Y: 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress},
				tea.MouseMsg{X: 2, Y: 1, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress},
			} {
				m := capturedPage(t, mode)
				next, _ := m.Update(event)
				if next.(Model).currentPage != m.currentPage {
					t.Fatalf("%T escaped %s", event, mode)
				}
			}
			m := capturedPage(t, mode)
			next, _ := m.Update(keyMsg("?"))
			if next.(Model).showHelp != (mode != "Core edit") {
				t.Fatal("help policy changed")
			}
		})
	}
}

func TestDetailQClosesPageModeWithoutQuitting(t *testing.T) {
	for _, mode := range []string{"connection detail", "connection TopN", "log detail"} {
		t.Run(mode, func(t *testing.T) {
			m := capturedPage(t, mode)
			next, cmd := m.Update(keyMsg("q"))
			if cmd != nil {
				t.Fatal("q issued a global command instead of closing the detail")
			}
			got := next.(Model)
			if got.connsState.DetailMode() || got.logsState.DetailMode() || got.connsState.ToPageState(nil, 120, 32).TopNModalMode {
				t.Fatal("q did not close the page mode")
			}
			_, cmd = got.Update(keyMsg("q"))
			if cmd == nil {
				t.Fatal("ordinary q did not quit after the mode closed")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatal("ordinary q did not produce QuitMsg")
			}
		})
	}
}

func TestPageInputCapturePreservesInterruptPolicy(t *testing.T) {
	for _, tc := range []struct {
		mode string
		quit bool
	}{
		{"connection detail", true}, {"connection TopN", true}, {"log detail", true}, {"node results", true}, {"rule edit", true}, {"Core edit", true},
		{"connection filter", false}, {"log filter", false}, {"node filter", false}, {"rule add", false}, {"rule filter", false},
		{"subscription edit", false}, {"subscription filter", false}, {"settings edit", false},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			m := capturedPage(t, tc.mode)
			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
			quits := false
			if cmd != nil {
				_, quits = cmd().(tea.QuitMsg)
			}
			if quits != tc.quit {
				t.Fatalf("Ctrl+C quit=%t, want %t", quits, tc.quit)
			}
		})
	}
}

func TestPageModalWheelDoesNotMoveUnderlyingSelection(t *testing.T) {
	for _, mode := range []string{"rule delete", "subscription add", "subscription edit", "subscription delete", "subscription filter", "connection filter", "node filter", "rule filter"} {
		t.Run(mode, func(t *testing.T) {
			m := capturedPage(t, mode)
			next, _ := m.Update(tea.MouseMsg{X: 2, Y: 10, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
			got := next.(Model)
			if got.rulesState.ToPageState(120, 32).SelectedRule != 0 || got.subState.ToPageState(120, 32).Selected != 0 ||
				got.connsState.ToPageState(nil, 120, 32).SelectedIndex != 0 || got.nodesState.SelectedGroup != 0 {
				t.Fatal("modal wheel moved the underlying list")
			}
		})
	}
}

func TestPageModeEscapeReleasesInput(t *testing.T) {
	for _, mode := range []string{"connection detail", "connection TopN", "log detail", "node results", "rule add", "rule edit", "rule delete", "subscription add", "subscription edit", "subscription delete", "Core edit", "settings edit"} {
		t.Run(mode, func(t *testing.T) {
			m := capturedPage(t, mode)
			scrolled, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
			m = scrolled.(Model)
			next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			got := next.(Model)
			if got.pageInputCapture().Active {
				t.Fatal("closed mode still owns input")
			}
			if ps := got.connsState.ToPageState(nil, 120, 32); ps.SelectedConnection != nil || ps.DetailLeftScroll != 0 || ps.DetailRightScroll != 0 || ps.DetailFocusPanel != 0 || ps.TopNModalScroll != 0 {
				t.Fatal("connection detail retained temporary state")
			}
			if got.nodesState.DetailScrollTop != 0 {
				t.Fatal("node results retained scroll state")
			}
			if ps := got.rulesState.ToPageState(120, 32); ps.TypeFilterCursor != 0 || ps.DeleteTarget.Payload != "" {
				t.Fatal("rules retained a temporary selection")
			}
		})
	}
}
