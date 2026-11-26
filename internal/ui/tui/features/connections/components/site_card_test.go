package components

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestAILatencyCardsRenderDistinctIcons(t *testing.T) {
	sites := make(map[string]model.SiteTest)
	for _, site := range model.DefaultSiteTests() {
		sites[site.Name] = site
	}
	for name, icon := range map[string]string{"ChatGPT": "✿", "Claude": "✶"} {
		t.Run(name, func(t *testing.T) {
			require.Contains(t, sites, name)
			card := ansi.Strip(RenderSiteCard(sites[name], false, 12))
			require.Contains(t, card, icon)
		})
	}
}
