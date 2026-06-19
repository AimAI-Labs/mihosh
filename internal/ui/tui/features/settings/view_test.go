package settings

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
)

func TestRenderSettingsPageToastUsesReservedTopLine(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("zh-CN")

	toast := common.NewToastManager()
	toast.Success(i18n.T("settings.toast.save_success_theme"))

	rendered := stripANSISettings(RenderSettingsPage(PageState{
		Config: &config.Config{
			APIAddress: "http://127.0.0.1:9090",
			Theme:      "tokyo-night",
		},
		Toast: toast,
	}, 80, 20))

	lines := strings.Split(rendered, "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d in %q", len(lines), rendered)
	}
	if !strings.Contains(lines[0], i18n.T("settings.toast.save_success_theme")) {
		t.Fatalf("expected toast on first line, got %q", lines[0])
	}
	if !strings.Contains(lines[1], i18n.T("settings.panel_title")) {
		t.Fatalf("expected settings panel border on second line, got %q", lines[1])
	}
	if !strings.Contains(lines[2], i18n.T("settings.label.api_address")) {
		t.Fatalf("expected API address row on third line, got %q", lines[2])
	}
}

func stripANSISettings(s string) string {
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
