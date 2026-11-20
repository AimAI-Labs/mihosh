package sub

import (
	"strings"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestSubHistoryModalListDiffAndRollback(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	i18n.Init()
	if err := config.Save(&config.DefaultConfig); err != nil {
		t.Fatal(err)
	}
	svc := service.NewProfileService(nil)
	p, err := svc.AddProfile("Alpha", profile.SubSource{Kind: profile.SourceRemote, URL: "https://example.org/a.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	if err := profile.WriteRaw(p.UID, []byte("proxies:\n  - name: old-node\n")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	if err := profile.WriteRaw(p.UID, []byte("proxies:\n  - name: new-node\n")); err != nil {
		t.Fatal(err)
	}

	s := NewState(svc).ApplySubs([]profile.Profile{p}, "")
	s, _ = s.Update(messages.PageResizeMsg{Width: 100, Height: 26})

	// Open history modal with 'H'
	s, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("H")})
	if s.history.mode != historyList || cmd == nil || !s.Querying() {
		t.Fatal("expected H to open history modal and load snapshots")
	}
	s, _ = s.Update(cmd())
	if len(s.history.snapshots) != 1 {
		t.Fatalf("expected 1 raw snapshot, got %d", len(s.history.snapshots))
	}

	// Render list view cleanly
	renderedList := ansi.Strip(RenderSubPage(s.ToPageState(100, 26)))
	if !strings.Contains(renderedList, i18n.T("sub.history.title")) || !strings.Contains(renderedList, s.history.snapshots[0].SHA256) {
		t.Fatalf("history list modal missing title or snapshot hash:\n%s", renderedList)
	}

	// Press Enter to view diff
	s, diffCmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if s.history.mode != historyDiff || diffCmd == nil {
		t.Fatal("expected Enter to enter diff view")
	}
	s, _ = s.Update(diffCmd())
	if len(s.history.diff) == 0 {
		t.Fatal("expected non-empty snapshot diff")
	}
	renderedDiff := ansi.Strip(RenderSubPage(s.ToPageState(100, 26)))
	if !strings.Contains(renderedDiff, "old-node") || !strings.Contains(renderedDiff, "new-node") {
		t.Fatalf("diff modal did not show old/new lines:\n%s", renderedDiff)
	}

	// Press 'r' to rollback
	s, rbCmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if s.history.mode != historyClosed || rbCmd == nil {
		t.Fatal("expected r to trigger rollback and close modal")
	}
	rbMsg, ok := rbCmd().(messages.SubRollbackDoneMsg)
	if !ok || rbMsg.Err != nil {
		t.Fatalf("rollback failed: %#v", rbMsg)
	}
	restored, err := profile.ReadRawFile(p.UID)
	if err != nil || !strings.Contains(string(restored), "old-node") {
		t.Fatalf("expected raw.yaml restored to old-node, got %q err=%v", string(restored), err)
	}
}
