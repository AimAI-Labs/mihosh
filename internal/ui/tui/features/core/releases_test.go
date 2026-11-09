package core

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func releaseFixture(t *testing.T) State {
	s := State{tab: 1, followSelection: true}
	s.catalog.loaded = true
	s.catalog.page = coreinfra.ReleasePage{Page: 1, HasNext: true}
	for i := range 10 {
		tag := fmt.Sprintf("v1.20.%d", 10-i)
		name := "mihomo-linux-amd64-compatible-" + tag + ".gz"
		s.catalog.page.Releases = append(s.catalog.page.Releases, coreinfra.Release{
			Tag: tag, PublishedAt: time.Date(2026, 10, 10-i, 0, 0, 0, 0, time.UTC), Trusted: true,
			Assets: []coreinfra.Asset{{Name: name, Digest: "sha256:" + strings.Repeat("a", 64), Size: 12 << 20}},
		})
	}
	s = s.applyReleasePage(s.catalog.page)
	t.Cleanup(func() { s.Close() })
	return s.selectRelease(0)
}

func TestVersionTabQueriesListWithoutInstallingOrSwitching(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	svc := service.NewCoreService()
	defer svc.Close()
	s := NewState(svc)
	s, cmd := s.Update(coreKey("2"))
	defer s.Close()
	if s.tab != 1 || !s.Busy || s.task != "releases" || s.catalog.requested != 1 || cmd == nil {
		t.Fatal("opening Versions did not queue a read-only release list request")
	}
	old := s.generation
	s, _ = s.Update(messages.CoreOperationMsg{ID: old + 1, Releases: &coreinfra.ReleasePage{Page: 99}})
	if !s.Busy || s.catalog.loaded {
		t.Fatal("stale list replaced the active request")
	}
	fixture := releaseFixture(t)
	page := coreinfra.ReleasePage{Page: 1, HasNext: true}
	for i := 0; i < fixture.releaseCount(); i++ {
		r, _ := fixture.releaseAt(i)
		page.Releases = append(page.Releases, r)
	}
	s, _ = s.Update(messages.CoreOperationMsg{ID: old, Action: "releases", Releases: &page})
	if s.Busy || !s.catalog.loaded || s.Release.Tag != "v1.20.10" || s.releaseCount() != 10 {
		t.Fatal("release list did not become selectable")
	}
	s, cmd = s.Update(coreKey("down"))
	if s.Release.Tag != "v1.20.9" || s.Busy || cmd != nil {
		t.Fatal("navigating versions must only change the selection")
	}
	s, cmd = s.Update(coreKey("enter"))
	defer s.Close()
	if !s.Busy || s.task != "install" || s.Release.Tag != "v1.20.9" || cmd == nil {
		t.Fatal("Enter did not queue download of the selected version")
	}
	// Commands are intentionally not executed: selecting/downloading must not start a kernel.
}

func TestVersionListInstalledAndChecksumGuards(t *testing.T) {
	s := releaseFixture(t)
	s.Status = service.CoreStatus{Mode: "managed", Version: s.Release.Tag, Installed: []string{s.Release.Tag}}
	for _, action := range []string{"install", "activate"} {
		next, cmd := s.releaseAction(action)
		if next.Busy || cmd != nil {
			t.Fatalf("%s must not repeat work on the selected current version", action)
		}
	}
	s = s.selectRelease(1)
	s, cmd := s.releaseAction("activate")
	if s.Busy || cmd != nil {
		t.Fatal("an uninstalled version was activated")
	}
	r, _ := s.releaseAt(1)
	r.Trusted = false
	if err := s.catalog.store.put(1, r); err != nil {
		t.Fatal(err)
	}
	s, cmd = s.releaseAction("install")
	if s.Busy || cmd != nil || s.Err == nil {
		t.Fatal("untrusted mirror checksum allowed installation")
	}
	r.Trusted = true
	if err := s.catalog.store.put(1, r); err != nil {
		t.Fatal(err)
	}
	s.Status.Installed = append(s.Status.Installed, r.Tag)
	s, cmd = s.releaseAction("activate")
	defer s.Close()
	if !s.Busy || s.task != "activate" || cmd == nil {
		t.Fatal("explicit activation of an installed version was not queued")
	}
}

