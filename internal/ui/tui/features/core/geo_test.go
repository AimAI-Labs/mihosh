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
	s, cmd := s.Update(coreKey("4"))
	if s.tab != 3 || cmd == nil || s.Busy {
		t.Fatal("GEO tab did not load without starting a download")
	}
	s, _ = s.Update(cmd())
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
				header := strings.Split(s.layout(width, max(1, height-footerHeight)).content, "\n")[1]
				if !strings.Contains(ansi.Strip(header), i18n.T("core.tab.geo")) {
					t.Fatalf("%s width=%d: selected GEO tab is hidden in the header: %q", language, width, header)
				}
			}
		}
	}
	s, _ = s.Update(messages.PageMouseScrollMsg{Width: 120, Height: 30})
	if s.selected != 1 || !s.followSelection {
		t.Fatal("wheel did not select the next GEO file")
	}
	s, cmd = s.Update(coreKey("enter"))
	defer s.Close()
	if !s.Busy || s.task != "geo-update" || cmd == nil {
		t.Fatal("selected GEO update was not queued")
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
