package core

import (
	"errors"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
)

type releaseCatalog struct {
	page             coreinfra.ReleasePage
	requested        int
	loaded           bool
	lookup           *coreinfra.Release
	scroll           int
	store            *releaseStore
	failed           bool
	resume           bool
	forceRefresh     bool
	firstPageFetched time.Time
	cleanup          *releaseCleanup
}

// The initial Bubble Tea model is also closed on terminal errors. Share only
// the resource owner so that closing that value cleans up the latest snapshot.
type releaseCleanup struct{ store *releaseStore }

func (s State) releaseCount() int {
	count := len(s.catalog.page.Releases)
	if s.catalog.store != nil {
		count = s.catalog.store.count
	}
	if s.catalog.lookup != nil {
		count++
	}
	return count
}

func (s State) releaseAt(index int) (coreinfra.Release, bool) {
	if index < 0 {
		return coreinfra.Release{}, false
	}
	if s.catalog.lookup != nil {
		if index == 0 {
			return *s.catalog.lookup, true
		}
		index--
	}
	if s.catalog.store != nil {
		r, err := s.catalog.store.at(index)
		return r, err == nil
	}
	if index >= len(s.catalog.page.Releases) {
		return coreinfra.Release{}, false
	}
	return s.catalog.page.Releases[index], true
}

func (s State) releaseIndex(tag string) (int, bool) {
	if s.catalog.lookup != nil && s.catalog.lookup.Tag == tag {
		return 0, true
	}
	offset := 0
	if s.catalog.lookup != nil {
		offset = 1
	}
	if s.catalog.store != nil {
		index, found := s.catalog.store.find(tag)
		return index + offset, found
	}
	for i, r := range s.catalog.page.Releases {
		if r.Tag == tag {
			return i + offset, true
		}
	}
	return 0, false
}

func (s State) applyReleaseLookup(r coreinfra.Release) State {
	if !r.IsStable() {
		return s
	}
	s.Release = r
	if s.catalog.store != nil {
		if index, found := s.catalog.store.find(r.Tag); found {
			if err := s.catalog.store.put(index, r); err != nil {
				s.Err = err
				return s
			}
			s.catalog.lookup = nil
			if s.tab == 1 {
				return s.selectRelease(index)
			}
			return s
		}
	}
	s.catalog.lookup = &r
	if s.tab == 1 {
		return s.selectRelease(0)
	}
	return s
}

func (s State) selectionCount() int {
	if s.tab == 1 {
		return max(1, s.releaseCount())
	}
	return max(1, len(s.actions()))
}

func (s State) selectedRelease() (coreinfra.Release, bool) {
	return s.releaseAt(min(s.selected, s.releaseCount()-1))
}

func (s State) selectRelease(index int) State {
	s.selected = max(0, min(index, s.selectionCount()-1))
	s.followSelection = true
	if r, ok := s.selectedRelease(); ok {
		s.Release = r
	}
	return s
}

func (s State) loadReleasePage(page int) (State, tea.Cmd) {
	if s.Busy || page < 1 {
		return s, nil
	}
	s.catalog.requested = page
	s.catalog.failed = false
	return s.begin("releases")
}

func (s State) openTab(tab int) (State, tea.Cmd) {
	s = s.selectTab(tab)
	if s.isDownloadsPage() {
		if !s.catalog.loaded && s.svc != nil {
			if page, ok := service.CachedReleasePage(1); ok {
				s = s.applyReleasePage(page)
			}
		}
		if !s.Busy && (!s.catalog.loaded || (!s.catalog.firstPageFetched.IsZero() && !service.ReleasePageIsFresh(s.catalog.firstPageFetched))) {
			s.catalog.forceRefresh = false
			next, cmd := s.loadReleasePage(1)
			return next, tea.Batch(cmd, next.Refresh())
		}
	}
	if s.tab == 4 && s.svc == nil {
		return s, s.editor.Refresh()
	}
	return s, s.Refresh()
}