func TestVersionPaginationRetainsCurrentPageOnFailureAndRetriesRequestedPage(t *testing.T) {
	s := releaseFixture(t)
	s, cmd := s.releaseAction("next")
	defer s.Close()
	if !s.Busy || s.catalog.requested != 2 || cmd == nil || s.catalog.page.Page != 1 {
		t.Fatal("next page did not preserve the visible list while fetching")
	}
	s, _ = s.Update(messages.CoreOperationMsg{ID: s.generation, Action: "releases", Err: errors.New("offline")})
	if s.catalog.page.Page != 1 || s.releaseCount() != 10 || s.Busy {
		t.Fatal("failed pagination discarded the available versions")
	}
	s, cmd = s.Update(coreKey("r"))
	defer s.Close()
	if !s.Busy || s.catalog.requested != 2 || cmd == nil {
		t.Fatal("retry did not target the failed page")
	}
}

func TestVersionManualLookupIsSelectableAndBounded(t *testing.T) {
	s := releaseFixture(t)
	for i := range 30 {
		r := coreinfra.PinnedRelease()
		r.Tag = fmt.Sprintf("v1.21.%d", i)
		s, _ = s.Update(messages.CoreOperationMsg{ID: s.generation, Action: "release", Release: &r})
		if s.Release.Tag != r.Tag || s.releaseCount() != 11 || s.selected != 0 {
			t.Fatal("manual lookup is unavailable or accumulating unbounded results")
		}
	}
}

func TestVersionListMouseSelectionDownloadAndNavigationMatchRenderedPositions(t *testing.T) {
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	for _, lang := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(lang)
		for _, width := range []int{80, 100, 120} {
			s := releaseFixture(t)
			view := ansi.Strip(s.Render(width, 30))
			found := false
			for y, line := range strings.Split(view, "\n") {
				if at := strings.Index(line, "v1.20.9"); at >= 0 {
					x := lipgloss.Width(line[:at])
					next, cmd := s.Update(messages.PageMouseClickMsg{X: x, Y: y, Width: width, Height: 30})
					if next.selected != 1 || next.Release.Tag != "v1.20.9" || next.Busy || cmd != nil {
						t.Fatalf("%s %d: row click did not select without downloading", lang, width)
					}
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("%s %d: list is not visible:\n%s", lang, width, view)
			}
			for _, target := range []struct{ key, action string }{{"core.catalog.download", "install"}, {"core.catalog.load_more", "releases"}} {
				found = false
				for y, line := range strings.Split(view, "\n") {
					if at := strings.Index(line, i18n.T(target.key)); at >= 0 {
						next, cmd := s.Update(messages.PageMouseClickMsg{X: lipgloss.Width(line[:at]), Y: y, Width: width, Height: 30})
						if !next.Busy || next.task != target.action || cmd == nil {
							t.Fatalf("%s %d: click on %s did not queue %s", lang, width, target.key, target.action)
						}
						next.cancel()
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("%s %d: %s is not visible:\n%s", lang, width, target.key, view)
				}
			}
		}
	}
}

func TestVersionListViewportKeepsSelectionAndBounds(t *testing.T) {
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	for _, lang := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(lang)
		for _, width := range []int{24, 64, 80, 100, 120} {
			for _, height := range []int{16, 24, 35} {
				s := releaseFixture(t).selectRelease(9)
				view := ansi.Strip(s.Render(width, height))
				if lipgloss.Height(view) > height {
					t.Fatal("viewport height overflow")
				}
				for _, line := range strings.Split(view, "\n") {
					if lipgloss.Width(line) > width {
						t.Fatal("viewport width overflow")
					}
				}
				if width >= 80 && height >= 24 && !strings.Contains(view, "› v1.20.1") {
					t.Fatalf("%s %dx%d: selected last release is invisible:\n%s", lang, width, height, view)
				}
			}
		}
	}
}

