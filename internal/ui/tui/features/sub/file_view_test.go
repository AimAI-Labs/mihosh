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
