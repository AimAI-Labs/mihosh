package connections

import (
	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type dnsQueryView struct {
	open       bool
	input      textinput.Model
	typeIdx    int
	loading    bool
	lastDomain string
	lastType   string
	resp       *model.DNSQueryResponse
	err        error
	scroll     int
}

func newDNSInput() textinput.Model {
	in := textinput.New()
	in.Placeholder = i18n.T("conns.dns.domain_placeholder")
	in.CharLimit = 253
	in.Width = 44
	return in
}

func (s *State) openDNSQuery() tea.Cmd {
	if s.dns.input.CharLimit == 0 {
		s.dns.input = newDNSInput()
	}
	s.dns.input.Placeholder = i18n.T("conns.dns.domain_placeholder")
	s.dns.open = true
	s.dns.scroll = 0
	if conn := s.selectedConnection(); conn != nil && conn.Metadata.Host != "" {
		s.dns.input.SetValue(conn.Metadata.Host)
		s.dns.input.CursorEnd()
	}
	s.dns.input.Focus()
	return textinput.Blink
}

func (s *State) closeDNSQuery() {
	s.dns.open = false
	s.dns.loading = false
	s.dns.input.Blur()
}

func (d dnsQueryView) currentRecordType() string {
	if d.typeIdx < 0 || d.typeIdx >= len(model.DNSRecordTypes) {
		return "A"
	}
	return model.DNSRecordTypes[d.typeIdx]
}

// QueryDNS executes Mihomo /dns/query asynchronously.
func QueryDNS(client *service.Client, domain, recordType string) tea.Cmd {
	return messages.CoreRequest(client, func(client *service.Client) tea.Msg {
		resp, err := client.QueryDNS(domain, recordType)
		return messages.DNSQueryResultMsg{
			Domain:     domain,
			RecordType: recordType,
			Resp:       resp,
			Err:        err,
		}
	})
}

func (s State) submitDNSQuery() (State, tea.Cmd) {
	domain := strings.TrimSpace(s.dns.input.Value())
	if domain == "" {
		return s, nil
	}
	qtype := s.dns.currentRecordType()
	s.dns.loading = true
	s.dns.lastDomain = domain
	s.dns.lastType = qtype
	s.dns.err = nil
	s.dns.scroll = 0
	return s, QueryDNS(s.client, domain, qtype)
}

func (s State) applyDNSQueryResult(msg messages.DNSQueryResultMsg) (State, tea.Cmd) {
	if !s.dns.open || msg.Domain != s.dns.lastDomain || msg.RecordType != s.dns.lastType {
		return s, nil
	}
	s.dns.loading = false
	s.dns.resp = msg.Resp
	s.dns.err = msg.Err
	s.dns.scroll = 0
	return s, nil
}

func (s State) handleDNSKey(msg tea.KeyMsg) (State, tea.Cmd) {
	switch {
	case key.Matches(msg, common.Keys.Escape):
		s.closeDNSQuery()
		return s, nil
	case msg.String() == "tab":
		s.dns.typeIdx = (s.dns.typeIdx + 1) % len(model.DNSRecordTypes)
		return s, nil
	case msg.String() == "shift+tab":
		s.dns.typeIdx = (s.dns.typeIdx - 1 + len(model.DNSRecordTypes)) % len(model.DNSRecordTypes)
		return s, nil
	case msg.String() == "up":
		if s.dns.scroll > 0 {
			s.dns.scroll--
		}
		return s, nil
	case msg.String() == "down":
		s.dns.scroll++
		return s, nil
	case key.Matches(msg, common.Keys.Enter):
		return s.submitDNSQuery()
	}
	var cmd tea.Cmd
	s.dns.input, cmd = s.dns.input.Update(msg)
	return s, cmd
}
