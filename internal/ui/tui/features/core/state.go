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
	"github.com/charmbracelet/lipgloss"
)

type State struct {
	svc               *service.CoreService
	Status            service.CoreStatus
	Release           coreinfra.Release
	Progress          coreinfra.Progress
	Err               error
	Busy              bool
	Editing           bool
	form              string
	args              []string
	fields            []textinput.Model
	focus             int
	scroll            int
	tab               int
	selected          int
	followSelection   bool
	connected         bool
	externalVersion   string
	connectionChecked bool
	task              string
	cancel            context.CancelFunc
	updates           chan coreinfra.Progress
	generation        uint64
	catalog           releaseCatalog
}

func NewState(svc *service.CoreService) State {
	release := svc.CachedRelease()
	if asset, err := release.SelectAsset(runtime.GOOS, runtime.GOARCH); err == nil {
		release.Assets = []coreinfra.Asset{asset}
	}
	return State{svc: svc, Release: release, followSelection: true, catalog: releaseCatalog{page: coreinfra.ReleasePage{Releases: []coreinfra.Release{release}}, cleanup: &releaseCleanup{}}}
}
func (s State) Refresh() tea.Cmd {
	return func() tea.Msg { status, err := s.svc.Status(); return messages.CoreStatusMsg{Status: status, Err: err} }
}
func (s *State) Close() {
	if s.catalog.cleanup != nil {
		s.catalog.cleanup.store.close()
	} else {
		s.catalog.store.close()
	}
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
	s.task = action
	s.scroll = 0
	if action != "releases" || !s.catalog.loaded {
		s.followSelection = false
	}
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
	s.args = nil
	status := s.Status
	releasePage := s.catalog.requested
	return s, tea.Batch(s.poll(), func() tea.Msg {
		defer cancel()
		var err error
		var result *coreinfra.Release
		var releases *coreinfra.ReleasePage
		switch action {
		case "releases":
			page, e := svc.Releases(ctx, releasePage, notify)
			err = e
			if e == nil {
				releases = &page
			}
		case "service":
			err = svc.ServiceAction(ctx, args[0])
		case "system-proxy":
			err = svc.SetSystemProxy(ctx, !status.SystemProxy)
		case "tun":
			err = svc.SetTUN(ctx, !status.TUN)
		case "recover":
			err = svc.RecoverNetwork(ctx)
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
			err = svc.StopContext(ctx)
		}
		resultAction := action
		if action == "service" {
			resultAction += "-" + args[0]
		}
		if action == "tun" && !status.Running {
			resultAction = "tun-offline"
		}
		return messages.CoreOperationMsg{ID: id, Release: result, Releases: releases, Err: err, Action: resultAction}
	})
}
func (s State) Update(msg tea.Msg) (State, tea.Cmd) {
	switch msg := msg.(type) {
	case messages.CoreConnectionMsg:
		s.connected = msg.Err == nil
		s.externalVersion = msg.Version
		s.connectionChecked = true
		return s, nil
	case messages.CoreStatusMsg:
		s.Status = msg.Status
		if !s.Editing {
			s.selected = min(s.selected, s.selectionCount()-1)
		}
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
		if msg.Action == "releases" && msg.Err != nil {
			s.catalog.failed = true
		}
		if msg.Err != nil {
			s.scroll, s.followSelection = 0, false
		}
		if msg.Release != nil {
			s.Release = *msg.Release
			if msg.Action == "release" || msg.Action == "check" {
				s = s.applyReleaseLookup(*msg.Release)
			}
		}
		if msg.Releases != nil {
			before := s.releaseCount()
			s = s.applyReleasePage(*msg.Releases)
			if s.Err == nil && s.catalog.page.HasNext && (len(msg.Releases.Releases) == 0 || (s.catalog.resume && s.releaseCount() == before)) {
				next, cmd := s.releaseAction("next")
				return next, tea.Batch(cmd, s.Refresh())
			}
		}
		return s, s.Refresh()
	case messages.PageMouseClickMsg:
		if msg.X < 0 || msg.X >= msg.Width || msg.Y < 0 || msg.Y >= msg.Height-1 {
			return s, nil
		}
		if s.Editing && s.form != "service" {
			return s, nil
		}
		page := s.layout(msg.Width, max(1, msg.Height-1))
		y := msg.Y + s.viewportOffset(page, max(1, msg.Height-1))
		if !s.Editing {
			for _, hit := range page.tabs {
				if msg.X >= hit.x && msg.X < hit.x+hit.width && y == hit.y {
					return s.openTab(hit.index)
				}
			}
		}
		if !s.Editing && s.tab == 1 {
			return s.clickReleasePage(page, msg.X, msg.Y)
		}
		for _, hit := range page.actions {
			if msg.X >= hit.x && msg.X < hit.x+hit.width && y >= hit.y && y < hit.y+hit.height {
				s.selected = hit.index
				if s.Editing {
					return s.submitService()
				}
				return s.choose(s.actions()[hit.index].action)
			}
		}
	case messages.PageMouseScrollMsg:
		if s.Editing && s.form != "service" {
			return s, nil
		}
		if !s.Editing && s.tab == 1 {
			return s.scrollReleasePage(msg)
		}
		page := s.layout(msg.Width, max(1, msg.Height-1))
		s.scroll = s.viewportOffset(page, max(1, msg.Height-1))
		s.followSelection = false
		if msg.Up {
			s.scroll = max(0, s.scroll-1)
		} else {
			s.scroll = min(max(0, lipgloss.Height(page.content)-max(1, msg.Height-1)), s.scroll+1)
		}
		return s, nil
	case tea.KeyMsg:
		if s.Editing {
			return s.updateForm(msg)
		}
		if s.tab == 1 {
			if next, cmd, handled := s.updateReleaseKeys(msg); handled {
				return next, cmd
			}
		}
		switch msg.String() {
		case "1", "2", "3":
			return s.openTab(int(msg.String()[0] - '1'))
		case "left":
			return s.openTab(s.tab - 1)
		case "right":
			return s.openTab(s.tab + 1)
		case "h":
			return s.editForm("service")
		case "y":
			return s.begin("system-proxy")
		case "n":
			return s.begin("tun")
		case "c":
			return s.begin("recover")
		case "up", "k":
			s.selected = max(0, s.selected-1)
			s.followSelection = true
			return s, nil
		case "down", "j":
			s.selected = min(s.selectionCount()-1, s.selected+1)
			s.followSelection = true
			return s, nil
		case "esc":
			if s.cancel != nil {
				s.cancel()
			}
			return s, nil
		case "enter":
			return s.choose(s.actions()[min(s.selected, s.selectionCount()-1)].action)
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
			s = s.selectTab(1)
			return s.releaseAction("refresh")
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
	if s.Busy {
		return s, nil
	}
	s.Err = nil
	if kind == "service" {
		s.form, s.Editing = kind, true
		s.selected, s.scroll = 0, 0
		s.followSelection = true
		return s, nil
	}
	if kind != "network" {
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
	if s.form == "service" {
		switch msg.String() {
		case "esc":
			s.Editing = false
			s.selected, s.scroll = 0, 0
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
		s.Editing = false
		s.fields = nil
		return s, nil
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

func (s State) submitService() (State, tea.Cmd) {
	s.args = []string{serviceActions[s.selected]}
	s.Editing = false
	s.selected, s.scroll = 0, 0
	return s.begin("service")
}

func (s State) OperationID() uint64 { return s.generation }

func (s State) Leave() State {
	if s.Editing && s.form == "service" {
		s.selected, s.scroll = 0, 0
	}
	s.Editing = false
	s.fields = nil
	s.args = nil
	return s
}
