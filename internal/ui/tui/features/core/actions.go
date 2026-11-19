package core

import (
	"slices"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type actionItem struct {
	key, action string
}

var tabs = []string{"advanced", "versions", "advanced", "geo", "mihomo"}
var tabOrder = []int{0, 1, 3, 4}
var serviceActions = []string{"install", "start", "stop", "restart", "enable", "disable", "boot", "uninstall"}

func (s State) isAdvancedPage() bool { return s.tab == 0 || s.tab == 2 }

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
	items = append(items, actionItem{"h", "service"}, actionItem{"e", "network"}, actionItem{"y", "system-proxy"}, actionItem{"w", "shell-proxy"})
	if s.Status.Mode == "managed" || s.Status.RecoveryPending {
		items = append(items, actionItem{"c", "recover"})
	}
	if s.Status.Mode == "managed" {
		items = append(items, actionItem{"n", "tun"}, actionItem{"", "basic-config"})
	} else {
		items = append(items, actionItem{"m", "configure"})
	}
	if s.connected {
		apiActions := []string{"reload", "flush_dns", "flush_fakeip"}
		if s.Status.Mode != "managed" {
			apiActions = append(apiActions, "restart", "upgrade_auto", "upgrade_release", "upgrade_alpha", "update_geo")
		}
		for _, id := range apiActions {
			items = append(items, actionItem{"", "api:" + id})
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
	case "network", "version", "import", "external", "service", "configure", "recover", "basic-config", "stop", "restart", "api:restart", "api:update_geo":
		return s.editForm(action)
	default:
		if id, ok := strings.CutPrefix(action, "api:"); ok {
			return s.runCoreAPIAction(id)
		}
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

func (s State) stepTab(delta int) (State, tea.Cmd) {
	index := max(0, slices.Index(tabOrder, s.tab))
	return s.openTab(tabOrder[(index+delta+len(tabOrder))%len(tabOrder)])
}

func (s State) runCoreAPIAction(id string) (State, tea.Cmd) {
	if s.Busy || !s.connected {
		return s, nil
	}
	var cmd tea.Cmd
	s.editor, cmd = s.editor.RunCoreAction(id)
	return s, cmd
}
