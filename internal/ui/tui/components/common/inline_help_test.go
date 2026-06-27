package common

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// stripANSI 移除 ANSI 转义序列，便于对纯文本做断言
var stripANSI = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]")

func TestFormatInlineHintRow_JoinsWithSeparator(t *testing.T) {
	hints := []InlineHelpHint{
		{Key: "↑↓", Desc: "Select"},
		{Key: "↵", Desc: "Detail"},
	}
	row := FormatInlineHintRow(hints)
	plain := stripANSI.ReplaceAllString(row, "")

	// 按键与描述之间有空格，条目之间以 " · " 分隔
	if !strings.Contains(plain, "↑↓ Select") {
		t.Fatalf("expected '↑↓ Select' in row, got %q", plain)
	}
	if !strings.Contains(plain, "↵ Detail") {
		t.Fatalf("expected 'Enter Detail' in row, got %q", plain)
	}
	if !strings.Contains(plain, " · ") {
		t.Fatalf("expected ' · ' separator in row, got %q", plain)
	}
}

func TestFormatInlineHintRow_Empty(t *testing.T) {
	if got := FormatInlineHintRow(nil); got != "" {
		t.Fatalf("expected empty string for nil hints, got %q", got)
	}
}

func TestOverlayHelpAtBottomRight_PlacesPanelAtBottomRight(t *testing.T) {
	const width = 40
	const height = 5

	// 构造一个填满 width×height 的底层页面，每行内容为该行字符宽度标识
	var lines []string
	for i := 0; i < height; i++ {
		lines = append(lines, strings.Repeat("x", width))
	}
	base := strings.Join(lines, "\n")

	panel := "AB" // 2 字符宽的浮层

	result := OverlayHelpAtBottomRight(base, panel, width, height)
	resultLines := strings.Split(result, "\n")

	// 浮层应位于最后一行（紧贴底栏上边）
	last := resultLines[height-1]
	plain := stripANSI.ReplaceAllString(last, "")

	// 浮层 "AB"（2 字符宽）放在 startCol = width - panelW - 1 = 40-2-1 = 37 处，
	// 占据第 37、38 列；第 39 列（最后一个 x）被保留在右侧。
	// 故末行 = 37 个 x + "AB" + 1 个 x。
	wantLine := strings.Repeat("x", 37) + "ABx"
	if plain != wantLine {
		t.Fatalf("last line = %q, want %q", plain, wantLine)
	}

	// 其余行应保持原样
	for i := 0; i < height-1; i++ {
		plain := stripANSI.ReplaceAllString(resultLines[i], "")
		if plain != strings.Repeat("x", width) {
			t.Fatalf("line %d = %q, want %q", i, plain, strings.Repeat("x", width))
		}
	}
}

func TestOverlayHelpAtBottomRight_PreservesLeftContent(t *testing.T) {
	const width = 20
	const height = 3

	// 底层最后一行有独特内容，验证浮层叠加后左侧内容不丢失
	base := "header line\n" +
		strings.Repeat("y", width) + "\n" +
		"LEFT-MIDDLE-RIGHT"
	panel := "PANEL"

	result := OverlayHelpAtBottomRight(base, panel, width, height)
	resultLines := strings.Split(result, "\n")

	last := resultLines[height-1]
	plain := stripANSI.ReplaceAllString(last, "")

	// 左侧 "LEFT-MIDDLE-RIGHT" 的前缀应保留（startCol = 20-5-1 = 14）
	if !strings.HasPrefix(plain, "LEFT-MIDDLE-") {
		t.Fatalf("expected left content preserved, got %q", plain)
	}
	// 右侧应包含浮层文本
	if !strings.Contains(plain, "PANEL") {
		t.Fatalf("expected panel text in last line, got %q", plain)
	}
}

func TestOverlayHelpAtBottomRight_PanelWiderThanWidth(t *testing.T) {
	// 浮层宽度超过页面宽度时，startCol 兜底为 0，不应 panic
	base := strings.Repeat("z", 10)
	panel := strings.Repeat("P", 50)

	result := OverlayHelpAtBottomRight(base, panel, 10, 1)
	if result == "" {
		t.Fatal("expected non-empty result")
	}
}

func TestOverlayHelpAtBottomRight_NoOverlapChangesHeight(t *testing.T) {
	// 叠加浮层不应改变底层行数
	const width = 30
	const height = 10
	base := strings.Repeat("line\n", height)
	base = strings.TrimSuffix(base, "\n")

	result := OverlayHelpAtBottomRight(base, "hint", width, height)
	if got := lipgloss.Height(result); got != height {
		t.Fatalf("result height = %d, want %d", got, height)
	}
}
