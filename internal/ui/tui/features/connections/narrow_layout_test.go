package connections

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections/components"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/require"
)

func TestConnectionsNarrowLayoutFitsSingleLineWithinWidth(t *testing.T) {
	i18n.Init()
	conn := model.Connection{
		ID:       "conn-1",
		Download: 2048,
		Upload:   1024,
		Rule:     "DOMAIN-SUFFIX",
		Chains:   []string{"Node-A", "ProxyGroup-LongName"},
		Metadata: model.Metadata{
			Host:    "very-long-subdomain.example.com",
			Network: "tcp",
			Type:    "HTTPS",
		},
	}

	for _, width := range []int{32, 60} {
		bar := RenderConnModeSwitchComponent(ConnViewActive, width, true, true, 5)
		barLines := strings.Split(bar, "\n")
		require.Len(t, barLines, 3)
		for _, line := range barLines {
			require.Equal(t, width, lipgloss.Width(line), "mode switch bar width mismatch at width=%d", width)
		}

		header := components.RenderTableHeader(lipgloss.NewStyle(), width)
		require.NotContains(t, header, "\n")
		require.LessOrEqual(t, lipgloss.Width(header), width)

		row := components.RenderConnectionRow(conn, lipgloss.NewStyle(), "► ", width)
		require.NotContains(t, row, "\n")
		require.LessOrEqual(t, lipgloss.Width(row), width)
	}
}
