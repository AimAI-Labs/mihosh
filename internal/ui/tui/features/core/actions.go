package core

import (
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
)

type actionItem struct {
	key, action string
}

var tabs = []string{"daily", "versions", "advanced", "geo", "mihomo"}
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
		return []actionItem{{"e", "network"}, {"h", "service"}, {"y", "system-proxy"}, {"n", "tun"}, {"m", "configure"}, {"c", "recover"}, {"w", "shell-proxy"}}
	case 3:
		items := make([]actionItem, len(s.geoFiles))
		for i := range items {
			items[i] = actionItem{"enter", "geo-update"}
		}
		return items
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

func (s State) switchValue(action string) (bool, bool) {
	switch action {
	case "system-proxy":
		return s.Status.SystemProxy, true
	case "tun":
		return s.Status.TUN, true
	case "shell-proxy":
		return s.Status.ShellProxy, true
	default:
		return false, false
	}
}

func (s State) setSwitchAction(action string, target bool) (State, tea.Cmd) {
	current, ok := s.switchValue(action)
	if !ok || target == current {
		return s, nil
	}
	s.args = []string{strconv.FormatBool(target)}
	return s.begin(action)
}

func (s State) toggleSwitch(action string) (State, tea.Cmd) {
	current, ok := s.switchValue(action)
	if !ok {
		return s, nil
	}
	return s.setSwitchAction(action, !current)
}

func (s State) choose(action string) (State, tea.Cmd) {
	switch action {
	case "geo-update":
		if s.selected < 0 || s.selected >= len(s.geoFiles) || !s.geoFiles[s.selected].Updatable {
			return s, nil
		}
		return s.editForm(action)
	case "system-proxy", "tun", "shell-proxy":
		return s.toggleSwitch(action)
	case "network", "version", "import", "external", "service", "configure", "recover":
		return s.editForm(action)
	default:
		return s.begin(action)
	}
}

func (s State) selectTab(tab int) State {
	s.editor = s.editor.CancelEdit()
	s.tab = (tab + len(tabs)) % len(tabs)
	s.selected, s.scroll = 0, 0
	s.followSelection = true
	return s
}
