package core

import (
	"strconv"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
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
	if kind == "geo-update" {
		if s.selected >= 0 && s.selected < len(s.geoFiles) {
			s.args = []string{s.geoFiles[s.selected].Key}
		}
		return s, nil
	}
	if isChoiceForm(kind) {
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

		s.fields = make([]textinput.Model, count)
		for i := range s.fields {
			s.fields[i] = textinput.New()
			s.fields[i].CharLimit = 2048
			s.fields[i].Width = 50
		}
		switch kind {
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

func isChoiceForm(kind string) bool {
	switch kind {
	case "configure", "recover", "geo-update", "basic-config", "stop", "restart", "api:restart", "api:update_geo":
		return true
	}
	return false
}

func (s State) updateForm(msg tea.KeyMsg) (State, tea.Cmd) {
	s.formFollowFocus = true
	if s.Busy && s.form != "network" && msg.String() == "enter" {
		return s, nil
	}
	if isChoiceForm(s.form) {
		switch msg.String() {
		case "esc":
			return s.closeForm(), nil
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
	action := s.form
	args := append([]string(nil), s.args...)
	s = s.closeForm()
	if action == "geo-update" {
		s.args = args
	}
	if id, ok := strings.CutPrefix(action, "api:"); ok {
		return s.runCoreAPIAction(id)
	}
	return s.begin(action)
}
