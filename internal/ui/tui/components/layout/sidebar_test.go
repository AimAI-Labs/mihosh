package layout

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/AimAI-Labs/mihosh/pkg/i18n"
)

func TestSidebarWidthUsesLongestEnglishLabel(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")

	longest := 0
	for _, key := range []string{"menu.nodes", "menu.connections", "menu.logs", "menu.rules", "menu.settings"} {
		if width := utf8.RuneCountInString(i18n.T(key)); width > longest {
			longest = width
		}
	}

	if got := SidebarWidth(); got < longest {
		t.Fatalf("expected sidebar width >= longest english label width %d, got %d", longest, got)
	}
}

func TestRenderSidebar_DoesNotWrapEnglishSettingsLabel(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")

	sidebar := RenderSidebar(PageConnections, 20)

	if strings.Contains(sidebar, "Settin\ngs") {
		t.Fatalf("expected Settings label to stay on one line, got %q", sidebar)
	}
	if !strings.Contains(sidebar, "Settings") {
		t.Fatalf("expected Settings label in sidebar, got %q", sidebar)
	}
}
