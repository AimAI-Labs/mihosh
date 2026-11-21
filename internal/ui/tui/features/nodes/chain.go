package nodes

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type chainModalFocus int

const (
	chainFocusCandidates chainModalFocus = iota
	chainFocusHops
)

// ChainCandidate represents one selectable proxy in the proxy chain modal.
type ChainCandidate struct {
	Name    string
	Type    string
	Delay   int
	IsGroup bool
}

// ChainModalState holds all page-local state for the proxy chain editor modal.
type ChainModalState struct {
	Open            bool
	Group           string
	HadExisting     bool
	DraftNodes      []string
	Candidates      []ChainCandidate
	Focus           chainModalFocus
	CandidateCursor int
	CandidateScroll int
	HopCursor       int
	HopScroll       int
	Applying        bool
	Notice          string
}

// SetServices attaches ConfigService and CoreService for persisting and reloading proxy chains.
func (s State) SetServices(configSvc *service.ConfigService, coreSvc *service.CoreService) State {
	s.configSvc = configSvc
	s.coreSvc = coreSvc
	return s.ReloadChains()
}

// ReloadChains reads persisted proxy chains from chain-edits.yaml into page-local state.
func (s State) ReloadChains() State {
	var edits service.ChainEdits
	var err error
	if s.configSvc != nil {
		edits, err = s.configSvc.ReadProxyChains()
	} else {
		edits, err = service.ReadChainEdits()
	}
	if err != nil {
		return s
	}
	chains := make(map[string]service.ProxyChain, len(edits.Chains))
	for _, c := range edits.Chains {
		if c.Enabled && len(c.Nodes) >= 2 {
			chains[c.Group] = c
		}
	}
	s.Chains = chains
	return s
}

// ActiveGroupChain returns the active proxy chain for the currently selected group, if any.
func (s State) ActiveGroupChain() (service.ProxyChain, bool) {
	if len(s.Chains) == 0 {
		return service.ProxyChain{}, false
	}
	if s.SelectedGroup >= 0 && s.SelectedGroup < len(s.GroupNames) {
		if c, ok := s.Chains[s.GroupNames[s.SelectedGroup]]; ok && c.Enabled && len(c.Nodes) >= 2 {
			return c, true
		}
	}
	return service.ProxyChain{}, false
}

// OpenChainModal opens the proxy chain configuration modal for the currently selected group.
func (s State) OpenChainModal() (State, tea.Cmd) {
	if s.Chains == nil {
		s = s.ReloadChains()
	}
	groupName := ""
	if s.SelectedGroup >= 0 && s.SelectedGroup < len(s.GroupNames) {
		groupName = s.GroupNames[s.SelectedGroup]
	} else {
		groupName = s.ResolveActiveGroupName()
	}

	candidates := s.buildChainCandidates(groupName)
	if len(candidates) == 0 {
		return s, nodesNoticeCmd(i18n.T("nodes.chain.no_candidates"), messages.NoticeWarning)
	}

	var draft []string
	hadExisting := false
	if existing, ok := s.Chains[groupName]; ok && existing.Enabled && len(existing.Nodes) >= 2 {
		draft = append([]string(nil), existing.Nodes...)
		hadExisting = true
	}

	s.ChainModal = ChainModalState{
		Open:        true,
		Group:       groupName,
		HadExisting: hadExisting,
		DraftNodes:  draft,
		Candidates:  candidates,
		Focus:       chainFocusCandidates,
	}
	return s, nil
}

func (s State) buildChainCandidates(groupName string) []ChainCandidate {
	seen := make(map[string]struct{})
	var candidates []ChainCandidate

	appendCandidate := func(name string) {
		if name == "" || service.IsChainBuiltInProxy(name) {
			return
		}
		if _, dup := seen[name]; dup {
			return
		}
		seen[name] = struct{}{}
		_, isGroup := s.Groups[name]
		proxyType := ""
		if p, ok := s.Proxies[name]; ok {
			proxyType = p.Type
		} else if g, ok := s.Groups[name]; ok {
			proxyType = g.Type
		}
		candidates = append(candidates, ChainCandidate{
			Name:    name,
			Type:    proxyType,
			Delay:   s.getProxyDelayUnsafe(name),
			IsGroup: isGroup,
		})
	}

	// Prioritize nodes belonging to the target group in their current order.
	if group, ok := s.Groups[groupName]; ok {
		for _, name := range group.All {
			appendCandidate(name)
		}
	}
	// Include any remaining concrete proxies so users can pick relays from another group (e.g. a dedicated relay group).
	extraNames := make([]string, 0, len(s.Proxies))
	for name := range s.Proxies {
		if _, exists := seen[name]; !exists {
			if _, isGroup := s.Groups[name]; !isGroup && !service.IsChainBuiltInProxy(name) {
				extraNames = append(extraNames, name)
			}
		}
	}
	sort.Strings(extraNames)
	for _, name := range extraNames {
		appendCandidate(name)
	}
	return candidates
}

