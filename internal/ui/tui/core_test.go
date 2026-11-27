package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	corepage "github.com/AimAI-Labs/mihosh/internal/ui/tui/features/core"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestOfflineStartupRoutesToCoreWithoutStartingStreams(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	svc := service.NewCoreService()
	defer svc.Close()
	m := Model{config: &config.Config{}, coreSvc: svc, coreState: corepage.NewState(svc, nil, nil), width: 100, height: 30}
	updated, cmd := m.Update(messages.CoreConnectionMsg{Err: errors.New("not installed")})
	m = updated.(Model)
	if m.currentPage != layout.PageCore || m.wsStarted || cmd != nil {
		t.Fatal("offline startup started online tasks")
	}
	if !strings.Contains(m.View(), i18n.T("core.tab.downloads")) {
		t.Fatal("Core page not rendered")
	}
}

func TestCoreGeoResultIsDeliveredWhileAnotherPageIsOpen(t *testing.T) {
	i18n.Init()
	svc := service.NewCoreService()
	defer svc.Close()
	s := corepage.NewState(svc, nil, nil)
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	m := Model{currentPage: layout.PageSub, coreSvc: svc, coreState: s}
	updated, _ := m.Update(messages.CoreGeoDataMsg{Files: []service.GeoDataFile{{Key: "mmdb", Name: "Country.mmdb", Updatable: true}}})
	m = updated.(Model)
	if m.currentPage != layout.PageSub || !strings.Contains(m.coreState.Render(120, 30), "Country.mmdb") {
		t.Fatal("GEO result was lost after leaving the Core page")
	}
}
func TestCoreIsReachableFromNavigationByMouse(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	m := Model{width: 100, height: 30}
	x := -1
	for candidate := 1; candidate < 99; candidate++ {
		if layout.GetClickedTopNavPage(candidate, 1, 100) == layout.PageCore {
			x = candidate
			break
		}
	}
	if x < 0 {
		t.Fatal("Core navigation missing")
	}
	result, _ := m.Update(tea.MouseMsg{X: x, Y: 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if result.(Model).currentPage != layout.PageCore {
		t.Fatal("Core click did not route")
	}
}

func TestStaleCoreConnectionCannotOverrideCurrentTarget(t *testing.T) {
	m := Model{currentPage: layout.PageSub, connectionEpoch: 2, connectionReady: true}
	updated, cmd := m.Update(messages.CoreConnectionMsg{Epoch: 1, Err: errors.New("old target failed")})
	next := updated.(Model)
	if next.currentPage != layout.PageSub || !next.connectionReady || cmd != nil {
		t.Fatal("stale health result changed current target")
	}
}

func TestQueuedWebSocketMessageFromPreviousControllerIsIgnored(t *testing.T) {
	ws := api.NewWSClient("127.0.0.1:9090", "")
	old := ws.Generation()
	ws.UpdateEndpoint("127.0.0.1:9091", "")
	m := Model{wsClient: ws, chartData: model.NewChartData(10)}
	updated, _ := m.Update(messages.CoreWSBatchMsg{Generation: old, Memory: &messages.MemoryWSMsg{Memory: 1234}})
	if len(updated.(Model).chartData.MemoryHistory) != 0 {
		t.Fatal("old controller message polluted new chart")
	}
}

func TestCoreReleaseCatalogueSurvivesPageChangeAndGlobalRefresh(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	svc := service.NewCoreService()
	defer svc.Close()
	s := corepage.NewState(svc, nil, nil)
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	defer s.Close()
	m := Model{currentPage: layout.PageSub, coreSvc: svc, coreState: s, width: 120, height: 30}
	page := coreinfra.ReleasePage{Page: 1, Releases: []coreinfra.Release{coreinfra.PinnedRelease()}}
	updated, _ := m.Update(messages.CoreOperationMsg{ID: s.OperationID(), Action: "releases", Releases: &page})
	m = updated.(Model)
	if m.currentPage != layout.PageSub || m.coreState.Busy {
		t.Fatal("release query completion changed the page or left the task running")
	}
	m.currentPage = layout.PageCore
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	if !strings.Contains(m.View(), "Available core versions") {
		t.Fatal("background query did not reach the version browser")
	}
	previous := m.coreState.OperationID()
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = updated.(Model)
	defer m.coreState.Close()
	if !m.coreState.Busy || m.coreState.OperationID() != previous+1 || cmd == nil {
		t.Fatal("global r did not refresh the release list")
	}
}

func TestCoreDownloadTaskAllowsGlobalNavigationAndMouse(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	svc := service.NewCoreService()
	defer svc.Close()
	s := corepage.NewState(svc, nil, nil)
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	defer s.Close()
	m := Model{currentPage: layout.PageCore, coreSvc: svc, coreState: s, width: 120, height: 30}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if updated.(Model).currentPage == layout.PageCore || !updated.(Model).coreState.Busy {
		t.Fatal("running download blocked global keyboard navigation or lost its state")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if !updated.(Model).showHelp {
		t.Fatal("running download blocked help")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if updated.(Model).coreState.OperationID() != s.OperationID() || !updated.(Model).coreState.Busy {
		t.Fatal("global refresh started a second task")
	}
	for _, mouse := range []tea.MouseMsg{
		{X: 5, Y: 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress},
		{X: 5, Y: 1, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress},
	} {
		next, _ := m.Update(mouse)
		if next.(Model).currentPage == layout.PageCore || !next.(Model).coreState.Busy {
			t.Fatal("running download blocked global mouse navigation or lost its state")
		}
	}
}

func TestCoreSettingsDialogsBlockGlobalNavigationAndMouse(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	svc := service.NewCoreService()
	defer svc.Close()
	for _, shortcut := range []string{"e", "m", "c"} {
		s := corepage.NewState(svc, nil, nil)
		tab := "3"
		if shortcut == "e" {
			tab = "2"
		}
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tab)})
		if s.Busy {
			s, _ = s.Update(messages.CoreOperationMsg{ID: s.OperationID(), Action: "releases", Releases: &coreinfra.ReleasePage{Page: 1}})
		}
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(shortcut)})
		m := Model{currentPage: layout.PageCore, coreState: s, width: 120, height: 30}
		for _, key := range []string{"r", "?", "q"} {
			next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
			if next.(Model).currentPage != layout.PageCore || next.(Model).showHelp || !next.(Model).coreState.Editing || next.(Model).coreState.Busy {
				t.Fatal("global key escaped the settings dialog")
			}
		}
		for _, mouse := range []tea.MouseMsg{
			{X: 5, Y: 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress},
			{X: 5, Y: 1, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress},
		} {
			next, cmd := m.Update(mouse)
			if next.(Model).currentPage != layout.PageCore || next.(Model).coreState.Editing == isMouseLeftPress(mouse) || cmd != nil {
				t.Fatal("mouse navigation escaped the settings dialog")
			}
		}
	}
	s := corepage.NewState(svc, nil, nil)
	s.Status.Mode = "managed"
	s.Status.ServiceUnit = "mihosh-example.service"
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	if !s.IsMihomoPage() || s.Editing {
		t.Fatal("h shortcut did not jump to the Mihomo service status page")
	}
}

