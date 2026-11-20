package connections

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestDNSQueryModalPrefillsSelectedHostQueriesAndDetectsFakeIP(t *testing.T) {
	i18n.Init()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dns/query" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"Status": 0,
			"Answer": [
				{"name": "api.example.com.", "type": 1, "TTL": 30, "data": "198.18.0.42"}
			]
		}`))
	}))
	defer srv.Close()

	client := api.NewClient(config.MihomoEndpoint{ExternalController: strings.TrimPrefix(srv.URL, "http://")}, 5000)
	s := NewState("", nil, client, 5000, nil)
	s.connViewMode = ConnViewActive
	s.Connections = &model.ConnectionsResponse{
		Connections: []model.Connection{
			{ID: "c1", Metadata: model.Metadata{Host: "api.example.com", Network: "tcp"}},
		},
	}

	// Press 'D' to open DNS query modal; selected host is prefilled.
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	if !s.dns.open || !s.InputCapture().Active || !s.InputCapture().Help {
		t.Fatal("expected DNS modal open and capturing input")
	}
	if s.dns.input.Value() != "api.example.com" {
		t.Fatalf("expected prefilled domain api.example.com, got %q", s.dns.input.Value())
	}

	// Tab switches record type A -> AAAA, Shift+Tab back to A.
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyTab})
	if s.dns.currentRecordType() != "AAAA" {
		t.Fatalf("expected AAAA after Tab, got %s", s.dns.currentRecordType())
	}
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if s.dns.currentRecordType() != "A" {
		t.Fatalf("expected A after Shift+Tab, got %s", s.dns.currentRecordType())
	}

	// Enter submits query against mock /dns/query controller.
	s, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !s.dns.loading || cmd == nil {
		t.Fatal("expected Enter to trigger DNS query command")
	}
	rawMsg := cmd()
	if httpMsg, ok := rawMsg.(messages.CoreHTTPMsg); ok {
		rawMsg = httpMsg.Payload
	}
	s, _ = s.Update(rawMsg)
	if s.dns.loading || s.dns.err != nil || s.dns.resp == nil || !s.dns.resp.HasFakeIP() {
		t.Fatalf("expected Fake-IP DNS response, got loading=%t err=%v resp=%+v", s.dns.loading, s.dns.err, s.dns.resp)
	}

	rendered := ansi.Strip(RenderConnectionsPage(s.ToPageState(nil, 100, 28)))
	if !strings.Contains(rendered, "198.18.0.42") || !strings.Contains(rendered, "Fake-IP") {
		t.Fatalf("expected Fake-IP answer in rendered DNS modal:\n%s", rendered)
	}

	// Esc closes the modal cleanly.
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if s.dns.open || s.InputCapture().Active {
		t.Fatal("expected Esc to close DNS modal")
	}
}
