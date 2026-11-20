package rules

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func TestRuleFormsPersistAndApplyMergeEdits(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	var payload atomic.Value
	var reject atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPut || req.URL.Path != "/configs" {
			http.NotFound(w, req)
			return
		}
		if reject.Load() {
			http.Error(w, "invalid candidate", http.StatusBadRequest)
			return
		}
		var body struct{ Payload string }
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload.Store(body.Payload)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	cfg := config.DefaultConfig
	cfg.Core.Mode, cfg.Core.ExternalController = "external", server.Listener.Addr().String()
	require.NoError(t, config.Save(&cfg))
	client := api.NewClient(config.ResolveMihomoEndpoint(), 500)
	profiles := service.NewProfileService(client)
	source, err := profiles.AddProfile("remote test", profile.SubSource{Kind: profile.SourceLocal, Path: "unused"})
	require.NoError(t, err)
	raw := []byte("rules:\n  - DOMAIN,old.example,DIRECT\n  - MATCH,DIRECT\n")
	require.NoError(t, profile.WriteRaw(source.UID, raw))
	require.NoError(t, profiles.SetActiveSub(source.UID))
	require.NoError(t, profile.WriteMerge([]byte("# keep\nmode: rule\n")))

	s := NewState(client, service.NewConfigService(), nil).ApplyRules([]model.Rule{{Type: "Domain", Payload: "old.example", Proxy: "DIRECT"}})
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	runeKey := func(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }
	s, _ = s.Update(runeKey('n'))
	s.addForm.fields[addFieldPayload].SetValue("new.example")
	s, cmd := s.Update(enter)
	require.NotNil(t, cmd)
	outcome := cmd().(messages.RuleEditAppliedMsg)
	require.NoError(t, outcome.Err)
	require.True(t, outcome.Remote && outcome.Applied)
	candidate := payload.Load().(string)
	require.True(t, strings.Index(candidate, "new.example") < strings.Index(candidate, "old.example"))
	require.NotContains(t, candidate, "rule-edits")

	s, _ = s.Update(enter)
	s.addForm.fields[addFieldPayload].SetValue("edited.example")
	s, cmd = s.Update(enter)
	require.NotNil(t, cmd)
	require.NoError(t, cmd().(messages.RuleEditAppliedMsg).Err)
	candidate = payload.Load().(string)
	require.Contains(t, candidate, "edited.example")
	require.NotContains(t, candidate, "old.example")

	s = s.ApplyRules([]model.Rule{{Type: "Domain", Payload: "edited.example", Proxy: "DIRECT"}})
	s, _ = s.Update(runeKey('d'))
	s, cmd = s.Update(enter)
	require.NotNil(t, cmd)
	require.NoError(t, cmd().(messages.RuleEditAppliedMsg).Err)
	require.NotContains(t, payload.Load().(string), "edited.example")
	merge, err := profile.ReadMerge()
	require.NoError(t, err)
	require.Contains(t, string(merge), "# keep")
	require.Contains(t, string(merge), "rules:")
	require.NotContains(t, string(merge), "rule-edits:")
	require.NotContains(t, string(merge), "edited.example")
	require.Contains(t, string(merge), "MATCH,DIRECT")

	reject.Store(true)
	s, _ = s.Update(runeKey('n'))
	s.addForm.fields[addFieldPayload].SetValue("rollback.example")
	_, cmd = s.Update(enter)
	require.Error(t, cmd().(messages.RuleEditAppliedMsg).Err)
	after, err := profile.ReadMerge()
	require.NoError(t, err)
	require.Equal(t, merge, after)
	untouched, err := profile.ReadRawFile(source.UID)
	require.NoError(t, err)
	require.Equal(t, raw, untouched)
}
