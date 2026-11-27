package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
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
	s := NewState(svc, nil, nil)
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
	if s.Busy || s.task != "" || !s.catalog.loaded || s.Release.Tag != "v1.20.10" || s.releaseCount() != 10 {
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
	s, cmd = s.Update(coreKey("enter"))
	if s.Busy || s.Err != nil || s.task != "" || cmd == nil {
		t.Fatal("untrusted mirror checksum must report an error without starting a task")
	}
	notice, ok := cmd().(messages.NoticeMsg)
	if !ok || notice.Level != messages.NoticeError || notice.Text != i18n.T("core.catalog.unverified") {
		t.Fatalf("unexpected checksum notice: %+v", notice)
	}
	r.Trusted = true
	if err := s.catalog.store.put(1, r); err != nil {
		t.Fatal(err)
	}
	next, cmd := s.Update(coreKey("enter"))
	defer next.Close()
	if !next.Busy || next.task != "install" || cmd == nil {
		t.Fatal("Enter after the checksum notice did not start the verified install")
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

func TestVersionListMouseSelectionAndDownloadMatchRenderedPositions(t *testing.T) {
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
					next, cmd = next.Update(coreKey("enter"))
					if !next.Busy || next.task != "install" || next.Release.Tag != "v1.20.9" || cmd == nil {
						t.Fatalf("%s %d: Enter did not download the clicked version", lang, width)
					}
					next.cancel()
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("%s %d: list is not visible:\n%s", lang, width, view)
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
			for _, want := range []string{"› v1.20.10", "Enter/Esc " + i18n.T("core.inline.dismiss")} {
				if !strings.Contains(view, want) {
					t.Fatalf("%s %d: failed-query view hides %q:\n%s", lang, width, want, view)
				}
			}
		}
	}
}

