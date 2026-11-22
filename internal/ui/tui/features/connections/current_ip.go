package connections

import (
	"context"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections/components"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	currentIPRefreshInterval = 300 // 5 minutes, matching Clash Verge Rev
	currentIPRetryInterval   = 30  // retry sooner after a lookup error
)

type currentIPState struct {
	info      *model.IPInfo
	loading   bool
	err       string
	masked    bool
	countdown int
	fetched   bool
	dirty     bool
	requestID uint64
	cancel    context.CancelFunc
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
