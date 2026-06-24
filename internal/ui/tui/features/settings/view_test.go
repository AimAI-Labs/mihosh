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
			TestURL: "http://example.com",
			Theme:   "tokyo-night",
		},
		Toast: toast,
	}, 80, 20))

	lines := strings.Split(rendered, "\n")
	if len(MihoshSettingKeys) != 5 {
		t.Fatalf("expected 5 setting keys, got %d", len(MihoshSettingKeys))
	}
	if MihoshSettingKeys[3] != "auto-refresh-interval" {
		t.Fatalf("expected auto-refresh-interval setting key, got %q", MihoshSettingKeys[3])
	}
	if MihoshSettingKeys[4] != "theme" {
		t.Fatalf("expected theme setting key, got %q", MihoshSettingKeys[4])
	}
	if !strings.Contains(lines[4], i18n.T("settings.panel_title")) {
		t.Fatalf("expected settings panel border on fifth line, got %q", lines[4])
	}
	if !strings.Contains(lines[5], i18n.T("settings.label.test_url")) {
		t.Fatalf("expected test-url row on sixth line, got %q", lines[5])
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

// TestRenderSettingsPage_MihomoTabShowsLoading 验证切换到 Mihomo 标签页但未加载时，显示加载提示而非配置项。
func TestRenderSettingsPage_MihomoTabShowsLoading(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("zh-CN")

	rendered := stripANSISettings(RenderSettingsPage(PageState{
		Config:       &config.Config{},
		ActiveTab:    1, // Mihomo 标签页
		MihomoLoaded: false,
	}, 80, 20))

	if !strings.Contains(rendered, i18n.T("settings.mihomo.loading")) {
		t.Fatalf("expected loading hint when Mihomo not loaded, got %q", rendered)
	}
	// 不应渲染 mixed-port 等配置项（未加载）
	if strings.Contains(rendered, i18n.T("settings.label.mixed-port")) {
		t.Fatalf("expected no mihomo setting rows while loading, got %q", rendered)
	}
}

// TestRenderSettingsPage_MihomoTabShowsConfig 验证加载完成后 Mihomo 标签页显示配置项。
func TestRenderSettingsPage_MihomoTabShowsConfig(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("zh-CN")

	rendered := stripANSISettings(RenderSettingsPage(PageState{
		Config:       &config.Config{},
		ActiveTab:    1, // Mihomo 标签页
		MihomoLoaded: true,
		MihomoConfig: nil, // 已加载但无数据：仍展示列表（值为空）
	}, 80, 24))

	if strings.Contains(rendered, i18n.T("settings.mihomo.loading")) {
		t.Fatalf("expected no loading hint when Mihomo loaded, got %q", rendered)
	}
	// 已加载时应渲染 external-controller 等配置项（标签较长会在窄宽下换行，故匹配稳定子串）
	if !strings.Contains(rendered, "Controller") {
		t.Fatalf("expected external-controller row when loaded, got %q", rendered)
	}
}

// TestRenderSettingsPage_TabBarHasBorder 验证标签栏渲染了圆角边框。
func TestRenderSettingsPage_TabBarHasBorder(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("zh-CN")

	rendered := stripANSISettings(RenderSettingsPage(PageState{
		Config: &config.Config{},
	}, 80, 20))
	lines := strings.Split(rendered, "\n")

	if !strings.Contains(lines[1], "╭") || !strings.Contains(lines[1], "╮") {
		t.Fatalf("expected tab bar top border on line 1, got %q", lines[1])
	}
	if !strings.Contains(lines[2], i18n.T("settings.tab.mihosh")) || !strings.Contains(lines[2], i18n.T("settings.tab.mihomo")) {
		t.Fatalf("expected tab labels on line 2, got %q", lines[2])
	}
	if !strings.Contains(lines[3], "╰") || !strings.Contains(lines[3], "╯") {
		t.Fatalf("expected tab bar bottom border on line 3, got %q", lines[3])
	}
}
