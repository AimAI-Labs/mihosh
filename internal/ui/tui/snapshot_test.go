package tui

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

var updateTUISnapshots = flag.Bool("update-tui-snapshots", false, "update TUI snapshot files")

func TestTUISnapshotNodesAndHelp(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())

	m := newTestModel(nil, "http://127.0.0.1:1", 100)
	t.Cleanup(m.Close)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)
	m.currentPage = layout.PageNodes
	snapshotTUI(t, "nodes_initial", m.View())

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(Model)
	snapshotTUI(t, "nodes_help", m.View())
}

func TestTUISnapshotCorePage(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())

	m := newTestModel(nil, "http://127.0.0.1:1", 100)
	t.Cleanup(m.Close)
	m.currentPage = layout.PageCore
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)
	snapshotTUI(t, "core_initial", m.View())
}

func snapshotTUI(t *testing.T, name, actual string) {
	t.Helper()
	actual = normalizeTUISnapshot(actual)
	path := filepath.Join("testdata", "snapshots", name+".golden")
	if *updateTUISnapshots {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(actual), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read TUI snapshot %s: %v, run with -update-tui-snapshots", path, err)
	}
	if string(want) != actual {
		t.Fatalf("TUI snapshot %s changed\n--- want ---\n%s\n--- got ---\n%s", name, want, actual)
	}
}

func normalizeTUISnapshot(value string) string {
	lines := strings.Split(strings.ReplaceAll(ansi.Strip(value), "\r\n", "\n"), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
}
