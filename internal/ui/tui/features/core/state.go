package core

import (
	"context"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/preferences"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type State struct {
	serviceLog          messages.CoreServiceLogMsg
	serviceScroll       int
	serviceFocus        bool
	pendingServiceFocus bool
	serviceActionIdx    int
	svc                 *service.CoreService
	Status              service.CoreStatus
	Release             service.Release
	Progress            service.Progress
	Err                 error
	Busy                bool
	Editing             bool
	form                string
	formErr             error
	args                []string
	fields              []textinput.Model
	focus               int
	choice              int
	formSelection       int
	formScroll          int
	dialogScroll        int
	formFollowFocus     bool
	viewWidth           int
	viewHeight          int
	scroll              int
	tab                 int
	selected            int
	followSelection     bool
	connected           bool
	externalVersion     string
	connectionChecked   bool
	task                string
	cancel              context.CancelFunc
	updates             chan service.Progress
	generation          uint64
	catalog             releaseCatalog
	geoFiles            []service.GeoDataFile
	geoErr              error
	shellPath           string
	editor              preferences.State
}

func NewState(svc *service.CoreService, configSvc *service.ConfigService, client *service.Client) State {
	if configSvc == nil {
		configSvc = service.NewConfigService()
	}
	release := svc.CachedRelease()
	if !release.IsStable() {
		release = service.PinnedRelease()
	}
	if asset, err := release.SelectAsset(runtime.GOOS, runtime.GOARCH); err == nil {
		release.Assets = []service.Asset{asset}
	}
	var status service.Status
	if cfg, err := service.Load(); err == nil && cfg != nil {
		status.ShellProxy = cfg.ShellProxy
	}
	return State{svc: svc, Status: status, editor: preferences.NewMihomoState(configSvc, client, svc), Release: release, followSelection: true, catalog: releaseCatalog{page: service.ReleasePage{Releases: []service.Release{release}}, cleanup: &releaseCleanup{}}}
}
func (s State) Refresh() tea.Cmd {
	if s.svc == nil {
		return nil
	}
	status := func() tea.Msg { status, err := s.svc.Status(); return messages.CoreStatusMsg{Status: status, Err: err} }
	if s.isDownloadsPage() {
		return tea.Batch(status, s.refreshGeoData())
	}
	if s.tab == 4 {
		return tea.Batch(status, s.editor.Refresh(), preferences.FetchManagedSysStatusCmd())
	}
	if s.isAdvancedPage() && runtime.GOOS == "linux" && s.Status.Mode != "managed" {
		return tea.Batch(status, preferences.FetchSysStatusCmd())
	}
	return status
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
	s.Progress = service.Progress{}
	s.generation++
	id := s.generation
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	updates := make(chan service.Progress, 1)
	s.updates = updates
	notify := func(p service.Progress) {
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
	forceRefresh := s.catalog.forceRefresh
	return s, tea.Batch(s.poll(), func() tea.Msg {
		defer cancel()
		var err error
		var result *service.Release
		var shellPath string
		var releases *service.ReleasePage
		switch action {
		case "releases":
			var (
				page service.ReleasePage
				e    error
			)
			if forceRefresh {
				page, e = svc.RefreshReleases(ctx, releasePage, notify)
			} else {
				page, e = svc.Releases(ctx, releasePage, notify)
			}
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
					r.Assets = []service.Asset{asset}
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
		case "geo-update":
			err = svc.UpdateGeoData(ctx, args[0], notify)
		case "shell-proxy":
			enabled := !status.ShellProxy
			if len(args) > 0 {
				enabled, err = strconv.ParseBool(args[0])
			}
			if err == nil {
				shellPath, err = service.SaveShellProxy(enabled)
			}
		case "configure":
			err = svc.Configure()
		case "basic-config":
			_, err = svc.UseBasicConfig(ctx)
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
		return messages.CoreOperationMsg{ID: id, Release: result, Releases: releases, Err: err, Action: resultAction, ShellPath: shellPath}
	})
}
func (s State) Update(msg tea.Msg) (State, tea.Cmd) {
	switch msg := msg.(type) {
	case messages.PageResizeMsg:
		s.viewWidth, s.viewHeight = msg.Width, msg.Height
		return s, nil
	case messages.MihomoConfigMsg:
		s.editor = s.editor.ApplyMihomoConfig(&msg)
		return s, nil
	case messages.MihomoOverridesMsg:
		s.editor = s.editor.ApplyMihomoOverrides(&msg)
		return s, nil
	case messages.MihomoOverridesSavedMsg:
		s.editor = s.editor.CompleteMihomoOverridesSave(&msg)
		return s, nil
	case messages.MihomoVersionMsg:
		s.editor = s.editor.ApplyMihomoVersion(msg.Version)
		return s, nil
	case messages.CoreTargetChangedMsg:
		if msg.Reset || msg.Invalidate {
			s.editor = s.editor.ClearActionStates()
		}
		return s, nil
	case messages.CoreActionDoneMsg, messages.CoreActionErrorMsg:
		s.editor = s.editor.ClearActionStates()
		return s, nil
	case messages.SysStatusResultMsg:
		s.editor.SysStatus.Update(msg)
		return s, nil
	case messages.CoreConnectionMsg:
		s.connected = msg.Err == nil
		s.externalVersion = msg.Version
		s.connectionChecked = true
		return s, nil
	case messages.CoreStatusMsg:
		s.Status = msg.Status
		if s.tab == 4 && s.pendingServiceFocus && s.hasServiceCard() {
			s.serviceFocus = true
			s.pendingServiceFocus = false
			s.serviceActionIdx = 0
		}
		s = s.clampServiceSelection()
		if !s.Editing {
			s.selected = min(s.selected, s.selectionCount()-1)
		}
		if msg.Err != nil {
			s.Err = msg.Err
		}
		return s, nil
	case messages.CoreServiceLogMsg:
		s.serviceLog = msg
		return s, nil
	case messages.CoreGeoDataMsg:
		s.geoFiles, s.geoErr = msg.Files, msg.Err
		if s.tab == 3 && !s.Editing {
			s.selected = max(0, min(s.selected, s.selectionCount()-1))
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
		if msg.Action == "shell-proxy" && msg.Err == nil {
			s.shellPath = msg.ShellPath
			if cfg, err := service.Load(); err == nil && cfg != nil {
				s.Status.ShellProxy = cfg.ShellProxy
			}
		}
		if (msg.Action == "system-proxy" || msg.Action == "tun" || msg.Action == "tun-offline") && msg.Err == nil {
			if cfg, err := service.Load(); err == nil && cfg != nil {
				s.Status.SystemProxy = cfg.Core.SystemProxy
				s.Status.TUN = cfg.Core.TUN
			}
		}
		if msg.Action == "releases" {
			if msg.Err != nil {
				s.catalog.failed = true
			} else {
				s.task = ""
				s.catalog.forceRefresh = false
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
		if s.cancelDownloadAt(msg.X, msg.Y, msg.Width, msg.Height) {
			s.cancel()
			return s, nil
		}
		contentHeight := max(1, msg.Height-footerHeight)
		if msg.X < 0 || msg.X >= msg.Width || msg.Y < 0 || msg.Y >= contentHeight {
			return s, nil
		}
		page := s.layout(msg.Width, contentHeight)
		y := msg.Y
		if y >= page.pinnedRows {
			y += s.viewportOffset(page, contentHeight)
		}
		for _, hit := range page.tabs {
			if msg.X >= hit.x && msg.X < hit.x+hit.width && y == hit.y {
				return s.openTab(hit.index)
			}
		}
		if s.isDownloadsPage() {
			for _, control := range page.controls {
				hit := control.actionHit
				if msg.X >= hit.x && msg.X < hit.x+hit.width && y == hit.y {
					return s.editForm(control.action)
				}
			}
			for _, hit := range page.actions {
				if msg.X >= hit.x && msg.X < hit.x+hit.width && y >= hit.y && y < hit.y+hit.height {
					if s.tab != 3 {
						s = s.selectTab(3)
					}
					s.selected, s.followSelection = hit.index, true
					return s.choose("geo-update")
				}
			}
			return s.clickReleasePage(page, msg.X, y)
		}
		if s.tab == 4 {
			for _, control := range page.controls {
				hit := control.actionHit
				if hit.height > 0 && msg.X >= hit.x && msg.X < hit.x+hit.width && y >= hit.y && y < hit.y+hit.height {
					if action, ok := strings.CutPrefix(control.action, "service:"); ok {
						s.editor = s.editor.CancelEdit()
						s.serviceFocus = true
						s.pendingServiceFocus = false
						s.serviceActionIdx = hit.index
						s = s.clampServiceSelection()
						s.args = []string{action}
						return s.begin("service")
					}
				}
			}
			if s.hasServiceCard() && y >= page.systemdY {
				s.editor = s.editor.CancelEdit()
				s.serviceFocus = true
				s.pendingServiceFocus = false
				s = s.clampServiceSelection()
				return s, nil
			}
			s.serviceFocus = false
			s.pendingServiceFocus = false
			msg.Y = y
			return s.updateEditorMouse(msg, page.editorY)
		}
		for _, control := range page.controls {
			hit := control.actionHit
			if hit.height > 0 && msg.X >= hit.x && msg.X < hit.x+hit.width && y >= hit.y && y < hit.y+hit.height {
				if action, ok := strings.CutPrefix(control.action, "switch:"); ok {
					for i, item := range s.actions() {
						if item.action == action {
							s.selected = i
							break
						}
					}
					return s.setSwitchAction(action, hit.index == 1)
				}
			}
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
		if s.isAdvancedPage() {
			height := max(1, msg.Height-footerHeight)
			page := s.layout(msg.Width, height)
			if msg.X < 0 || msg.X >= msg.Width || msg.Y < page.pinnedRows || msg.Y >= height {
				return s, nil
			}
			delta := 1
			if msg.Up {
				delta = -1
			}
			offset := s.viewportOffset(page, height)
			limit := max(0, lipgloss.Height(page.content)-height)
			if scroll := min(max(0, offset+delta), limit); scroll != offset {
				s.scroll, s.followSelection = scroll, false
			}
			return s, nil
		}
		if s.isDownloadsPage() {
			height := max(1, msg.Height-footerHeight)
			page := s.layout(msg.Width, height)
			if msg.X < 0 || msg.X >= msg.Width || msg.Y < page.pinnedRows || msg.Y >= height {
				return s, nil
			}
			y := msg.Y + s.viewportOffset(page, height)
			geo := page.geoBounds
			if msg.X >= geo.x && y >= geo.y && y < geo.y+geo.height {
				if s.tab != 3 {
					s = s.selectTab(3)
				}
			} else if s.tab == 3 && (y < geo.y || msg.X < geo.x) {
				s = s.selectTab(1)
			}
			if s.tab == 1 {
				return s.scrollReleasePage(msg)
			}
		}
		if s.tab == 4 {
			height := max(1, msg.Height-footerHeight)
			page := s.layout(msg.Width, height)
			overflow := max(0, lipgloss.Height(page.content)-height)
			y := msg.Y + s.viewportOffset(page, height)
			delta := 1
			if msg.Up {
				delta = -1
			}
			if !s.editor.IsEditing() && s.Status.ServiceUnit != "" && (y >= page.systemdY || overflow > 0) {
				if !msg.Up && s.scroll < overflow {
					s.scroll = min(overflow, s.scroll+1)
					return s, nil
				}
				current := max(0, min(s.serviceScroll, page.serviceMaxScroll))
				if msg.Up && current == 0 && s.scroll > 0 {
					s.scroll = max(0, s.scroll-1)
					return s, nil
				}
				s.serviceScroll = max(0, min(current+delta, page.serviceMaxScroll))
				return s, nil
			}
			if overflow <= 0 {
				return s.updateEditorMouse(msg, page.editorY)
			}
			if !s.editor.IsEditing() {
				s.scroll = max(0, min(s.scroll+delta, overflow))
			}
			return s, nil
		}
		if msg.Up {
			s.selected = max(0, s.selected-1)
		} else {
			s.selected = min(s.selectionCount()-1, s.selected+1)
		}
		s.followSelection = true
		return s, nil
	case tea.KeyMsg:
		if s.tab == 4 && !s.Editing {
			if s.serviceFocus && !s.hasServiceCard() {
				s.serviceFocus = false
			}
			if s.serviceFocus && !s.editor.IsEditing() {
				return s.updateServiceCardKey(msg)
			}
			if !s.editor.IsEditing() && s.hasServiceCard() {
				switch msg.String() {
				case "h", "tab", "shift+tab":
					s.serviceFocus = true
					s.pendingServiceFocus = false
					s = s.clampServiceSelection()
					return s, nil
				case "down", "j":
					if s.editor.SelectedSettingIndex() == 5 {
						s.serviceFocus = true
						s.pendingServiceFocus = false
						s = s.clampServiceSelection()
						return s, nil
					}
				}
			}
			if s.editor.IsEditing() || !isCoreTabKey(msg.String()) {
				if s.Busy || s.editor.IsSaving() {
					return s, nil
				}
				if !s.editor.IsEditing() && msg.String() == "r" {
					return s, s.Refresh()
				}
				s.pendingServiceFocus = false
				var cmd tea.Cmd
				s.editor, cmd = s.editor.Update(msg)
				return s, cmd
			}
		}
		if !s.Editing && !s.Busy && (s.task != "" || s.Err != nil) && (msg.String() == "enter" || msg.String() == "esc") {
			s.task, s.Err = "", nil
			s.Progress = service.Progress{}
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
		case "1", "2", "3", "4", "5":
			return s.openTab(int(msg.String()[0] - '1'))
		case "left":
			return s.stepTab(-1)
		case "right":
			return s.stepTab(1)
		case "h":
			return s.openServiceStatus()
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
			if items := s.actions(); len(items) > 0 {
				return s.choose(items[min(s.selected, len(items)-1)].action)
			}
		case " ":
			if items := s.actions(); len(items) > 0 {
				action := items[min(s.selected, len(items)-1)].action
				if _, isSwitch := s.switchValue(action); isSwitch {
					return s.choose(action)
				}
			}
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
			return s.choose("restart")
		case "u":
			return s.editForm("external")
		case "p":
			return s.openTab(4)
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
			return s.choose("stop")
		case "e":
			if s.isDownloadsPage() {
				return s.editForm("network")
			}
		case "w":
			return s.choose("shell-proxy")
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
	s.serviceFocus, s.pendingServiceFocus, s.serviceActionIdx = false, false, 0
	s.editor = s.editor.CancelEdit()
	return s
}

// InputCapture declares which input belongs to the current page mode.
func (s State) InputCapture() common.InputCapture {
	editing := s.tab == 4 && s.editor.IsEditing()
	return common.InputCapture{
		Active:    s.Editing || editing,
		Help:      s.Editing || editing,
		Interrupt: editing,
		Tab:       s.IsMihomoPage() && s.hasServiceCard(),
	}
}
