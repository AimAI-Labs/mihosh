package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestGeoManagementLoadsFilesAndQueuesSelectedUpdate(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	if err := config.Update(func(cfg *config.Config) error { cfg.Core.Mode = "managed"; return nil }); err != nil {
		t.Fatal(err)
	}
	path, err := config.ManagedMihomoConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), "Country.mmdb"), []byte("database"), 0600); err != nil {
		t.Fatal(err)
	}
	svc := service.NewCoreService()
	defer svc.Close()
	s := NewState(svc, nil, nil)
	s.Status.Mode = "managed"
	initial, initialCmd := s.Update(coreKey("4"))
	defer initial.Close()
	if !initial.Busy || initial.task != "releases" || initialCmd == nil {
		t.Fatal("opening Downloads with GEO focus did not request the version catalogue")
	}
	s.catalog.loaded = true
	s, cmd := s.Update(coreKey("4"))
	if s.tab != 3 || cmd == nil || s.Busy {
		t.Fatal("GEO tab did not load without starting a download")
	}
	for _, child := range cmd().(tea.BatchMsg) {
		if msg, ok := child().(messages.CoreGeoDataMsg); ok {
			s, _ = s.Update(msg)
		}
	}
	if len(s.geoFiles) != 4 || !s.geoFiles[0].Exists || s.geoFiles[0].Size != 8 || s.geoFiles[1].Exists || s.geoErr != nil {
		t.Fatalf("incorrect file status: %+v, %v", s.geoFiles, s.geoErr)
	}
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	for _, language := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(language)
		for _, width := range []int{24, 40, 80, 120} {
			for _, height := range []int{8, 16, 30} {
				view := s.Render(width, height)
				if lipgloss.Height(view) != height {
					t.Fatal("GEO view exceeded terminal height")
				}
				for _, line := range strings.Split(view, "\n") {
					if lipgloss.Width(line) > width || strings.Contains(line, "core.geo.") {
						t.Fatalf("invalid GEO view line: %q", line)
					}
				}
				header := strings.Split(s.layout(width, max(1, height-footerHeight)).content, "\n")
				if !strings.Contains(ansi.Strip(header[1]), i18n.T("core.tab.downloads")) {
					t.Fatalf("%s width=%d: Downloads is missing from the main tab row: %q", language, width, header[:3])
				}
			}
		}
	}
	s, _ = s.Update(messages.PageMouseScrollMsg{Width: 120, Height: 30})
	if s.selected != 1 || !s.followSelection {
		t.Fatal("wheel did not select the next GEO file")
	}
	s, cmd = s.Update(coreKey("enter"))
	if !s.Editing || s.form != "geo-update" || s.Busy || cmd != nil {
		t.Fatal("selecting GEO file must open a confirmation dialog before downloading")
	}
	if rendered := ansi.Strip(s.Render(120, 30)); !strings.Contains(rendered, s.geoFiles[1].Name) {
		t.Fatalf("GEO confirmation dialog did not display target file %q: %q", s.geoFiles[1].Name, rendered)
	}
	// Esc cancels cleanly without downloading.
	s, cmd = s.Update(coreKey("esc"))
	if s.Editing || s.Busy || cmd != nil {
		t.Fatal("esc did not cancel GEO confirmation dialog")
	}
	// Clicking a GEO row also opens the confirmation dialog instead of starting a download immediately.
	page := s.layout(120, 29)
	hit := page.actions[1]
	s, cmd = s.Update(messages.PageMouseClickMsg{X: hit.x + 1, Y: hit.y, Width: 120, Height: 30})
	if !s.Editing || s.form != "geo-update" || s.Busy || cmd != nil {
		t.Fatal("clicking GEO file must open a confirmation dialog before downloading")
	}
	// Clicking Cancel dismisses without downloading.
	dialogPage, _ := s.formDialog(120, 30)
	if len(dialogPage.controls) < 2 || dialogPage.controls[0].action != "confirm" || dialogPage.controls[1].action != "cancel" {
		t.Fatalf("expected confirm and cancel controls in GEO dialog, got %+v", dialogPage.controls)
	}
	cancelHit := dialogPage.controls[1].actionHit
	s, cmd = s.Update(messages.PageMouseClickMsg{X: cancelHit.x, Y: cancelHit.y, Width: 120, Height: 30})
	if s.Editing || s.Busy || cmd != nil {
		t.Fatal("cancel button did not dismiss GEO confirmation dialog")
	}
	// Clicking Confirm in the dialog starts the download.
	s, _ = s.Update(coreKey("enter"))
	dialogPage, _ = s.formDialog(120, 30)
	confirmHit := dialogPage.controls[0].actionHit
	s, cmd = s.Update(messages.PageMouseClickMsg{X: confirmHit.x, Y: confirmHit.y, Width: 120, Height: 30})
	defer s.Close()
	if s.Editing || !s.Busy || s.task != "geo-update" || cmd == nil {
		t.Fatal("confirmed GEO update was not queued")
	}
}

func TestDownloadSectionsAreReachableUnderOneTab(t *testing.T) {
	i18n.Init()
	const height = 30
	s := releaseFixture(t)
	s.geoFiles = []service.GeoDataFile{{Key: "mmdb", Name: "Country.mmdb", Updatable: true}}
	for _, width := range []int{80, 120} {
		page := s.layout(width, height-footerHeight)
		if len(page.tabs) != 3 || len(page.releases) == 0 || len(page.actions) != 1 {
			t.Fatal("both download panels must be present with a single top-level tab row")
		}
		version, geo := page.releases[0], page.actions[0]
		if width >= 100 && (geo.x <= version.x || geo.y != version.y) {
			t.Fatal("wide layout did not place the lists side by side")
		}
		if width < 100 && (geo.x != version.x || geo.y <= version.y) {
			t.Fatal("narrow layout did not stack GEO below the versions")
		}
		next, _ := s.Update(messages.PageMouseClickMsg{X: geo.x, Y: geo.y, Width: width, Height: height})
		if next.tab != 3 || !next.Editing || next.form != "geo-update" {
			t.Fatal("GEO row click did not open its confirmation from Downloads")
		}
		next, _ = next.Update(coreKey("esc"))
		next, _ = next.Update(coreKey("2"))
		if next.tab != 1 || next.Release.Tag != s.Release.Tag {
			t.Fatal("version focus did not restore the selected release")
		}
		next, _ = next.Update(coreKey("4"))
		next, _ = next.Update(coreKey("right"))
		if next.tab != 4 {
			t.Fatal("main navigation treated GEO as a separate tab")
		}
	}
}

func TestGeoEmptyAndCustomSourceDoNotStartDownloads(t *testing.T) {
	s := State{tab: 3}
	if next, cmd := s.Update(coreKey("enter")); next.Busy || cmd != nil {
		t.Fatal("empty GEO list started an operation")
	}
	s.geoFiles = []service.GeoDataFile{{Key: "mmdb", Updatable: false}}
	if next, cmd := s.Update(coreKey("enter")); next.Busy || cmd != nil {
		t.Fatal("custom source started an official download")
	}
	s.Editing, s.form, s.selected = true, "service", len(serviceActions)-1
	s, _ = s.Update(messages.CoreGeoDataMsg{Files: []service.GeoDataFile{{Key: "mmdb"}}})
	if s.selected != len(serviceActions)-1 {
		t.Fatal("background GEO response changed the selection in the service dialog")
	}
}