func (s State) applyReleasePage(page coreinfra.ReleasePage) State {
	previous := s.Release.Tag
	follow, scroll := s.followSelection, s.catalog.scroll
	anchor, _ := s.releaseAt(scroll)
	store := s.catalog.store
	if page.Page == 1 || store == nil {
		var err error
		store, err = newReleaseStore()
		if err != nil {
			s.Err = err
			s.catalog.failed = true
			return s
		}
	}
	if err := store.append(page.Releases); err != nil {
		if store != s.catalog.store {
			store.close()
		}
		s.Err = err
		s.catalog.failed = true
		return s
	}
	if store != s.catalog.store {
		s.catalog.store.close()
	}
	s.catalog.store = store
	if s.catalog.cleanup != nil {
		s.catalog.cleanup.store = store
	}
	if s.catalog.lookup != nil {
		if _, found := store.find(s.catalog.lookup.Tag); found {
			s.catalog.lookup = nil
		}
	}
	page.Releases = nil // records live in the bounded, disk-backed catalogue
	s.catalog.page, s.catalog.loaded, s.catalog.failed = page, true, false
	if page.Page == 1 {
		s.catalog.scroll = 0
		s.catalog.firstPageFetched = page.FetchedAt
	}
	if s.tab == 1 {
		index, found := s.releaseIndex(previous)
		if !found {
			index = 0
		}
		s = s.selectRelease(index)
		if page.Page > 1 {
			s.followSelection = follow
			s.catalog.scroll = scroll
			if index, found := s.releaseIndex(anchor.Tag); found {
				s.catalog.scroll = index
			}
		}
	}
	return s
}

// Prefetch one page near the end. A failed request pauses automatic retries.
func (s State) maybeLoadMore() (State, tea.Cmd) {
	if s.Busy || s.catalog.failed || !s.catalog.loaded || !s.catalog.page.HasNext {
		return s, nil
	}
	if s.selected >= s.releaseCount()-3 {
		return s.releaseAction("next")
	}
	return s, nil
}

func (s State) releaseAction(action string) (State, tea.Cmd) {
	if s.Busy {
		return s, nil
	}
	switch action {
	case "refresh":
		s.catalog.resume = false
		s.catalog.forceRefresh = true
		return s.loadReleasePage(1)
	case "retry":
		s.catalog.resume = true
		return s.loadReleasePage(max(1, s.catalog.requested))
	case "next":
		if s.catalog.page.HasNext {
			s.catalog.resume = true
			s.catalog.forceRefresh = false
			return s.loadReleasePage(s.catalog.page.Page + 1)
		}
		return s, nil
	case "install", "activate":
		r, ok := s.selectedRelease()
		if !ok {
			return s, nil
		}
		s.Release = r
		installed := s.releaseInstalled(r.Version())
		if action == "install" {
			if installed {
				return s, nil
			}
			if len(r.Assets) == 0 {
				return s, nil
			}
			if _, err := r.TrustedDigest(r.Assets[0], coreinfra.PinnedDigests()); err != nil {
				s.Err = errors.New(i18n.T("core.catalog.unverified"))
				s.task = "install"
				return s, nil
			}
		} else if !installed || (s.Status.Mode == "managed" && s.Status.Version == r.Version()) {
			return s, nil
		}
		return s.begin(action)
	}
	return s, nil
}

func (s State) releaseInstalled(tag string) bool {
	for _, installed := range s.Status.Installed {
		if installed == tag {
			return true
		}
	}
	return false
}

func (s State) updateReleaseKeys(msg tea.KeyMsg) (State, tea.Cmd, bool) {
	var cmd tea.Cmd
	switch msg.String() {
	case "up", "k":
		s = s.selectRelease(s.selected - 1)
	case "down", "j":
		s = s.selectRelease(s.selected + 1)
	case "home":
		s = s.selectRelease(0)
	case "end":
		s = s.selectRelease(s.selectionCount() - 1)
	case "enter", "i":
		s, cmd = s.releaseAction("install")
	case "a":
		s, cmd = s.releaseAction("activate")
	case "v", "r":
		if s.catalog.failed {
			s, cmd = s.releaseAction("retry")
		} else {
			s, cmd = s.releaseAction("refresh")
		}
	case "pgup", "[":
		s = s.selectRelease(s.selected - 10)
	case "pgdown", "]":
		if s.selected >= s.releaseCount()-1 {
			s, cmd = s.releaseAction("next")
		} else {
			s = s.selectRelease(s.selected + 10)
		}
	default:
		return s, nil, false
	}
	if cmd == nil {
		switch msg.String() {
		case "down", "j", "end", "pgdown", "]":
			s, cmd = s.maybeLoadMore()
		}
	}
	return s, cmd, true
}

func (s State) BrowsingReleases() bool { return s.tab == 1 && !s.Editing }

// RefreshCurrent also refreshes the catalogue when the global r binding is used.
func (s State) RefreshCurrent() (State, tea.Cmd) {
	if !s.Editing && s.isDownloadsPage() {
		action := "refresh"
		if s.catalog.failed {
			action = "retry"
		}
		next, cmd := s.releaseAction(action)
		return next, tea.Batch(cmd, next.Refresh())
	}
	return s, s.Refresh()
}
