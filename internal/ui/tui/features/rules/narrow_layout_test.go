package rules

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/require"
)

func TestRulesNarrowLayoutDoesNotWrapRowsOrHeader(t *testing.T) {
	i18n.Init()
	items := make([]filteredRule, 0, 10)
	for i := 0; i < 10; i++ {
		items = append(items, filteredRule{
			Index: i,
			Rule: model.Rule{
				Type:      "DOMAIN-SUFFIX",
				Payload:   "very-long-domain-name.example.org",
				Proxy:     "VeryLongProxyGroupName",
				NoResolve: true,
			},
		})
	}

	for _, width := range []int{32, 60} {
		searchBox := renderRuleSearchBox("long-filter-query-string", true, FilterEngineFuzzy, []string{"DOMAIN-SUFFIX", "IP-CIDR"})
		header := RenderRulesHeaderComponent(searchBox, "10/10", width)
		_ = header
		header = RenderRulesHeaderComponent("10/10 rules", searchBox, width)
		headerLines := strings.Split(header, "\n")
		require.Len(t, headerLines, 3)
		for _, line := range headerLines {
			require.Equal(t, width, lipgloss.Width(line), "rules header width mismatch at width=%d", width)
		}

		rendered := renderRuleList(items, 0, 0, 5, width, 0, 0)
		listLines := strings.Split(rendered, "\n")
		require.Len(t, listLines, 5)
		for _, line := range listLines {
			require.LessOrEqual(t, lipgloss.Width(line), width, "rule row overflow at width=%d", width)
		}
	}
}
