package connections

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
)

func TestCloseFilteredActiveConnectionsClosesOnlyMatchedIDs(t *testing.T) {
	i18n.Init()

	var (
		mu          sync.Mutex
		closedPaths []string
		closeAllHit bool
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodDelete {
			if r.URL.Path == "/connections" {
				closeAllHit = true
			} else {
				closedPaths = append(closedPaths, r.URL.Path)
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/connections" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"connections":[]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := api.NewClient(config.MihomoEndpoint{ExternalController: strings.TrimPrefix(srv.URL, "http://")}, 5000)
	s := NewState("", nil, client, 5000, nil)
	s.connViewMode = ConnViewActive
	s.Connections = &model.ConnectionsResponse{
		Connections: []model.Connection{
			{ID: "id-udp-1", Metadata: model.Metadata{Host: "a.test", Network: "udp", Process: "chrome"}},
			{ID: "id-tcp-1", Metadata: model.Metadata{Host: "b.test", Network: "tcp", Process: "curl"}},
			{ID: "id-udp-2", Metadata: model.Metadata{Host: "c.test", Network: "udp", Process: "firefox"}},
		},
	}

	// Filter to UDP only (press 't' twice: All -> TCP -> UDP)
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})

	// Press 'X': must close ONLY id-udp-1 and id-udp-2, never DELETE /connections
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	if cmd == nil {
		t.Fatal("expected command for closing matched connections")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected BatchMsg, got %T", cmd())
	}
	var resultMsg messages.FilteredConnectionsClosedMsg
	var gotFilteredMsg bool
	for _, c := range batch {
		if msg, ok := c().(messages.FilteredConnectionsClosedMsg); ok {
			resultMsg = msg
			gotFilteredMsg = true
		}
	}
	if !gotFilteredMsg || resultMsg.Closed != 2 || resultMsg.Failed != 0 {
		t.Fatalf("expected FilteredConnectionsClosedMsg{Closed:2, Failed:0}, got ok=%t %+v", gotFilteredMsg, resultMsg)
	}

	mu.Lock()
	sort.Strings(closedPaths)
	gotAll := closeAllHit
	gotPaths := append([]string(nil), closedPaths...)
	mu.Unlock()

	if gotAll {
		t.Fatal("must not call DELETE /connections when a filter is active")
	}
	if strings.Join(gotPaths, ",") != "/connections/id-udp-1,/connections/id-udp-2" {
		t.Fatalf("unexpected closed paths: %v", gotPaths)
	}
}

func TestCloseFilteredActiveConnectionsWithZeroMatchesSendsNoDeletes(t *testing.T) {
	i18n.Init()
	deleteCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCount++
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := api.NewClient(config.MihomoEndpoint{ExternalController: strings.TrimPrefix(srv.URL, "http://")}, 5000)
	s := NewState("", nil, client, 5000, nil)
	s.connViewMode = ConnViewActive
	s.Connections = &model.ConnectionsResponse{
		Connections: []model.Connection{
			{ID: "id-tcp-1", Metadata: model.Metadata{Host: "b.test", Network: "tcp"}},
		},
	}
	s.protoFilter = "udp" // 0 matches

	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	if cmd == nil {
		t.Fatal("expected info notice when 0 connections match")
	}
	msg := cmd()
	if notice, ok := msg.(messages.NoticeMsg); !ok || notice.Level != messages.NoticeInfo {
		t.Fatalf("expected info NoticeMsg, got %#v", msg)
	}
	if deleteCount != 0 {
		t.Fatalf("expected 0 DELETE calls, got %d", deleteCount)
	}
}

func TestCloseAllActiveConnectionsWithoutFilterUsesBulkEndpoint(t *testing.T) {
	i18n.Init()
	closeAllHit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && r.URL.Path == "/connections" {
			closeAllHit = true
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := api.NewClient(config.MihomoEndpoint{ExternalController: strings.TrimPrefix(srv.URL, "http://")}, 5000)
	s := NewState("", nil, client, 5000, nil)
	s.connViewMode = ConnViewActive
	s.Connections = &model.ConnectionsResponse{
		Connections: []model.Connection{
			{ID: "c1", Metadata: model.Metadata{Host: "a.test", Network: "tcp"}},
		},
	}

	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	for _, c := range cmd().(tea.BatchMsg) {
		_ = c()
	}
	if !closeAllHit {
		t.Fatal("expected DELETE /connections when no filter is active")
	}
}