func TestCoreMihomoEditorCapturesGlobalKeys(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	svc := service.NewCoreService()
	defer svc.Close()
	s := corepage.NewState(svc, nil, nil)
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	s, _ = s.Update(messages.MihomoOverridesMsg{})
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := Model{currentPage: layout.PageCore, coreState: s, width: 120, height: 30}
	for _, text := range []string{"q", "?", "2"} {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text)})
		m = next.(Model)
		if !m.coreState.IsMihomoPage() || !m.coreState.InputCapture().Active || m.showHelp || m.currentPage != layout.PageCore {
			t.Fatal("global key escaped the Mihomo field editor")
		}
	}
}

func TestCoreVersionHelpUsesSelectedAssetAndCapturesScrolling(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	i18n.Init()
	svc := service.NewCoreService()
	defer svc.Close()
	s := corepage.NewState(svc, nil, nil)
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	r := coreinfra.PinnedRelease()
	r.Assets = []coreinfra.Asset{{Name: "selected-mihomo-build.gz"}}
	s, _ = s.Update(messages.CoreOperationMsg{ID: s.OperationID(), Action: "releases", Releases: &coreinfra.ReleasePage{Page: 1, Releases: []coreinfra.Release{r}}})
	defer s.Close()
	m := Model{currentPage: layout.PageCore, coreState: s, width: 80, height: 24}
	ctx := m.buildHelpContext()
	if !ctx.CoreReleases || ctx.CoreAsset != "selected-mihomo-build.gz" || ctx.CorePlatform == "" {
		t.Fatal("help context did not use the selected release")
	}
	if len(ctx.CoreReleaseNotes) < 3 || !strings.Contains(ctx.CoreReleaseNotes[0], r.Version()) {
		t.Fatal("help context did not include selected version details and catalogue status")
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	if !m.showHelp || m.helpState.Offset() != 1 || m.coreState.OperationID() != s.OperationID() {
		t.Fatal("help scroll reached the underlying release page")
	}
	updated, _ = m.Update(tea.MouseMsg{X: 5, Y: 1, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	m = updated.(Model)
	if !m.showHelp || m.helpState.Offset() != 2 || m.currentPage != layout.PageCore {
		t.Fatal("help wheel escaped to global navigation")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.showHelp || m.helpState.Offset() != 0 {
		t.Fatal("closing help retained its scroll state")
	}
}
func TestCoreMihomoServiceFocusFromTabAndFooter(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	i18n.Init()
	svc := service.NewCoreService()
	defer svc.Close()
	s := corepage.NewState(svc, nil, nil)
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("5")})
	s.Status.ServiceUnit = "fixture.service"
	m := Model{currentPage: layout.PageCore, coreState: s, width: 140, height: 38}

	expectFocus := func(want string, msg tea.Msg) {
		t.Helper()
		next, _ := m.Update(msg)
		m = next.(Model)
		if m.currentPage != layout.PageCore {
			t.Fatalf("%v switched to page %d", msg, m.currentPage)
		}
		for _, hint := range m.coreState.InlineHelpHints() {
			if hint.Key == want {
				return
			}
		}
		t.Fatalf("%v did not switch to the pane with %s in the footer", msg, want)
	}
	expectFocus("Tab/Esc", tea.KeyMsg{Type: tea.KeyTab})
	expectFocus("Tab", tea.KeyMsg{Type: tea.KeyShiftTab})

	clickTab := func(want string) {
		t.Helper()
		page := m.mainPageLayout()
		for y := page.noticeRow() + 1; y < page.statusRow(); y++ {
			for x := 0; x < page.width; x++ {
				px, py, _, _, ok := page.resolveMainPageMouseHit(x, y)
				if !ok {
					continue
				}
				keyMsg, hit := page.resolveInlineHelpClick(px, py)
				if hit && keyMsg.Type == tea.KeyTab {
					expectFocus(want, tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
					return
				}
			}
		}
		t.Fatal("focus shortcut not found in the clickable footer")
	}
	clickTab("Tab/Esc")
	clickTab("Tab")

	m.coreState.Status.ServiceUnit = ""
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if next.(Model).currentPage != (layout.PageCore+1)%layout.PageCount {
		t.Fatal("Tab did not navigate globally when no service card was present")
	}
}

func TestCoreDialogsRenderAndClickBottomRightFooterHints(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	svc := service.NewCoreService()
	defer svc.Close()

	clickFooterHint := func(t *testing.T, m Model, keyLabel string) (Model, tea.Cmd) {
		t.Helper()
		page := m.mainPageLayout()
		view := strings.Split(ansi.Strip(m.View()), "\n")
		for y := page.noticeRow() + 1; y < page.statusRow(); y++ {
			if at := strings.Index(view[y], keyLabel); at >= 0 {
				x := lipgloss.Width(view[y][:at])
				next, cmd := m.Update(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
				return next.(Model), cmd
			}
		}
		t.Fatalf("footer hint %q not found in:\n%s", keyLabel, strings.Join(view, "\n"))
		return m, nil
	}

	for _, language := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(language)
		for _, size := range [][2]int{{120, 30}, {32, 24}} {
			for _, tc := range []struct {
				shortcut string
				wantHint string
			}{
				{"x", "Enter " + i18n.T("core.inline.run")},
				{"u", "Tab " + i18n.T("core.inline.field")},
				{"enter", "↑↓ " + i18n.T("core.inline.scroll")},
			} {
				s := corepage.NewState(svc, nil, nil)
				s.Status = service.CoreStatus{Mode: "managed", Version: "v1.2.3", Running: true}
				s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
				m := Model{currentPage: layout.PageCore, coreSvc: svc, coreState: s, width: size[0], height: size[1]}
				var key tea.Msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.shortcut)}
				if tc.shortcut == "enter" {
					key = tea.KeyMsg{Type: tea.KeyEnter}
				}
				next, _ := m.Update(key)
				m = next.(Model)
				if !m.coreState.Editing {
					t.Fatalf("%s %v %s: dialog did not open", language, size, tc.shortcut)
				}

				page := m.mainPageLayout()
				if len(page.help) == 0 {
					t.Fatalf("%s %v %s: expected dialog hints in mainPageLayout", language, size, tc.shortcut)
				}
				rows := strings.Split(ansi.Strip(m.View()), "\n")
				aboveNotice := strings.Join(rows[:page.noticeRow()], "\n")
				cancelLabel := "Esc " + i18n.T("core.inline.cancel")
				if strings.Contains(aboveNotice, cancelLabel) {
					t.Fatalf("%s %v %s: dialog hints must not render inside the dialog box above notice row:\n%s", language, size, tc.shortcut, aboveNotice)
				}
				footerBlock := strings.Join(rows[page.noticeRow()+1:page.statusRow()], "\n")
				if !strings.Contains(footerBlock, cancelLabel) || !strings.Contains(footerBlock, tc.wantHint) {
					t.Fatalf("%s %v %s: bottom-right footer missing hints (%q, %q):\n%s", language, size, tc.shortcut, tc.wantHint, cancelLabel, footerBlock)
				}
				for i, hints := range page.help {
					y := page.noticeRow() + 1 + i
					want := ansi.Strip(common.FormatInlineHintRow(hints))
					if !strings.HasSuffix(strings.TrimRight(rows[y], " "), strings.TrimRight(want, " ")) {
						t.Fatalf("%s %v %s row %d not right-aligned: %q", language, size, tc.shortcut, y, rows[y])
					}
				}

				// Clicking Esc in the bottom-right footer closes the dialog.
				closed, cmd := clickFooterHint(t, m, cancelLabel)
				if closed.coreState.Editing || cmd != nil {
					t.Fatalf("%s %v %s: clicking Esc footer hint did not dismiss dialog", language, size, tc.shortcut)
				}
			}
		}

		// Clicking Enter in the bottom-right footer on the stop dialog executes stop.
		s := corepage.NewState(svc, nil, nil)
		s.Status = service.CoreStatus{Mode: "managed", Version: "v1.2.3", Running: true}
		m := Model{currentPage: layout.PageCore, coreSvc: svc, coreState: s, width: 120, height: 30}
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
		m = next.(Model)
		confirmed, cmd := clickFooterHint(t, m, "Enter "+i18n.T("core.inline.run"))
		if confirmed.coreState.Editing || !confirmed.coreState.Busy || cmd == nil {
			t.Fatalf("%s: clicking Enter footer hint on stop dialog did not run action", language)
		}
		confirmed.coreState.Close()

		// Clicking Tab in the bottom-right footer on the external form moves focus to field 2.
		s2 := corepage.NewState(svc, nil, nil)
		m2 := Model{currentPage: layout.PageCore, coreSvc: svc, coreState: s2, width: 120, height: 30}
		next, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
		m2 = next.(Model)
		tabbed, _ := clickFooterHint(t, m2, "Tab "+i18n.T("core.inline.field"))
		if !strings.Contains(ansi.Strip(tabbed.View()), "▸ "+i18n.T("core.secret")) {
			t.Fatalf("%s: clicking Tab footer hint did not move focus to secret field:\n%s", language, ansi.Strip(tabbed.View()))
		}
	}
}