func (s State) handleChainModalKey(msg tea.KeyMsg) (State, tea.Cmd) {
	if s.ChainModal.Applying {
		if key.Matches(msg, common.Keys.Escape) {
			return s, nil
		}
		return s, nil
	}

	m := &s.ChainModal
	m.Notice = ""

	switch {
	case key.Matches(msg, common.Keys.Escape):
		s.ChainModal = ChainModalState{}
		return s, nil

	case msg.String() == "tab", msg.String() == "shift+tab":
		if m.Focus == chainFocusCandidates {
			m.Focus = chainFocusHops
			if m.HopCursor >= len(m.DraftNodes) {
				m.HopCursor = max(0, len(m.DraftNodes)-1)
			}
		} else {
			m.Focus = chainFocusCandidates
		}
		return s, nil

	case key.Matches(msg, common.Keys.Up):
		if m.Focus == chainFocusHops {
			if m.HopCursor > 0 {
				m.HopCursor--
				m.HopScroll = min(m.HopScroll, m.HopCursor)
			}
		} else {
			if m.CandidateCursor > 0 {
				m.CandidateCursor--
				m.CandidateScroll = min(m.CandidateScroll, m.CandidateCursor)
			}
		}
		return s, nil

	case key.Matches(msg, common.Keys.Down):
		if m.Focus == chainFocusHops {
			if m.HopCursor < len(m.DraftNodes)-1 {
				m.HopCursor++
			}
		} else {
			if m.CandidateCursor < len(m.Candidates)-1 {
				m.CandidateCursor++
			}
		}
		return s, nil

	case msg.String() == "enter", msg.String() == " ", msg.String() == "a":
		if m.Focus == chainFocusCandidates && m.CandidateCursor >= 0 && m.CandidateCursor < len(m.Candidates) {
			cand := m.Candidates[m.CandidateCursor]
			if idx := slices.Index(m.DraftNodes, cand.Name); idx >= 0 {
				m.DraftNodes = slices.Delete(m.DraftNodes, idx, idx+1)
				if m.HopCursor >= len(m.DraftNodes) {
					m.HopCursor = max(0, len(m.DraftNodes)-1)
				}
			} else {
				if cand.IsGroup && len(m.DraftNodes) > 0 {
					m.Notice = i18n.T("nodes.chain.group_entry_only")
					return s, nil
				}
				m.DraftNodes = append(m.DraftNodes, cand.Name)
				m.HopCursor = len(m.DraftNodes) - 1
			}
		}
		return s, nil

	case msg.String() == "backspace", msg.String() == "delete", msg.String() == "d":
		if len(m.DraftNodes) == 0 {
			return s, nil
		}
		if m.Focus == chainFocusHops && m.HopCursor >= 0 && m.HopCursor < len(m.DraftNodes) {
			m.DraftNodes = slices.Delete(m.DraftNodes, m.HopCursor, m.HopCursor+1)
			if m.HopCursor >= len(m.DraftNodes) {
				m.HopCursor = max(0, len(m.DraftNodes)-1)
			}
		} else {
			m.DraftNodes = m.DraftNodes[:len(m.DraftNodes)-1]
			if m.HopCursor >= len(m.DraftNodes) {
				m.HopCursor = max(0, len(m.DraftNodes)-1)
			}
		}
		return s, nil

	case msg.String() == "[", msg.String() == "K", (m.Focus == chainFocusHops && key.Matches(msg, common.Keys.Left)):
		if len(m.DraftNodes) >= 2 && m.HopCursor > 0 && m.HopCursor < len(m.DraftNodes) {
			// Guard: a proxy-group cannot be moved past position 0.
			if m.HopCursor == 1 && s.isGroupName(m.DraftNodes[0]) {
				m.Notice = i18n.T("nodes.chain.group_entry_only")
				return s, nil
			}
			m.DraftNodes[m.HopCursor-1], m.DraftNodes[m.HopCursor] = m.DraftNodes[m.HopCursor], m.DraftNodes[m.HopCursor-1]
			m.HopCursor--
			m.HopScroll = min(m.HopScroll, m.HopCursor)
		}
		return s, nil

	case msg.String() == "]", msg.String() == "J", (m.Focus == chainFocusHops && key.Matches(msg, common.Keys.Right)):
		if len(m.DraftNodes) >= 2 && m.HopCursor >= 0 && m.HopCursor < len(m.DraftNodes)-1 {
			if m.HopCursor == 0 && s.isGroupName(m.DraftNodes[0]) {
				m.Notice = i18n.T("nodes.chain.group_entry_only")
				return s, nil
			}
			m.DraftNodes[m.HopCursor], m.DraftNodes[m.HopCursor+1] = m.DraftNodes[m.HopCursor+1], m.DraftNodes[m.HopCursor]
			m.HopCursor++
		}
		return s, nil

	case msg.String() == "x":
		m.DraftNodes = nil
		m.HopCursor = 0
		m.HopScroll = 0
		return s, nil

	case msg.String() == "ctrl+s", msg.String() == "s":
		return s.submitChainModal()
	}

	return s, nil
}

