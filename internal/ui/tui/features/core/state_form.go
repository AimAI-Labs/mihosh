package core

import (
	"net"
	"strconv"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func (s State) editForm(kind string) (State, tea.Cmd) {
	if s.Editing {
		return s, nil
	}
	if !s.Busy {
		s.Err, s.task = nil, ""
	}
	s.formErr = nil
	s.dialogScroll, s.formFollowFocus = 0, true
	s.formSelection, s.formScroll = s.selected, s.scroll
	s.form, s.Editing, s.choice = kind, true, 0
	s.fields, s.args = nil, nil
	s.focus = 0
	if kind == "system-proxy" || kind == "tun" {
		if (kind == "system-proxy" && s.Status.SystemProxy) || (kind == "tun" && s.Status.TUN) {
			s.choice = 1
		}
		return s, nil
	}
	if kind == "configure" || kind == "recover" {
		return s, nil
	}
	if kind == "service" {
		s.selected, s.scroll = 0, 0
		s.followSelection = true
		return s, nil
	}
	if kind != "network" {
		count := 1
		if kind == "import" || kind == "external" {
			count = 3
		}
		if kind == "ports" || kind == "profile" {
			count = 2
		}

		s.fields = make([]textinput.Model, count)
		for i := range s.fields {
			s.fields[i] = textinput.New()
			s.fields[i].CharLimit = 2048
			s.fields[i].Width = 50
		}
		switch kind {
		case "profile":
			s.fields[0].SetValue(i18n.T("core.profile_default"))
			s.fields[1].EchoMode = textinput.EchoPassword
		case "version":
			s.fields[0].SetValue(s.Release.Tag)
		case "import":
			s.fields[1].SetValue(s.Release.Tag)
		case "external":
			cfg, _ := config.Load()
			if cfg != nil {
				s.fields[0].SetValue(cfg.Core.ExternalController)
				s.fields[1].SetValue(cfg.Core.ExternalSecret)
			}
			s.fields[1].EchoMode = textinput.EchoPassword
			s.fields[2].SetValue("7890")
			if cfg != nil && cfg.Core.ExternalMixedPort > 0 {
				s.fields[2].SetValue(strconv.Itoa(cfg.Core.ExternalMixedPort))
			}
		case "ports":
			endpoint := config.ResolveMihomoEndpoint()
			s.fields[0].SetValue("9090")
			if _, port, e := net.SplitHostPort(endpoint.ExternalController); e == nil {
				s.fields[0].SetValue(port)
			}
			s.fields[1].SetValue(strconv.Itoa(endpoint.MixedPort))
		}
		s.fields[0].Focus()
		return s, textinput.Blink
	}
	cfg, err := config.Load()
	if err != nil {
		c := config.DefaultConfig
		cfg = &c
	}
	s.fields = make([]textinput.Model, 3)
	for i := range s.fields {
		s.fields[i] = textinput.New()
		s.fields[i].CharLimit = 2048
		s.fields[i].Width = 50
	}
	s.fields[0].SetValue(cfg.Core.Proxy)
	s.fields[0].EchoMode = textinput.EchoPassword
	gateways := cfg.Core.Accelerators
	if gateways == nil {
		gateways = coreinfra.DefaultAccelerators()
	}
	var urls []string
	for _, a := range gateways {
		urls = append(urls, a.URL)
	}
	s.fields[1].SetValue(strings.Join(urls, ","))
	timeout := cfg.Core.DownloadTimeoutSeconds
	if timeout == 0 {
		timeout = 600
	}
	s.fields[2].SetValue(strconv.Itoa(timeout))
	s.fields[0].Focus()
	return s, textinput.Blink
}
func (s State) updateForm(msg tea.KeyMsg) (State, tea.Cmd) {
	s.formFollowFocus = true
	if s.Busy && s.form != "network" && msg.String() == "enter" {
		return s, nil
	}
	if s.form == "system-proxy" || s.form == "tun" || s.form == "configure" || s.form == "recover" {
		switch msg.String() {
		case "esc":
			return s.closeForm(), nil
		case "left", "right", "tab", "shift+tab", "up", "down":
			if s.form == "system-proxy" || s.form == "tun" {
				s.choice = 1 - s.choice
			}
		case "enter":
			return s.submitChoice()
		}
		return s, nil
	}
	if s.form == "service" {
		switch msg.String() {
		case "esc":
			return s.closeForm(), nil
		case "up", "k":
			s.selected = max(0, s.selected-1)
			s.followSelection = true
		case "down", "j":
			s.selected = min(len(serviceActions)-1, s.selected+1)
			s.followSelection = true
		case "enter":
			return s.submitService()
		}
		return s, nil
	}
	switch msg.String() {
	case "esc":
		return s.closeForm(), nil
	case "tab", "shift+tab":
		s.fields[s.focus].Blur()
		delta := 1
		if msg.String() == "shift+tab" {
			delta = -1
		}
		s.focus = (s.focus + delta + len(s.fields)) % len(s.fields)
		return s, s.fields[s.focus].Focus()
	case "enter":
		if s.form != "network" {
			s.args = nil
			for _, f := range s.fields {
				value := strings.TrimSpace(f.Value())
				if s.form == "external" && len(s.args) == 1 {
					value = f.Value()
				}

				s.args = append(s.args, value)
			}
			action := "release"
			if s.form == "import" {
				action = "import"
			}
			if s.form == "external" {
				action = "connect"
			}
			if s.form == "ports" {
				action = "ports"
			}
			if s.form == "profile" {
				action = "profile"
			}
			args := s.args
			s = s.closeForm()
			s.args = args
			return s.begin(action)
		}
		gateways := []coreinfra.Accelerator{}
		for _, raw := range strings.Split(s.fields[1].Value(), ",") {
			if raw = strings.TrimSpace(raw); raw != "" {
				gateways = append(gateways, coreinfra.Accelerator{URL: raw, Metadata: true, Asset: true})
			}
		}
		timeout, err := strconv.Atoi(s.fields[2].Value())
		if err != nil {
			s.formErr = err
			return s, nil
		}
		err = s.svc.SaveNetwork(s.fields[0].Value(), gateways, timeout)
		s.formErr = err
		if err == nil {
			s = s.closeForm()
		}
		return s, nil
	}
	var cmd tea.Cmd
	s.fields[s.focus], cmd = s.fields[s.focus].Update(msg)
	return s, cmd
}

func (s State) submitService() (State, tea.Cmd) {
	if s.Busy {
		return s, nil
	}
	action := serviceActions[s.selected]
	s = s.closeForm()
	s.args = []string{action}
	return s.begin("service")
}

func (s State) closeForm() State {
	s.selected, s.scroll = s.formSelection, s.formScroll
	s.Editing, s.form = false, ""
	s.fields, s.args, s.formErr = nil, nil, nil
	s.focus, s.choice, s.formSelection, s.formScroll = 0, 0, 0, 0
	s.dialogScroll, s.formFollowFocus = 0, false
	return s
}

func (s State) submitChoice() (State, tea.Cmd) {
	if s.Busy {
		return s, nil
	}
	action, enabled := s.form, s.choice == 1
	s = s.closeForm()
	if action == "system-proxy" || action == "tun" {
		current := s.Status.SystemProxy
		if action == "tun" {
			current = s.Status.TUN
		}
		if enabled == current {
			return s, nil
		}
		s.args = []string{strconv.FormatBool(enabled)}
	}
	return s.begin(action)
}