func TestReleaseTaskStatusStaysInFooterWithoutNavigationLabels(t *testing.T) {
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	for _, language := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(language)
		for _, width := range []int{24, 80, 100, 120} {
			for _, height := range []int{8, 16, 28} {
				for _, status := range []string{"", "core.catalog.loading", "core.catalog.load_failed"} {
					s := releaseFixture(t)
					s.Busy = status == "core.catalog.loading"
					s.catalog.failed = status == "core.catalog.load_failed"
					if status != "" {
						s.task = "releases"
					}
					if s.catalog.failed {
						s.Err = errors.New(i18n.T(status))
					}
					view := ansi.Strip(s.Render(width, height))
					lines := strings.Split(view, "\n")
					hint := "? " + i18n.T("core.inline.help")
					if s.Busy {
						hint = "Esc " + i18n.T("core.inline.cancel")
					} else if s.task != "" {
						hint = "Enter/Esc " + i18n.T("core.inline.dismiss")
					}
					if len(lines) != height || !strings.HasSuffix(lines[height-1], hint) {
						t.Fatalf("%s %dx%d: task controls moved away from the bottom right", language, width, height)
					}
					for _, line := range lines {
						if lipgloss.Width(line) > width {
							t.Fatalf("%s %dx%d: release footer exceeds terminal width", language, width, height)
						}
					}
					for _, key := range []string{"core.catalog.load_more", "core.catalog.refresh", "core.catalog.retry", "core.action.releases"} {
						if strings.Contains(view, i18n.T(key)) {
							t.Fatalf("removed navigation label remains visible: %s", key)
						}
					}
					footer := strings.Join(lines[height-4:height-1], "\n")
					if s.Busy && strings.TrimSpace(footer) != "" {
						t.Fatal("background catalogue load displayed a task notice")
					}
					if s.catalog.failed && width >= 80 {
						if !strings.Contains(lines[height-1], i18n.T(status)) {
							t.Fatalf("%s %dx%d: task status is missing from footer", language, width, height)
						}
					}
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
	generation := s.generation
	s, _ = s.Update(coreKey("down"))
	if s.generation != generation || s.selected != 8 {
		t.Fatal("prefetch prevented browsing or issued a duplicate request")
	}
	tag := s.Release.Tag
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
	wheel := messages.PageMouseScrollMsg{X: 4, Y: 6, Width: 80, Height: 26}
	for i := 0; i < 15 && !s.Busy; i++ {
		s, cmd = s.Update(wheel)
	}
	defer s.Close()
	if !s.Busy || cmd == nil || s.catalog.requested != 2 {
		t.Fatal("wheel did not load near the visible end")
	}
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
	start := s.catalog.scroll
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
	initial := NewState(svc, nil, nil)
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

func TestReleaseRowsUseOneLineAndPreserveMouseBounds(t *testing.T) {
	i18n.Init()
	s := releaseFixture(t)
	for _, width := range []int{24, 80, 100, 120} {
		page := s.layout(width, 29)
		if strings.Contains(ansi.Strip(page.content), i18n.T("core.catalog.selection")) {
			t.Fatal("selected version card remains visible")
		}
		if len(page.releases) < 2 {
			t.Fatal("release rows are not visible")
		}
		lines := strings.Split(ansi.Strip(page.content), "\n")
		listWidth := width
		if width >= 100 {
			listWidth = width * 43 / 100
		}
		for i, hit := range page.releases {
			if lipgloss.Width(lines[hit.y]) != width || hit.x != 1 || hit.width != listWidth-2 {
				t.Fatal("release row does not fit the download panel")
			}
			if hit.height != 1 || (i > 0 && hit.y != page.releases[i-1].y+1) {
				t.Fatal("release rows still consume two lines")
			}
			r, _ := s.releaseAt(hit.index)
			if !strings.Contains(lines[hit.y], r.Version()) {
				t.Fatal("release mouse bounds do not match rendered row")
			}
		}
	}
}

func TestPrereleaseLookupDoesNotEnterStableCatalogue(t *testing.T) {
	for _, release := range []coreinfra.Release{
		{Tag: "Prerelease-Alpha", Prerelease: true},
		{Tag: "Prerelease-Beta", Prerelease: true},
		{Tag: "v1.21.0-rc.1"},
		{Tag: "v1.21.0", Prerelease: true},
	} {
		s := releaseFixture(t).selectRelease(3)
		selected := s.Release.Tag
		s, _ = s.Update(messages.CoreOperationMsg{ID: s.generation, Action: "release", Release: &release})
		if s.releaseCount() != 10 || s.selected != 3 || s.Release.Tag != selected || s.catalog.lookup != nil {
			t.Fatalf("lookup added a non-stable release to the catalogue: %s", release.Tag)
		}
	}
}

func TestPrereleaseCacheDoesNotEnterStableCatalogue(t *testing.T) {
	root := t.TempDir()
	t.Setenv("MIHOSH_CONFIG_DIR", root)
	cached := coreinfra.PinnedRelease()
	cached.Prerelease = true
	data, err := json.Marshal(cached)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "core"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "core", "release-cache.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	svc := service.NewCoreService()
	defer svc.Close()
	s := NewState(svc, nil, nil)
	defer s.Close()
	if release, ok := s.selectedRelease(); !ok || release.Prerelease || release.Draft || release.Tag != coreinfra.PinnedRelease().Tag {
		t.Fatalf("cached prerelease entered the catalogue: %+v", release)
	}
}

func TestReleaseRowColorsDistinguishVersionStatesAcrossThemes(t *testing.T) {
	i18n.Init()
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
	originalTheme := theme.CurrentName()
	t.Cleanup(func() { theme.SetTheme(originalTheme) })
	s := releaseFixture(t)
	current, _ := s.releaseAt(0)
	installed, _ := s.releaseAt(1)
	s.Status = service.CoreStatus{Mode: "managed", Version: current.Version(), Installed: []string{current.Version(), installed.Version()}}
	prerelease, _ := s.releaseAt(3)
	prerelease.Prerelease = true
	if err := s.catalog.store.put(3, prerelease); err != nil {
		t.Fatal(err)
	}
	for _, name := range theme.Names() {
		theme.SetTheme(name)
		page := s.layout(120, 29)
		lines := strings.Split(page.content, "\n")
		colors := map[string]bool{}
		for _, hit := range page.releases[:4] {
			row := ansi.Cut(lines[hit.y], hit.x+1, hit.x+hit.width-1)
			r, _ := s.releaseAt(hit.index)
			at := strings.Index(row, r.Version())
			if at < 0 {
				t.Fatal("colored version is missing")
			}
			codes := regexp.MustCompile(`\x1b\[38;2;[0-9;]+m`).FindAllString(row[:at], -1)
			if len(codes) == 0 {
				t.Fatal("version row is not colored")
			}
			color := codes[len(codes)-1]
			if colors[color] {
				t.Fatalf("theme %s: release states lack distinct colors: %q", name, row)
			}
			colors[color] = true
		}
	}
}

func TestReleaseColumnsAlignMixedVersionsAndHideOptionalMetadata(t *testing.T) {
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	for _, language := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(language)
		s := releaseFixture(t)
		r, _ := s.releaseAt(1)
		r.Tag, r.Prerelease = "alpha-478af2a", true
		r.Assets = nil
		if err := s.catalog.store.put(1, r); err != nil {
			t.Fatal(err)
		}
		s.Status = service.CoreStatus{Mode: "managed", Version: s.Release.Version()}
		for _, width := range []int{28, 44, 70} {
			columns := s.releaseColumns(0, 3, width)
			statusAt, dateAt, sizeAt := -1, -1, -1
			for i := range 3 {
				r, _ := s.releaseAt(i)
				row := s.releaseRow(r, "  ", columns, width)
				if lipgloss.Width(row) > width || !strings.Contains(row, "["+s.releaseBadge(r)+"]") {
					t.Fatalf("%s width %d: status lost or row overflow: %q", language, width, row)
				}
				at := lipgloss.Width(row[:strings.Index(row, "[")])
				if statusAt >= 0 && at != statusAt {
					t.Fatalf("status columns do not align: %q", row)
				}
				statusAt = at
				date, size := releaseMetadata(r)
				for _, field := range []struct {
					text string
					want bool
					at   *int
				}{{date, width >= 44, &dateAt}, {size, width >= 70, &sizeAt}} {
					index := strings.Index(row, field.text)
					if (index >= 0) != field.want {
						t.Fatalf("%s width %d: incorrect metadata visibility: %q", language, width, row)
					}
					if index >= 0 {
						at := lipgloss.Width(row[:index])
						if *field.at >= 0 && at != *field.at {
							t.Fatalf("metadata columns do not align: %q", row)
						}
						*field.at = at
					}
				}
			}
		}
	}
}

func TestReleasePageKeepsEssentialInformationAndFailuresWithoutStaticHints(t *testing.T) {
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	for _, language := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(language)
		s := releaseFixture(t)
		view := ansi.Strip(s.Render(120, 30))
		for _, hint := range []string{"core.catalog.platform", "core.catalog.download_hint", "core.catalog.more_hint", "core.catalog.unverified", "core.catalog.complete", "core.catalog.cached"} {
			if strings.Contains(view, i18n.T(hint)) {
				t.Fatalf("static explanation remains on the page: %s", hint)
			}
		}
		if strings.Contains(view, s.Release.Assets[0].Name) {
			t.Fatal("asset filename remains on the page")
		}
		if strings.Contains(view, i18n.Tf("core.catalog.loaded", 10)) {
			t.Fatal("loaded version statistics remain on the page")
		}
		for _, want := range []string{s.Release.Version(), i18n.T("core.inline.download"), i18n.T("core.inline.activate")} {
			if !strings.Contains(view, want) {
				t.Fatalf("essential information missing: %s", want)
			}
		}
		notes := strings.Join(s.ReleaseHelpNotes(), "\n")
		for _, want := range []string{s.Release.Version(), "2026-10-10", "12.0 MiB", i18n.Tf("core.catalog.loaded", 10)} {
			if !strings.Contains(notes, want) {
				t.Fatalf("selected version help lost %q", want)
			}
		}
		r := s.Release
		r.Trusted = false
		s = s.applyReleaseLookup(r)
		s.Err = errors.New("release query failed")
		s.catalog.failed = true
		view = ansi.Strip(s.Render(120, 30))
		for _, want := range []string{"release query failed", i18n.T("core.catalog.load_failed")} {
			if !strings.Contains(view, want) {
				t.Fatalf("operation failure was hidden: %s", want)
			}
		}
	}
}

func TestOpenReleasesTabUsesDiskCacheWhileRefreshBypassesIt(t *testing.T) {
	root := t.TempDir()
	t.Setenv("MIHOSH_CONFIG_DIR", root)

	pinned := coreinfra.PinnedRelease()
	cacheDir := filepath.Join(root, "core", "releases-cache")
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		t.Fatal(err)
	}
	payload := struct {
		FetchedAt time.Time             `json:"fetched_at"`
		GOOS      string                `json:"goos"`
		GOARCH    string                `json:"goarch"`
		Page      coreinfra.ReleasePage `json:"page"`
	}{
		FetchedAt: time.Now().UTC(),
		GOOS:      runtime.GOOS,
		GOARCH:    runtime.GOARCH,
		Page:      coreinfra.ReleasePage{Page: 1, HasNext: false, Releases: []coreinfra.Release{pinned}},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	cacheFile := filepath.Join(cacheDir, fmt.Sprintf("%s-%s-page-1.json", runtime.GOOS, runtime.GOARCH))
	if err := os.WriteFile(cacheFile, raw, 0600); err != nil {
		t.Fatal(err)
	}

	svc := service.NewCoreService()
	defer svc.Close()
	s := NewState(svc, nil, nil)
	defer s.Close()

	s, _ = s.openTab(1)
	if s.Busy || s.catalog.forceRefresh || !s.catalog.loaded || s.releaseCount() != 1 || s.Release.Tag != pinned.Tag {
		t.Fatalf("fresh cache was not displayed immediately: loaded=%v count=%d busy=%v", s.catalog.loaded, s.releaseCount(), s.Busy)
	}

	// A stale cache still renders immediately while the refresh is in progress.
	payload.FetchedAt = payload.FetchedAt.Add(-25 * time.Hour)
	raw, err = json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cacheFile, raw, 0600); err != nil {
		t.Fatal(err)
	}
	stale := NewState(svc, nil, nil)
	defer stale.Close()
	stale, cmd := stale.openTab(1)
	if cmd == nil || !stale.Busy || !stale.catalog.loaded || stale.releaseCount() != 1 {
		t.Fatal("stale cache must remain visible during refresh")
	}
	stale, _ = stale.Update(messages.CoreOperationMsg{ID: stale.generation, Action: "releases", Err: errors.New("offline")})
	if stale.Busy || stale.releaseCount() != 1 || !stale.catalog.failed {
		t.Fatal("failed refresh discarded the cached list")
	}

	// Re-entering after the in-memory catalogue expires refreshes it as well.
	s.catalog.firstPageFetched = payload.FetchedAt
	expired, cmd := s.openTab(1)
	if cmd == nil || !expired.Busy || expired.releaseCount() != 1 {
		t.Fatal("expired catalogue was not refreshed on re-entry")
	}
	expired.Close()

	// Triggering refresh via 'r' sets forceRefresh so upstream is queried.
	s, refreshCmd := s.releaseAction("refresh")
	if refreshCmd == nil || !s.catalog.forceRefresh {
		t.Fatalf("releaseAction(refresh) must bypass cache: cmd=%v forceRefresh=%v", refreshCmd != nil, s.catalog.forceRefresh)
	}
}

func TestReleaseCatalogueStorageFailuresProduceNotices(t *testing.T) {
	for _, tc := range []struct {
		action string
		level  messages.NoticeLevel
	}{
		{"releases", messages.NoticeWarning},
		{"release", messages.NoticeError},
	} {
		t.Run(tc.action, func(t *testing.T) {
			s := releaseFixture(t)
			r, _ := s.selectedRelease()
			if err := os.Mkdir(filepath.Join(s.catalog.store.root, "0.json.tmp"), 0700); err != nil {
				t.Fatal(err)
			}
			result := messages.CoreOperationMsg{Action: tc.action, ID: s.OperationID()}
			if tc.action == "releases" {
				result.Releases = &coreinfra.ReleasePage{Page: 2, Releases: []coreinfra.Release{r}}
			} else {
				result.Release = &r
			}
			s, _ = s.Update(result)
			if s.Err == nil {
				t.Fatal("expected catalogue write failure")
			}
			notice, ok := s.OperationNotice(result)
			if !ok || notice.Level != tc.level || !strings.Contains(notice.Detail, "0.json.tmp") {
				t.Fatalf("catalogue write failure did not surface: %+v", notice)
			}
		})
	}
}
