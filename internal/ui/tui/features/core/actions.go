package core

import tea "github.com/charmbracelet/bubbletea"

type actionItem struct {
	key, action string
}

var tabs = []string{"daily", "versions", "advanced"}
var serviceActions = []string{"install", "start", "stop", "restart", "enable", "disable", "boot", "uninstall"}

func (s State) primaryAction() actionItem {
	switch {
	case s.Status.Mode != "managed" && s.connected:
		return actionItem{"u", "external"}
	case s.Status.Mode != "managed" || s.Status.Version == "":
		return actionItem{"enter", "setup"}
	case !s.Status.Running:
		return actionItem{"s", "start"}
	default:
		return actionItem{"x", "stop"}
	}
}

func (s State) actions() []actionItem {
	switch s.tab {
	case 1:
		return []actionItem{{"v", "check"}, {"g", "upgrade"}, {"b", "rollback"}, {"t", "version"}, {"i", "install"}, {"a", "activate"}, {"l", "import"}}
	case 2:
		return []actionItem{{"e", "network"}, {"h", "service"}, {"y", "system-proxy"}, {"n", "tun"}, {"p", "ports"}, {"m", "configure"}, {"c", "recover"}}
	}
	primary := s.primaryAction()
	items := []actionItem{primary}
	run := actionItem{"s", "start"}
	if s.Status.Running {
		run = actionItem{"x", "stop"}
	}
	others := []actionItem{{"enter", "setup"}, {"u", "external"}}
	if s.Status.Mode == "managed" && s.Status.Version != "" {
		others = []actionItem{run}
		if s.Status.Running {
			others = append(others, actionItem{"z", "restart"})
		}
		others = append(others, actionItem{"u", "external"})
	}
	for _, item := range others {
		if item.action != primary.action {
			items = append(items, item)
		}
	}
	return items
}

func (s State) choose(action string) (State, tea.Cmd) {
	switch action {
	case "network", "version", "import", "ports", "external", "service", "system-proxy", "tun", "configure", "recover":
		return s.editForm(action)
	default:
		return s.begin(action)
	}
}

func (s State) selectTab(tab int) State {
	s.tab = (tab + len(tabs)) % len(tabs)
	s.selected, s.scroll = 0, 0
	s.followSelection = true
	return s
}
