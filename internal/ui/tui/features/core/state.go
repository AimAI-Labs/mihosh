package core

import (
	"context"
	"runtime"
	"strconv"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
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
	formErr           error
	args              []string
	fields            []textinput.Model
	focus             int
	choice            int
	formSelection     int
	formScroll        int
	dialogScroll      int
	formFollowFocus   bool
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
	if !release.IsStable() {
		release = coreinfra.PinnedRelease()
	}
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
			enabled := !status.SystemProxy
			if len(args) > 0 {
				enabled, err = strconv.ParseBool(args[0])
			}
			if err == nil {
				err = svc.SetSystemProxy(ctx, enabled)
			}
		case "tun":
			enabled := !status.TUN
			if len(args) > 0 {
				enabled, err = strconv.ParseBool(args[0])
			}
			if err == nil {
				err = svc.SetTUN(ctx, enabled)
			}
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
		case "upgrade":
			err = svc.Upgrade(ctx, notify)
		case "setup":
			err = svc.Setup(ctx, notify)
		case "import":
			err = svc.Import(ctx, args[0], args[1], args[2])
		case "activate":
			err = svc.ActivateVersion(ctx, release.Version())
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
		if msg.Action == "releases" {
			if msg.Err != nil {
				s.catalog.failed = true
			} else {
				s.task = ""
			}
		}
		if msg.Release != nil {
			if msg.Action == "release" || msg.Action == "check" {
				s = s.applyReleaseLookup(*msg.Release)
			} else {
				s.Release = *msg.Release
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
		if s.Editing {
			return s.clickForm(msg)
		}
		contentHeight := max(1, msg.Height-footerHeight)
		if msg.X < 0 || msg.X >= msg.Width || msg.Y < 0 || msg.Y >= contentHeight {
			return s, nil
		}
		page := s.layout(msg.Width, contentHeight)
		y := msg.Y + s.viewportOffset(page, contentHeight)
		for _, hit := range page.tabs {
			if msg.X >= hit.x && msg.X < hit.x+hit.width && y == hit.y {
				return s.openTab(hit.index)
			}
		}
		if s.tab == 1 {
			return s.clickReleasePage(page, msg.X, msg.Y)
		}
		for _, hit := range page.actions {
			if msg.X >= hit.x && msg.X < hit.x+hit.width && y >= hit.y && y < hit.y+hit.height {
				s.selected = hit.index
				return s.choose(s.actions()[hit.index].action)
			}
		}
	case messages.PageMouseScrollMsg:
		if s.Editing {
			return s.scrollForm(msg)
		}
		if s.tab == 1 {
			return s.scrollReleasePage(msg)
		}
		contentHeight := max(1, msg.Height-footerHeight)
		page := s.layout(msg.Width, contentHeight)
		s.scroll = s.viewportOffset(page, contentHeight)
		s.followSelection = false
		if msg.Up {
			s.scroll = max(0, s.scroll-1)
		} else {
			s.scroll = min(max(0, lipgloss.Height(page.content)-contentHeight), s.scroll+1)
		}
		return s, nil
	case tea.KeyMsg:
		if !s.Editing && !s.Busy && (s.task != "" || s.Err != nil) && (msg.String() == "enter" || msg.String() == "esc") {
			s.task, s.Err = "", nil
			s.Progress = coreinfra.Progress{}
			return s, nil
		}
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
			return s.choose("system-proxy")
		case "n":
			return s.choose("tun")
		case "c":
			return s.choose("recover")
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
			return s.choose("configure")
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
func (s State) OperationID() uint64 { return s.generation }

func (s State) Leave() State {
	if s.Editing {
		s = s.closeForm()
	}
	s.args = nil
	return s
}

// InputCapture declares which input belongs to the current page mode.
func (s State) InputCapture() common.InputCapture {
	return common.InputCapture{Active: s.Editing, Help: s.Editing}
}
