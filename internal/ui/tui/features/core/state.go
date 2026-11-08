package core

import (
	"context"
	"net"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type State struct {
	svc        *service.CoreService
	Status     service.CoreStatus
	Release    coreinfra.Release
	Progress   coreinfra.Progress
	Err        error
	Busy       bool
	Editing    bool
	form       string
	args       []string
	fields     []textinput.Model
	focus      int
	scroll     int
	cancel     context.CancelFunc
	updates    chan coreinfra.Progress
	generation uint64
}

func NewState(svc *service.CoreService) State {
	release := svc.CachedRelease()
	if asset, err := release.SelectAsset(runtime.GOOS, runtime.GOARCH); err == nil {
		release.Assets = []coreinfra.Asset{asset}
	}
	return State{svc: svc, Release: release}
}
func (s State) Refresh() tea.Cmd {
	return func() tea.Msg { status, err := s.svc.Status(); return messages.CoreStatusMsg{Status: status, Err: err} }
}
func (s *State) Close() {
	if s.cancel != nil {
		s.cancel()
	}
}
func (s State) poll() tea.Cmd {
	id := s.generation
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return messages.CoreProgressTickMsg{ID: id} })
}
func (s State) begin(action string) (State, tea.Cmd) {
	if s.Busy {
		return s, nil
	}
	s.Busy = true
	s.Err = nil
	s.Progress = coreinfra.Progress{}
	s.generation++
	id := s.generation
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	updates := make(chan coreinfra.Progress, 1)
	s.updates = updates
	notify := func(p coreinfra.Progress) {
		select {
		case updates <- p:
		default:
			select {
			case <-updates:
			default:
			}
			select {
			case updates <- p:
			default:
			}
		}
	}
	svc := s.svc
	release := s.Release
	args := append([]string(nil), s.args...)
	return s, tea.Batch(s.poll(), func() tea.Msg {
		defer cancel()
		var err error
		var result *coreinfra.Release
		switch action {
		case "check", "release":
			tag := "latest"
			if action == "release" && len(args) > 0 {
				tag = args[0]
			}
			r, e := svc.Release(ctx, tag, notify)
			err = e
			if e == nil {
				asset, selectErr := r.SelectAsset(runtime.GOOS, runtime.GOARCH)
				if selectErr != nil {
					err = selectErr
				} else {
					r.Assets = []coreinfra.Asset{asset}
					result = &r
				}
			}
		case "profile":
			err = svc.ImportProfile(ctx, args[0], args[1], true)
		case "upgrade":
			err = svc.Upgrade(ctx, notify)
		case "setup":
			err = svc.Setup(ctx, notify)
		case "import":
			err = svc.Import(ctx, args[0], args[1], args[2])
		case "activate":
			err = svc.ActivateVersion(ctx, release.Tag)
		case "rollback":
			err = svc.ActivateVersion(ctx, "")
		case "restart":
			err = svc.Restart(ctx)
		case "ports":
			controller, e := strconv.Atoi(args[0])
			if e != nil {
				err = e
				break
			}
			mixed, e := strconv.Atoi(args[1])
			if e != nil {
				err = e
				break
			}
			err = svc.SetPorts(controller, mixed)
		case "connect":
			mixed, e := strconv.Atoi(args[2])
			if e != nil {
				err = e
				break
			}
			if args[0] == "" {
				err = svc.UseExternal()
			} else {
				err = svc.ConnectExternal(args[0], args[1], mixed)
			}
		case "external":
			err = svc.UseExternal()
		case "install":
			err = svc.Install(ctx, release, notify)
		case "configure":
			err = svc.Configure()
		case "start":
			err = svc.Start(ctx)
		case "stop":
			err = svc.Stop()
		}
		return messages.CoreOperationMsg{ID: id, Release: result, Err: err, Action: action}
	})
}
func (s State) Update(msg tea.Msg) (State, tea.Cmd) {
	switch msg := msg.(type) {
	case messages.CoreConnectionMsg:
		s.Err = msg.Err
		return s, nil
	case messages.CoreStatusMsg:
		s.Status = msg.Status
		if msg.Err != nil {
			s.Err = msg.Err
		}
		return s, nil
	case messages.CoreProgressTickMsg:
		if !s.Busy || msg.ID != s.generation {
			return s, nil
		}
		select {
		case s.Progress = <-s.updates:
		default:
		}
		return s, s.poll()
	case messages.CoreOperationMsg:
		if msg.ID != s.generation {
			return s, nil
		}
		s.Busy = false
		s.cancel = nil
		s.Err = msg.Err
		if msg.Release != nil {
			s.Release = *msg.Release
		}
		return s, s.Refresh()
	case messages.PageMouseClickMsg:
		if s.Editing {
			return s, nil
		}
		originX, originY := actionOrigin(msg.Width)
		row := msg.Y + s.effectiveScroll(msg.Width, msg.Height) - originY
		if msg.X >= originX && row >= 0 && row < len(actions) {
			action := actions[row].action
			if action == "network" || action == "version" || action == "import" || action == "ports" || action == "external" || action == "profile" {
				return s.editForm(action)
			}
			return s.begin(action)
		}
	case messages.PageMouseScrollMsg:
		if msg.Up {
			s.scroll = max(0, s.scroll-1)
		} else {
			s.scroll = min(24, s.scroll+1)
		}
		return s, nil
	case tea.KeyMsg:
		if s.Editing {
			return s.updateForm(msg)
		}
		switch msg.String() {
		case "up", "k":
			s.scroll = max(0, s.scroll-1)
			return s, nil
		case "down", "j":
			s.scroll = min(24, s.scroll+1)
			return s, nil
		case "esc":
			if s.cancel != nil {
				s.cancel()
			}
			return s, nil
		case "enter":
			return s.begin("setup")
		case "t":
			return s.editForm("version")
		case "l":
			return s.editForm("import")
		case "o":
			return s.editForm("profile")
		case "g":
			return s.begin("upgrade")
		case "a":
			return s.begin("activate")
		case "b":
			return s.begin("rollback")
		case "z":
			return s.begin("restart")
		case "u":
			return s.editForm("external")
		case "p":
			return s.editForm("ports")
		case "v":
			return s.begin("check")
		case "i":
			return s.begin("install")
		case "m":
			return s.begin("configure")
		case "s":
			return s.begin("start")
		case "x":
			return s.begin("stop")
		case "e":
			return s.editForm("network")
		}
	}
	return s, nil
}
func (s State) editForm(kind string) (State, tea.Cmd) {
	if kind != "network" {
		if s.Busy {
			return s, nil
		}
		s.form = kind
		s.Editing = true
		s.focus = 0
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
	s.form = "network"
	if s.Busy {
		return s, nil
	}
	cfg, err := config.Load()
	if err != nil {
		c := config.DefaultConfig
		cfg = &c
	}
	s.Editing = true
	s.focus = 0
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
	switch msg.String() {
	case "esc":
		s.Editing = false
		s.fields = nil
		return s, nil
	case "tab", "shift+tab":
		s.fields[s.focus].Blur()
		s.focus = (s.focus + 1) % len(s.fields)
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
			s.Editing = false
			s.fields = nil
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
			s.Err = err
			return s, nil
		}
		err = s.svc.SaveNetwork(s.fields[0].Value(), gateways, timeout)
		s.Err = err
		if err == nil {
			s.Editing = false
			s.fields = nil
		}
		return s, nil
	}
	var cmd tea.Cmd
	s.fields[s.focus], cmd = s.fields[s.focus].Update(msg)
	return s, cmd
}

// effectiveScroll mirrors Render's viewport so clicks address rendered action rows.
func (s State) effectiveScroll(width, height int) int {
	extra := 0
	if s.Busy {
		extra += 2
	}
	if s.Err != nil {
		extra++
	}
	total := 8 + len(actions) + 2 + extra
	if width >= 100 {
		total = max(2+len(actions), 10+extra)
	}
	return min(s.scroll, max(0, total-height))
}

func (s State) OperationID() uint64 { return s.generation }

func (s State) Leave() State { s.Editing = false; s.fields = nil; s.args = nil; return s }