func TestManualLookupReplacesUntrustedMetadataForAnExistingRow(t *testing.T) {
	s := releaseFixture(t)
	r, _ := s.releaseAt(3)
	r.Trusted = false
	if err := s.catalog.store.put(3, r); err != nil {
		t.Fatal(err)
	}
	r.Trusted = true
	s, _ = s.Update(messages.CoreOperationMsg{ID: s.generation, Action: "release", Release: &r})
	stored, _ := s.releaseAt(3)
	if s.selected != 3 || !s.Release.Trusted || !stored.Trusted || s.releaseCount() != 10 {
		t.Fatal("fresh official metadata did not replace the matching mirror row")
	}
}

func TestVersionListRemainsUsableAfterQueryFailureOnSmallTerminals(t *testing.T) {
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	for _, lang := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(lang)
		for _, width := range []int{80, 120} {
			s := releaseFixture(t)
			s.Err = errors.New(strings.Repeat("query failed; try another route. ", 10))
			view := ansi.Strip(s.Render(width, 24))
			for _, want := range []string{"› v1.20.10", i18n.T("core.catalog.refresh")} {
				if !strings.Contains(view, want) {
					t.Fatalf("%s %d: failed-query view hides %q:\n%s", lang, width, want, view)
				}
			}
		}
	}
}

func olderReleases(page int, count int, more bool) coreinfra.ReleasePage {
	result := coreinfra.ReleasePage{Page: page, HasNext: more}
	for i := range count {
		tag := fmt.Sprintf("v1.%d.%d", 20-page, count-i)
		result.Releases = append(result.Releases, coreinfra.Release{Tag: tag, Trusted: true, Assets: []coreinfra.Asset{{Name: "core.gz", Digest: "sha256:" + strings.Repeat("a", 64)}}})
	}
	return result
}

func TestLazyCataloguePrefetchesAppendsAndDeduplicatesWithoutMovingSelection(t *testing.T) {
	s := releaseFixture(t).selectRelease(6)
	s, cmd := s.Update(coreKey("down"))
	defer s.Close()
	if !s.Busy || s.catalog.requested != 2 || cmd == nil || s.selected != 7 {
		t.Fatal("approaching the end did not prefetch exactly one page")
	}
	tag := s.Release.Tag
	generation := s.generation
	s, _ = s.Update(coreKey("down"))
	if s.generation != generation || s.selected != 8 {
		t.Fatal("prefetch prevented browsing or issued a duplicate request")
	}
	tag = s.Release.Tag
	page := olderReleases(2, 10, true)
	duplicate, _ := s.releaseAt(9)
	page.Releases[0] = duplicate
	s, _ = s.Update(messages.CoreOperationMsg{ID: generation, Action: "releases", Releases: &page})
	if s.Busy || s.releaseCount() != 19 || s.Release.Tag != tag || s.selected != 8 {
		t.Fatalf("append lost selection or duplicates: count=%d selected=%d", s.releaseCount(), s.selected)
	}
	first, _ := s.releaseAt(0)
	last, _ := s.releaseAt(18)
	if first.Tag != "v1.20.10" || last.Tag != page.Releases[9].Tag {
		t.Fatal("loaded history was replaced or reordered")
	}
}

