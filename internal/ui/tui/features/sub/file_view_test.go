package sub

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestInactiveProfileDiffUsesReadOnlySourceAndResponsivePanes(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	source := []byte("mode: rule\n")
	require.NoError(t, profile.WriteRaw("test", source))
	require.NoError(t, profile.WriteMerge([]byte("mode: global\n")))
	s := State{}.ApplySubs([]profile.Profile{{UID: "test", Name: "Inactive"}}, "")
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	require.True(t, s.Querying())
	for range 2 {
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	s, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	result := cmd().(messages.SubFileLoadedMsg)
	require.NoError(t, result.Err)
	s, _ = s.Update(result)
	for _, width := range []int{80, 120} {
		old, new := -1, -1
		for index, line := range strings.Split(ansi.Strip(RenderSubPage(s.ToPageState(width, 26))), "\n") {
			if strings.Contains(line, "- mode: rule") {
				old = index
			}
			if strings.Contains(line, "+ mode: global") {
				new = index
			}
		}
		require.GreaterOrEqual(t, old, 0)
		if width >= 100 {
			require.Equal(t, old, new)
		} else {
			require.Greater(t, new, old)
		}
	}
	read, err := profile.ReadRawFile("test")
	require.NoError(t, err)
	require.Equal(t, source, read)
}

func TestDiffOptionsMenuRecomputesWithKeyboardAndMouse(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	require.NoError(t, profile.WriteRaw("test", []byte("mode:  rule\ndns:\n    enable: true\n")))
	i18n.Init()
	const width, height = 80, 24
	s := State{file: fileView{mode: fileMenu, uid: "test", selected: 2}}
	s, cmd := s.loadFileView()
	s, _ = s.Update(cmd())
	require.NotEmpty(t, s.file.diff)
	s, _ = s.handleFileKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	require.True(t, s.file.optionsOpen)
	s, cmd = s.handleFileKey(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	indentOnly := cmd()
	left, top, _, _ := s.file.dialog(width, height).Bounds()
	s, cmd = s.clickFile(messages.PageMouseClickMsg{X: left + 2, Y: top + 2, Width: width, Height: height})
	require.NotNil(t, cmd)
	s, _ = s.Update(indentOnly)
	require.True(t, s.file.loading)
	s, _ = s.Update(cmd())
	require.Empty(t, s.file.diff)
	s, _ = s.handleFileKey(tea.KeyMsg{Type: tea.KeyEsc})
	require.False(t, s.file.optionsOpen)
	require.Contains(t, ansi.Strip(s.file.renderFile(60, width, height)), i18n.T("sub.file.same"))
}

func TestDiffOptionsPersistAcrossSubscriptionsAndAllCanBeDisabled(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	subs := []profile.Profile{{UID: "first"}, {UID: "second"}}
	s := State{}.ApplySubs(subs, "").openFileMenu()
	require.Equal(t, profile.DefaultFileDiffOptions, s.file.ignore)
	s.file.mode, s.file.selected = fileDiff, 2
	for _, width := range []int{80, 120} {
		const height = 24
		left, _, _, bottom := s.file.dialog(width, height).Bounds()
		next, _ := s.clickFile(messages.PageMouseClickMsg{X: left + 2, Y: bottom - 2, Width: width, Height: height})
		require.True(t, next.file.optionsOpen)
	}
	for option := range diffOptions {
		s, _ = s.toggleDiffOption(option)
		require.NoError(t, s.file.err)
	}
	reopened := State{}.ApplySubs(subs, "")
	reopened.selected = 1
	reopened = reopened.openFileMenu()
	require.Equal(t, "second", reopened.file.uid)
	require.Zero(t, reopened.file.ignore)
}
