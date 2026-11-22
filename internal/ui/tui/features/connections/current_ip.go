package connections

import (
	"context"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections/components"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	currentIPRefreshInterval = 300 // 5 minutes, matching Clash Verge Rev
	currentIPRetryInterval   = 30  // retry sooner after a lookup error
)

type currentIPState struct {
	info        *model.IPInfo
	loading     bool
	err         string
	masked      bool
	countdown   int
	fetched     bool
	dirty       bool
	requestID   uint64
	cancel      context.CancelFunc
	modalOpen   bool
	modalScroll int
	width       int
	height      int
}

func (c *currentIPState) cancelFetch() {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	if c.loading {
		c.loading = false
		c.requestID++
	}
}

func (c *currentIPState) invalidate() {
	c.cancelFetch()
	c.info = nil
	c.err = ""
	c.countdown = 0
	c.fetched = false
	c.dirty = true
}

// SetCurrentIPFetcher overrides the exit IP lookup function (for testing).
func (s State) SetCurrentIPFetcher(fn func(context.Context, string) (*model.IPInfo, error)) State {
	s.currentIPFetcher = fn
	return s
}

// EnsureCurrentIP triggers an exit IP lookup if the traffic tab is active and IP data is missing or stale.
func (s State) EnsureCurrentIP() (State, tea.Cmd) {
	if s.connViewMode != ConnViewTraffic || s.currentIP.loading {
		return s, nil
	}
	if !s.currentIP.fetched || s.currentIP.dirty || s.currentIP.countdown <= 0 {
		return s.RefreshCurrentIP()
	}
	return s, nil
}

// RefreshCurrentIP immediately fetches the current exit IP info via the configured proxy.
func (s State) RefreshCurrentIP() (State, tea.Cmd) {
	s.currentIP.cancelFetch()
	s.currentIP.requestID++
	reqID := s.currentIP.requestID
	s.currentIP.loading = true
	s.currentIP.dirty = false
	s.currentIP.err = ""

	ctx, cancel := context.WithCancel(context.Background())
	s.currentIP.cancel = cancel

	proxyAddr := s.proxyAddr
	fetcher := s.currentIPFetcher
	if fetcher == nil {
		fetcher = service.FetchCurrentIPInfo
	}

	return s, func() tea.Msg {
		info, err := fetcher(ctx, proxyAddr)
		return messages.CurrentIPInfoMsg{
			RequestID: reqID,
			Info:      info,
			Err:       err,
		}
	}
}

// ApplyCurrentIPInfo updates the current exit IP bar when a lookup completes.
func (s State) ApplyCurrentIPInfo(msg messages.CurrentIPInfoMsg) State {
	if msg.RequestID != s.currentIP.requestID {
		return s
	}
	s.currentIP.cancel = nil
	s.currentIP.loading = false
	s.currentIP.fetched = true
	if msg.Err != nil {
		s.currentIP.info = nil
		s.currentIP.err = msg.Err.Error()
		s.currentIP.countdown = currentIPRetryInterval
		return s
	}
	s.currentIP.info = msg.Info
	s.currentIP.err = ""
	s.currentIP.countdown = currentIPRefreshInterval
	return s
}

// MarkCurrentIPStale marks cached exit IP info as stale so it refreshes when the traffic tab is visible.
func (s State) MarkCurrentIPStale() State {
	s.currentIP.dirty = true
	return s
}

// ToggleCurrentIPMask toggles privacy masking for the displayed current exit IP address.
func (s State) ToggleCurrentIPMask() State {
	s.currentIP.masked = !s.currentIP.masked
	return s
}

func (s State) tickCurrentIP() (State, tea.Cmd) {
	if s.connViewMode != ConnViewTraffic || s.currentIP.loading {
		return s, nil
	}
	if !s.currentIP.fetched || s.currentIP.dirty {
		return s.RefreshCurrentIP()
	}
	if s.currentIP.countdown > 1 {
		s.currentIP.countdown--
		return s, nil
	}
	s.currentIP.countdown = 0
	return s.RefreshCurrentIP()
}

func (s State) currentIPBarState() components.CurrentIPBarState {
	return components.CurrentIPBarState{
		Info:      s.currentIP.info,
		Loading:   s.currentIP.loading,
		Err:       s.currentIP.err,
		Masked:    s.currentIP.masked,
		Countdown: s.currentIP.countdown,
	}
}

// CurrentIPModalOpen reports whether the current IP info popup dialog is active.
func (s State) CurrentIPModalOpen() bool {
	return s.currentIP.modalOpen
}

func (s *State) openCurrentIPModal() {
	s.currentIP.modalOpen = true
	s.currentIP.modalScroll = 0
}

func (s *State) closeCurrentIPModal() {
	s.currentIP.modalOpen = false
	s.currentIP.modalScroll = 0
}

func (s State) handleCurrentIPModalKey(msg tea.KeyMsg) (State, tea.Cmd) {
	switch {
	case key.Matches(msg, common.Keys.Escape), key.Matches(msg, common.Keys.Enter), msg.String() == "q", msg.String() == "I":
		s.closeCurrentIPModal()
		return s, nil
	case msg.String() == "i":
		return s.ToggleCurrentIPMask(), nil
	case msg.String() == "r":
		return s.RefreshCurrentIP()
	case key.Matches(msg, common.Keys.Up):
		return s.scrollCurrentIPModal(true, s.currentIP.width, s.currentIP.height), nil
	case key.Matches(msg, common.Keys.Down):
		return s.scrollCurrentIPModal(false, s.currentIP.width, s.currentIP.height), nil
	}
	return s, nil
}

func (s State) clickCurrentIPModal(pageX, pageY, pageWidth, pageHeight int) (State, tea.Cmd) {
	s.currentIP.width = pageWidth
	s.currentIP.height = pageHeight
	layout := components.LayoutCurrentIPModal(s.currentIPBarState(), pageWidth, pageHeight, s.currentIP.modalScroll)
	left, top, right, bottom := layout.Dialog.Bounds()
	if pageX < left || pageX >= right || pageY < top || pageY >= bottom {
		s.closeCurrentIPModal()
		return s, nil
	}
	localX, localY := pageX-left, pageY-top
	if layout.RefreshX >= 0 && localY == layout.RefreshY &&
		localX >= layout.RefreshX && localX < layout.RefreshX+layout.RefreshWidth {
		return s.RefreshCurrentIP()
	}
	if layout.MaskX >= 0 && localY == layout.MaskY &&
		localX >= layout.MaskX && localX < layout.MaskX+layout.MaskWidth {
		return s.ToggleCurrentIPMask(), nil
	}
	return s, nil
}

func (s State) scrollCurrentIPModal(up bool, pageWidth, pageHeight int) State {
	if pageWidth > 0 && pageHeight > 0 {
		s.currentIP.width = pageWidth
		s.currentIP.height = pageHeight
	}
	if up {
		if s.currentIP.modalScroll > 0 {
			s.currentIP.modalScroll--
		}
		return s
	}
	if s.currentIP.width > 0 && s.currentIP.height > 0 {
		layout := components.LayoutCurrentIPModal(s.currentIPBarState(), s.currentIP.width, s.currentIP.height, s.currentIP.modalScroll)
		if s.currentIP.modalScroll < layout.MaxScroll {
			s.currentIP.modalScroll++
		}
		return s
	}
	s.currentIP.modalScroll++
	return s
}
