package common

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/require"
)

func TestCardRestoresThemeColorsAfterColoredSpan(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })

	palette := theme.Current()
	label := lipgloss.NewStyle().Foreground(theme.Resolve(palette.Success)).Render("Ready")
	card := RenderCard(32, palette.Border, palette.Surface, []string{label + " text"})
	reset := ansi.SGR(0)
	bodyStyle := lipgloss.NewStyle().
		Background(theme.Resolve(palette.Surface)).
		Foreground(theme.Resolve(palette.Foreground))
	prefix := strings.TrimSuffix(bodyStyle.Render(""), reset)
	require.NotEmpty(t, prefix)
	require.Contains(t, card, label+prefix+" text")
	require.Equal(t, 32, lipgloss.Width(card))
}
