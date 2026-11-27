package preferences

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestThemeModalOpenNavigatePreviewAndCancel(t *testing.T) {
	t.Cleanup(func() { theme.SetTheme("tokyo-night") })
	svc, cfg := setupTestConfig(t, nil)
	cfg.Theme = "tokyo-night"
	theme.SetTheme("tokyo-night")

	s := State{
		selectedSetting: ThemeSettingIndex(),
		Config:          cfg,
		configSvc:       svc,
	}

	// Pressing Enter on the theme row opens the modal instead of inline edit mode.
	next, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.Nil(t, cmd)
	require.True(t, next.ThemeModalOpen())
	require.False(t, next.IsEditing())
	require.True(t, next.InputCapture().Active)
	require.Equal(t, 0, next.themeModal.Cursor)

	// Navigating changes preview focus without mutating active global theme or saved config.
	next, _ = next.Update(tea.KeyMsg{Type: tea.KeyDown})
	next, _ = next.Update(tea.KeyMsg{Type: tea.KeyDown})
	require.Equal(t, 2, next.themeModal.Cursor)
	require.Equal(t, "tokyo-night", theme.CurrentName())
	require.Equal(t, "tokyo-night", next.Config.Theme)

	rendered := ansi.Strip(RenderSettingsPage(next.ToPageState(next.Config, nil), 100, 30))
	require.Contains(t, rendered, theme.Names()[2])

	// Pressing Esc closes modal without persisting.
	next, cmd = next.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.Nil(t, cmd)
	require.False(t, next.ThemeModalOpen())
	require.Equal(t, "tokyo-night", theme.CurrentName())
	require.Equal(t, "tokyo-night", next.Config.Theme)
}

func TestThemeModalApplyPersistsAndEmitsThemeChanged(t *testing.T) {
	t.Cleanup(func() { theme.SetTheme("tokyo-night") })
	svc, cfg := setupTestConfig(t, nil)
	cfg.Theme = "tokyo-night"
	theme.SetTheme("tokyo-night")

	s := State{
		selectedSetting: ThemeSettingIndex(),
		Config:          cfg,
		configSvc:       svc,
	}.OpenThemeModal()

	// Jump to last theme ("everforest") via End key and apply with Enter.
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnd})
	want := theme.Names()[len(theme.Names())-1]
	require.Equal(t, len(theme.Names())-1, s.themeModal.Cursor)

	s, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.False(t, s.ThemeModalOpen())
	require.Equal(t, want, theme.CurrentName())
	require.Equal(t, want, s.Config.Theme)
	require.NotNil(t, cmd)

	// Verify ThemeChangedMsg is emitted inside the batch.
	batchMsg := cmd()
	batch, ok := batchMsg.(tea.BatchMsg)
	require.True(t, ok, "expected tea.BatchMsg, got %T", batchMsg)
	foundChanged := false
	for _, sub := range batch {
		if sub != nil {
			if _, ok := sub().(messages.ThemeChangedMsg); ok {
				foundChanged = true
			}
		}
	}
	require.True(t, foundChanged, "expected ThemeChangedMsg in batch")
}

func TestThemeModalMouseSelectionAndOutsideClose(t *testing.T) {
	t.Cleanup(func() { theme.SetTheme("tokyo-night") })
	svc, cfg := setupTestConfig(t, nil)
	cfg.Theme = "tokyo-night"
	theme.SetTheme("tokyo-night")

	s := State{
		selectedSetting: ThemeSettingIndex(),
		Config:          cfg,
		configSvc:       svc,
	}.OpenThemeModal()

	const width, height = 100, 30
	modal := buildThemeModal(s.ToPageState(s.Config, nil), width, height)
	left, top, _, _ := modal.Bounds()

	// First click on row 3 highlights it for preview without applying yet.
	clickX := left + 2 + 2
	clickY := top + modal.listTop + 3
	s, cmd := s.Update(messages.PageMouseClickMsg{X: clickX, Y: clickY, Width: width, Height: height})
	require.Nil(t, cmd)
	require.True(t, s.ThemeModalOpen())
	require.Equal(t, 3, s.themeModal.Cursor)
	require.Equal(t, "tokyo-night", theme.CurrentName())

	// Second click on the already-focused item applies the theme.
	s, cmd = s.Update(messages.PageMouseClickMsg{X: clickX, Y: clickY, Width: width, Height: height})
	require.NotNil(t, cmd)
	require.False(t, s.ThemeModalOpen())
	require.Equal(t, theme.Names()[3], theme.CurrentName())

	// Re-open and click outside bounds closes without changes.
	s = s.OpenThemeModal()
	require.True(t, s.ThemeModalOpen())
	s, cmd = s.Update(messages.PageMouseClickMsg{X: 0, Y: 0, Width: width, Height: height})
	require.Nil(t, cmd)
	require.False(t, s.ThemeModalOpen())
}

func TestThemeModalScrollAndViewportBounds(t *testing.T) {
	t.Cleanup(func() { theme.SetTheme("tokyo-night") })
	svc, cfg := setupTestConfig(t, nil)

	s := State{
		selectedSetting: ThemeSettingIndex(),
		Config:          cfg,
		configSvc:       svc,
	}.OpenThemeModal()

	// Scroll down updates cursor.
	s, _ = s.Update(messages.PageMouseScrollMsg{Up: false})
	require.Equal(t, 1, s.themeModal.Cursor)
	s, _ = s.Update(messages.PageMouseScrollMsg{Up: true})
	require.Equal(t, 0, s.themeModal.Cursor)

	// Render across wide, medium, narrow, and short viewports without overflowing bounds.
	for _, vp := range [][2]int{{120, 34}, {80, 24}, {54, 18}, {36, 14}} {
		w, h := vp[0], vp[1]
		out := RenderSettingsPage(s.ToPageState(s.Config, nil), w, h)
		lines := strings.Split(out, "\n")
		require.LessOrEqual(t, len(lines), h, "height overflow at %dx%d", w, h)
		for i, line := range lines {
			require.LessOrEqual(t, lipgloss.Width(line), w, "width overflow on line %d at %dx%d", i, w, h)
		}
	}
}
