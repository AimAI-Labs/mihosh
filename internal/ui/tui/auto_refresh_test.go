package tui

import (
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/nodes"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
)

func TestDetectAutoRefreshChangeDetectsModeChange(t *testing.T) {
	state := nodes.State{Mode: "rule"}
	result := autoRefreshResult{Mode: "global"}

	if !detectAutoRefreshChange(state, result) {
		t.Fatalf("expected mode change to be detected")
	}
}

func TestDetectAutoRefreshChangeDetectsSelectedProxyChange(t *testing.T) {
	state := nodes.State{
		Groups: map[string]model.Group{
			"Proxy": {Name: "Proxy", Now: "HK-01", All: []string{"HK-01", "JP-01"}},
		},
	}
	result := autoRefreshResult{
		Groups: map[string]model.Group{
			"Proxy": {Name: "Proxy", Now: "JP-01", All: []string{"HK-01", "JP-01"}},
		},
	}

	if !detectAutoRefreshChange(state, result) {
		t.Fatalf("expected selected proxy change to be detected")
	}
}

func TestDetectAutoRefreshChangeIgnoresUnchangedData(t *testing.T) {
	state := nodes.State{
		Mode: "rule",
		Groups: map[string]model.Group{
			"Proxy": {Name: "Proxy", Now: "HK-01", All: []string{"HK-01", "JP-01"}},
		},
		GroupNames: []string{"Proxy"},
	}
	result := autoRefreshResult{
		Mode: "rule",
		Groups: map[string]model.Group{
			"Proxy": {Name: "Proxy", Now: "HK-01", All: []string{"HK-01", "JP-01"}},
		},
		OrderedNames: []string{"Proxy"},
	}

	if detectAutoRefreshChange(state, result) {
		t.Fatalf("expected unchanged data to be ignored")
	}
}

func TestAutoRefreshMessageShowsSyncedCheckmarkAfterSuccessfulRefresh(t *testing.T) {
	model := Model{
		config: &config.Config{AutoRefreshInterval: 5},
	}

	next, _ := model.Update(autoRefreshMsg{Changed: false})
	got := next.(Model)

	if !got.autoRefreshSynced {
		t.Fatalf("expected successful auto refresh to show synced checkmark")
	}

	next, _ = got.Update(messages.AutoRefreshTickMsg(time.Now()))
	got = next.(Model)
	if got.autoRefreshSynced {
		t.Fatalf("expected synced checkmark to clear after one tick")
	}
}

func TestAutoRefreshNoticeExpiresOnTicks(t *testing.T) {
	model := Model{
		config: &config.Config{AutoRefreshInterval: 5},
	}

	next, _ := model.Update(autoRefreshMsg{Changed: true})
	got := next.(Model)
	if got.notice == "" {
		t.Fatalf("expected changed refresh to set notice")
	}

	for i := 0; i < autoRefreshNoticeTicks; i++ {
		next, _ = got.Update(messages.AutoRefreshTickMsg(time.Now()))
		got = next.(Model)
	}

	if got.notice != "" {
		t.Fatalf("expected auto refresh notice to expire, got %q", got.notice)
	}
}
