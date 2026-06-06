package rules

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/charmbracelet/lipgloss"
)

func TestRenderRuleList_AppendsScrollbarWithoutGap(t *testing.T) {
	rules := make([]filteredRule, 0, 8)
	for i := 0; i < 8; i++ {
		rules = append(rules, filteredRule{
			Index: i,
			Rule: model.Rule{
				Type:    "DOMAIN",
				Payload: "example.com",
				Proxy:   "Proxy",
			},
		})
	}

	rendered := renderRuleList(rules, 0, 0, 4, 80, 0, 0)
	lines := strings.Split(rendered, "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 rendered rows, got %d", len(lines))
	}

	for i, line := range lines {
		plain := lipgloss.NewStyle().Render(line)
		if !strings.HasSuffix(plain, common.SymbolScrollbarTrack) && !strings.HasSuffix(plain, common.SymbolScrollbarThumb) {
			t.Fatalf("line %d should end with scrollbar glyph, got %q", i, plain)
		}
		if strings.HasSuffix(plain, " "+common.SymbolScrollbarTrack) || strings.HasSuffix(plain, " "+common.SymbolScrollbarThumb) {
			t.Fatalf("line %d should not include a gap before scrollbar, got %q", i, plain)
		}
	}
}

func TestRenderRuleList_HidesScrollbarWhenAllRulesFit(t *testing.T) {
	rules := []filteredRule{
		{
			Index: 0,
			Rule: model.Rule{
				Type:    "DOMAIN",
				Payload: "example.com",
				Proxy:   "Proxy",
			},
		},
	}

	rendered := renderRuleList(rules, 0, 0, 4, 80, 0, 0)
	if strings.Contains(rendered, common.SymbolScrollbarTrack) || strings.Contains(rendered, common.SymbolScrollbarThumb) {
		t.Fatalf("expected no scrollbar when all rules fit, got %q", rendered)
	}
}