func (s State) isGroupName(name string) bool {
	_, ok := s.Groups[name]
	return ok
}

func (s State) submitChainModal() (State, tea.Cmd) {
	m := &s.ChainModal
	if len(m.DraftNodes) == 1 {
		m.Notice = i18n.T("nodes.chain.need_two_nodes")
		return s, nil
	}
	if len(m.DraftNodes) == 0 && !m.HadExisting {
		s.ChainModal = ChainModalState{}
		return s, nil
	}

	chain := service.ProxyChain{
		Group:   m.Group,
		Nodes:   append([]string(nil), m.DraftNodes...),
		Enabled: len(m.DraftNodes) >= 2,
	}
	if err := service.ValidateProxyChain(chain); err != nil {
		m.Notice = err.Error()
		return s, nil
	}

	m.Applying = true
	m.Notice = i18n.T("nodes.chain.applying")
	return s, s.applyProxyChainCmd(chain)
}

func (s State) applyProxyChainCmd(chain service.ProxyChain) tea.Cmd {
	configSvc := s.configSvc
	if configSvc == nil {
		configSvc = service.NewConfigService()
	}
	client := s.client
	coreSvc := s.coreSvc
	return func() tea.Msg {
		result := configSvc.ApplyProxyChain(context.Background(), coreSvc, client, chain)
		err := result.Err
		switch {
		case errors.Is(err, service.ErrRemoteChainEditRequiresSubscription):
			err = errors.New(i18n.T("nodes.chain.remote_needs_subscription"))
		case errors.Is(err, service.ErrChainProxyNotFound):
			err = fmt.Errorf("%s: %w", i18n.T("nodes.chain.proxy_not_found"), err)
		case errors.Is(err, service.ErrChainCycle):
			err = errors.New(i18n.T("nodes.chain.cycle_error"))
		case errors.Is(err, service.ErrChainConflict):
			err = fmt.Errorf("%s: %w", i18n.T("nodes.chain.conflict_error"), err)
		case err != nil && result.Restored:
			err = fmt.Errorf("%s (%w)", i18n.T("settings.toast.mihomo_override_restored"), err)
		}
		exitNode := ""
		if len(chain.Nodes) > 0 {
			exitNode = chain.Nodes[len(chain.Nodes)-1]
		}
		return messages.ProxyChainAppliedMsg{
			Group:           chain.Group,
			ExitNode:        exitNode,
			Cleared:         !chain.Enabled,
			Applied:         result.Applied,
			Remote:          result.Remote,
			NeedsRestart:    result.NeedsRestart,
			Restarted:       result.Restarted,
			EndpointChanged: result.EndpointChanged,
			Err:             err,
		}
	}
}

// ApplyChainResult updates modal & cached chain map when an async apply finishes.
func (s State) ApplyChainResult(msg messages.ProxyChainAppliedMsg) State {
	if msg.Err != nil {
		if s.ChainModal.Open {
			s.ChainModal.Applying = false
			s.ChainModal.Notice = msg.Err.Error()
		}
		return s
	}
	s.ChainModal = ChainModalState{}
	return s.ReloadChains()
}

func (s State) handleChainModalScroll(up bool) State {
	m := &s.ChainModal
	if m.Applying {
		return s
	}
	if m.Focus == chainFocusHops {
		if up && m.HopCursor > 0 {
			m.HopCursor--
			m.HopScroll = min(m.HopScroll, m.HopCursor)
		} else if !up && m.HopCursor < len(m.DraftNodes)-1 {
			m.HopCursor++
		}
		return s
	}
	if up && m.CandidateCursor > 0 {
		m.CandidateCursor--
		m.CandidateScroll = min(m.CandidateScroll, m.CandidateCursor)
	} else if !up && m.CandidateCursor < len(m.Candidates)-1 {
		m.CandidateCursor++
	}
	return s
}

func (s State) handleChainModalClick(pageX, pageY, pageWidth, pageHeight int) (State, tea.Cmd) {
	if s.ChainModal.Applying {
		return s, nil
	}
	pageState := s.ToPageState(pageWidth, pageHeight)
	modal := buildChainModal(pageState)
	overlayLines := strings.Split(modal, "\n")
	overlayH := len(overlayLines)
	overlayW := 0
	for _, l := range overlayLines {
		if w := common.DisplayWidth(l); w > overlayW {
			overlayW = w
		}
	}
	startRow := max(0, (pageHeight-overlayH)/2)
	startCol := max(0, (pageWidth-overlayW)/2)
	if pageY < startRow || pageY >= startRow+overlayH || pageX < startCol || pageX >= startCol+overlayW {
		s.ChainModal = ChainModalState{}
	}
	return s, nil
}
