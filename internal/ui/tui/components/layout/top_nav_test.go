package layout

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/ui/styles"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
)

func TestTopNavWidthUsesAvailableWidth(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")

	width := 72
	nav := RenderTopNav(PageConnections, width)

	for _, line := range strings.Split(nav, "\n") {
		if got := lipgloss.Width(line); got != width {
			t.Fatalf("expected top nav line width %d, got %d for %q", width, got, line)
		}
	}
}

func TestRenderTopNav_DoesNotWrapEnglishSettingsLabel(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")

	nav := RenderTopNav(PageConnections, 72)

	if strings.Contains(nav, "Settin\ngs") {
		t.Fatalf("expected Settings label to stay on one line, got %q", nav)
	}
	if !strings.Contains(nav, "Settings") {
		t.Fatalf("expected Settings label in top nav, got %q", nav)
	}
}

func TestRenderTopNavIsTitlelessBox(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")

	nav := RenderTopNav(PageNodes, 72)

	if strings.Contains(nav, "Mihosh") || strings.Contains(nav, "Menu") {
		t.Fatalf("expected top nav box to be titleless, got %q", nav)
	}
	if !strings.HasPrefix(nav, "╭") || !strings.Contains(nav, "\n│") || !strings.HasSuffix(nav, "╯") {
		t.Fatalf("expected top nav to use node-style box borders, got %q", nav)
	}
}

func TestRenderTopNavUsesNeutralBorderWhenIdle(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")

	style := topNavBorderStyle()

	if got := style.GetForeground(); got != styles.ColorBorder {
		t.Fatalf("expected idle top nav border to use neutral border color, got %q", got)
	}
}

func TestRenderTopNavKeepsNeutralBorderWhenActiveStatusIsProvided(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")

	style := topNavBorderStyle()

	if got := style.GetForeground(); got != styles.ColorBorder {
		t.Fatalf("expected active top nav border to remain neutral, got %q", got)
	}
}

func TestRenderTopNavShowsRefreshCountdownSeconds(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")

	nav := RenderTopNav(PageNodes, 72, TopNavRefreshStatus{Enabled: true, SecondsRemaining: 4})

	if !strings.Contains(nav, "4s") {
		t.Fatalf("expected top nav to show countdown seconds, got %q", nav)
	}
}

func TestRenderTopNavKeepsRefreshCountdownWhenWidthIsTight(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")

	nav := RenderTopNav(PageNodes, 34, TopNavRefreshStatus{Enabled: true, SecondsRemaining: 120})

	if !strings.Contains(nav, "120s") {
		t.Fatalf("expected top nav to keep countdown when width is tight, got %q", nav)
	}
}

func TestRenderTopNavKeepsRefreshCountdownWithinNarrowWidth(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("zh-CN")

	width := 24
	nav := RenderTopNav(PageNodes, width, TopNavRefreshStatus{Enabled: true, SecondsRemaining: 123456789012345678})

	if !strings.Contains(nav, "123456789012345678s") {
		t.Fatalf("expected top nav to keep countdown seconds, got %q", nav)
	}
	for _, line := range strings.Split(nav, "\n") {
		if got := lipgloss.Width(line); got != width {
			t.Fatalf("expected top nav line width %d, got %d for %q", width, got, line)
		}
	}
}

func TestRenderTopNavShowsSyncedCheckmark(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")

	nav := RenderTopNav(PageNodes, 72, TopNavRefreshStatus{Enabled: true, Synced: true})

	if !strings.Contains(nav, "✔") {
		t.Fatalf("expected top nav to show synced checkmark, got %q", nav)
	}
}

func TestGetClickedTopNavPage(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")

	nav := RenderTopNav(PageNodes, 72)
	lines := strings.Split(stripANSI(nav), "\n")
	if len(lines) != TopNavHeight {
		t.Fatalf("expected top nav height %d, got %d in %q", TopNavHeight, len(lines), nav)
	}
	connectionsLabel := i18n.T("menu.connections")
	connectionsX := strings.Index(lines[1], connectionsLabel)
	if connectionsX < 0 {
		t.Fatalf("expected %s label in top nav, got %q", connectionsLabel, nav)
	}

	if got := GetClickedTopNavPage(connectionsX, 1, 72); got != PageConnections {
		t.Fatalf("expected click on %s to select PageConnections, got %v", connectionsLabel, got)
	}
	if got := GetClickedTopNavPage(connectionsX, 0, 72); got != -1 {
		t.Fatalf("expected border click to be ignored, got %v", got)
	}
}

func stripANSI(s string) string {
	var out strings.Builder
	inEscape := false
	for _, r := range s {
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		if r == '\x1b' {
			inEscape = true
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}
