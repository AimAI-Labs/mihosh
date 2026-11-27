package preferences

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestPreferencesNarrowLayoutFitsWidthAndKeepsEnumOptionsVisible(t *testing.T) {
	i18n.Init()
	cfg := service.DefaultConfig
	for _, tab := range []int{0, 1} {
		state := PageState{
			Config:       &cfg,
			ActiveTab:    tab,
			MihomoLoaded: true,
			MihomoOverrides: map[string]string{
				"external-controller": "127.0.0.1:9090",
				"secret":              "my-secret",
				"mixed-port":          "7890",
				"allow-lan":           "true",
				"log-level":           "info",
			},
		}
		rendered := RenderSettingsPage(state, 32, 30)
		for _, line := range strings.Split(rendered, "\n") {
			require.LessOrEqual(t, lipgloss.Width(line), 32, "tab %d overflowed width 32: %q", tab, ansi.Strip(line))
		}
		if tab == 1 {
			plain := ansi.Strip(rendered)
			require.Contains(t, plain, "true")
			require.Contains(t, plain, "info")
		}
	}

	bar := renderSettingsTabBar([]string{"Very Long Settings Tab Title For Narrow Screen"}, 0, 28)
	barLines := strings.Split(bar, "\n")
	require.Len(t, barLines, 3)
	for _, line := range barLines {
		require.Equal(t, 28, lipgloss.Width(line))
	}
}
