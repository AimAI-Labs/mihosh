package help

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestRenderHelpPage_LatencyColors(t *testing.T) {
	// 初始化国际化，模拟真实运行环境
	i18n.Init()

	// 强制设置颜色渲染 profile 确保非 TTY 下单元测试也会生成色彩 ANSI 序列
	lipgloss.SetColorProfile(termenv.TrueColor)

	// 触发渲染
	output := RenderHelpPage(100, 30)

	// 预期生成的三个带颜色的圆点
	expectedGreenDot := lipgloss.NewStyle().Foreground(lipgloss.Color("#9ece6a")).Render("●")
	expectedYellowDot := lipgloss.NewStyle().Foreground(lipgloss.Color("#e0af68")).Render("●")
	expectedRedDot := lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e")).Render("●")

	if !strings.Contains(output, expectedGreenDot) {
		t.Errorf("expected output to contain green dot %q", expectedGreenDot)
	}
	if !strings.Contains(output, expectedYellowDot) {
		t.Errorf("expected output to contain yellow dot %q", expectedYellowDot)
	}
	if !strings.Contains(output, expectedRedDot) {
		t.Errorf("expected output to contain red dot %q", expectedRedDot)
	}
}

func TestVersionHelpContainsMovedDetailsAndCanScrollToEveryNote(t *testing.T) {
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	for _, language := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(language)
		ctx := HelpContext{CurrentPage: PageCore, CoreReleases: true, CorePlatform: "linux/amd64", CoreAsset: "mihomo-linux-amd64-compatible-v1.20.10.gz"}
		sections := buildHelpSections(ctx)
		var allNotes string
		for _, section := range sections {
			allNotes += strings.Join(section.notes, "\n")
		}
		for _, want := range []string{ctx.CorePlatform, ctx.CoreAsset, i18n.T("core.catalog.download_hint"), i18n.T("core.desc.activate"), i18n.T("core.catalog.more_hint"), i18n.T("core.catalog.fallback")} {
			if !strings.Contains(allNotes, want) {
				t.Fatalf("help lost moved information: %q", want)
			}
		}
		for _, width := range []int{24, 80, 120} {
			ctx.ScrollOffset = 0
			_, limit := renderScrollablePopup(width, 24, ctx)
			if limit == 0 {
				t.Fatal("fixture should require scrolling")
			}
			var state State
			var seen string
			for offset := 0; offset <= limit; offset++ {
				ctx.ScrollOffset = state.Offset()
				view := ansi.Strip(renderPopup(width, 24, ctx))
				if lipgloss.Height(view) > 24 {
					t.Fatal("help exceeds terminal height")
				}
				for _, line := range strings.Split(view, "\n") {
					if lipgloss.Width(line) > width {
						t.Fatal("help exceeds terminal width")
					}
				}
				seen += view
				state = state.Scroll(1, width, 24, ctx)
			}
			if !strings.Contains(seen, "mihomo") || !strings.Contains(seen, "linux/") {
				t.Fatal("selected asset details cannot be reached by scrolling")
			}
			if state.Offset() != limit || state.Scroll(-limit-10, width, 24, ctx).Offset() != 0 {
				t.Fatal("help scrolling is not bounded")
			}
		}
	}
}
