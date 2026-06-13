package help

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
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