func TestLazyCatalogueMouseScrollPreservesViewportAndStopsOnError(t *testing.T) {
	s := releaseFixture(t)
	var cmd tea.Cmd
	wheel := messages.PageMouseScrollMsg{Width: 80, Height: 26}
	for i := 0; i < 15 && !s.Busy; i++ {
		s, cmd = s.Update(wheel)
	}
	defer s.Close()
	if !s.Busy || cmd == nil || s.catalog.requested != 2 {
		t.Fatal("wheel did not load near the visible end")
	}
	start := s.catalog.scroll
	generation := s.generation
	s, _ = s.Update(wheel)
	if s.generation != generation {
		t.Fatal("duplicate pending load")
	}
	s, _ = s.Update(messages.CoreOperationMsg{ID: generation, Action: "releases", Err: errors.New("offline")})
	for range 5 {
		s, _ = s.Update(wheel)
	}
	if s.Busy || !s.catalog.failed || s.releaseCount() != 10 {
		t.Fatal("failure retried automatically or discarded history")
	}
	s, cmd = s.Update(coreKey("r"))
	defer s.Close()
	if !s.Busy || cmd == nil || s.catalog.requested != 2 {
		t.Fatal("retry did not resume the failed page")
	}
	page := olderReleases(2, 10, false)
	start = s.catalog.scroll
	s, _ = s.Update(messages.CoreOperationMsg{ID: s.generation, Action: "releases", Releases: &page})
	if s.releaseCount() != 20 || s.catalog.scroll != start || s.followSelection || s.catalog.page.HasNext {
		t.Fatal("append moved the mouse viewport or lost the terminal state")
	}
	for range 5 {
		s, _ = s.Update(wheel)
	}
	if s.Busy {
		t.Fatal("loaded past the end of upstream history")
	}
}

func TestLazyCatalogueSkipsEmptyFilteredPagesAndEventuallyFinishes(t *testing.T) {
	s := releaseFixture(t)
	s, _ = s.releaseAction("next")
	defer s.Close()
	empty := coreinfra.ReleasePage{Page: 2, HasNext: true}
	s, cmd := s.Update(messages.CoreOperationMsg{ID: s.generation, Action: "releases", Releases: &empty})
	defer s.Close()
	if !s.Busy || cmd == nil || s.catalog.requested != 3 || s.releaseCount() != 10 {
		t.Fatal("a fully filtered page stopped lazy browsing")
	}
	page := olderReleases(3, 2, false)
	s, _ = s.Update(messages.CoreOperationMsg{ID: s.generation, Action: "releases", Releases: &page})
	if s.Busy || s.releaseCount() != 12 || s.catalog.page.HasNext {
		t.Fatal("terminal page was not appended")
	}
	s, cmd = s.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if s.Busy || cmd != nil || s.selected != 11 {
		t.Fatal("End did not navigate the complete loaded list")
	}
}

func TestLazyCatalogueRefreshReplacesSnapshotOnlyAfterSuccess(t *testing.T) {
	s := releaseFixture(t)
	root := s.catalog.store.root
	s, _ = s.releaseAction("refresh")
	defer s.Close()
	if s.releaseCount() != 10 {
		t.Fatal("refresh removed the visible snapshot before success")
	}
	page := olderReleases(1, 3, false)
	s, _ = s.Update(messages.CoreOperationMsg{ID: s.generation, Action: "releases", Releases: &page})
	defer s.Close()
	if s.releaseCount() != 3 || s.catalog.store.root == root {
		t.Fatal("refresh did not replace the snapshot")
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("previous snapshot temp data was retained")
	}
}

func TestInitialModelCloseCleansUpLatestLazySnapshot(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	svc := service.NewCoreService()
	defer svc.Close()
	initial := NewState(svc)
	defer initial.Close()
	updated := initial.applyReleasePage(olderReleases(1, 3, true))
	oldRoot := updated.catalog.store.root
	updated = updated.applyReleasePage(olderReleases(1, 2, false))
	root := updated.catalog.store.root
	initial.Close()
	for _, dir := range []string{oldRoot, root} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatal("an initial model value leaked an updated catalogue snapshot")
		}
	}
}
