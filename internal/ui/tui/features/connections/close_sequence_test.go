package connections

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

type closeSequenceProbe struct {
	command   tea.Cmd
	refreshed bool
	closed    tea.Msg
}

func (m closeSequenceProbe) Init() tea.Cmd { return m.command }
func (m closeSequenceProbe) View() string  { return "" }
func (m closeSequenceProbe) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if response, ok := msg.(messages.CoreHTTPMsg); ok {
		switch response.Payload.(type) {
		case messages.ConnectionsMsg:
			m.refreshed = true
			return m, tea.Quit
		case messages.ConnectionClosedMsg, messages.AllConnectionsClosedMsg, messages.FilteredConnectionsClosedMsg:
			m.closed = response.Payload
		}
	}
	return m, nil
}

func runCloseSequence(t *testing.T, cmd tea.Cmd) closeSequenceProbe {
	t.Helper()
	require.NotNil(t, cmd)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	p := tea.NewProgram(closeSequenceProbe{command: cmd}, tea.WithContext(ctx), tea.WithInput(nil), tea.WithoutRenderer(), tea.WithoutSignalHandler())
	end, err := p.Run()
	require.NoError(t, err)
	probe := end.(closeSequenceProbe)
	require.True(t, probe.refreshed)
	return probe
}

func TestConnectionCloseRefreshWaitsForDelete(t *testing.T) {
	for _, key := range []string{"x", "X"} {
		t.Run(key, func(t *testing.T) {
			var closed, refreshedTooEarly atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				switch req.Method {
				case http.MethodDelete:
					time.Sleep(25 * time.Millisecond)
					closed.Store(true)
					w.WriteHeader(http.StatusNoContent)
				case http.MethodGet:
					refreshedTooEarly.Store(!closed.Load())
					_, _ = w.Write([]byte(`{"connections":[]}`))
				}
			}))
			defer server.Close()
			client := api.NewClient(config.MihomoEndpoint{ExternalController: server.URL}, 1000)
			s := NewState("", nil, client, 1000, nil).ApplyConnections(&model.ConnectionsResponse{
				Connections: []model.Connection{{ID: "fixture"}},
			})
			s.connViewMode = ConnViewActive
			_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
			runCloseSequence(t, cmd)
			require.False(t, refreshedTooEarly.Load(), "connections were read before the close completed")
		})
	}
}

func TestFilteredCloseBindsControllerBeforeExecution(t *testing.T) {
	var closed atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodDelete && req.URL.Path == "/connections/fixture" {
			closed.Add(1)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client := api.NewClient(config.MihomoEndpoint{ExternalController: server.URL}, 1000)
	generation := client.Generation()
	cmd := CloseConnectionsByIDs(client, []string{"fixture", "fixture", ""})
	client.UpdateEndpoint("http://127.0.0.1:0", "")
	msg, ok := cmd().(messages.CoreHTTPMsg)
	require.True(t, ok, "batch close result has no generation")
	require.Equal(t, generation, msg.Generation)
	require.Equal(t, messages.FilteredConnectionsClosedMsg{Closed: 1}, msg.Payload)
	require.EqualValues(t, 1, closed.Load())
}
