package components

import (
	"strings"
	"testing"
)

func TestRenderSymmetricBarChartSpreadsShortHistoryAcrossWideChart(t *testing.T) {
	upload := []int64{100, 200, 300, 400}
	download := []int64{400, 300, 200, 100}

	result := RenderSymmetricBarChart(upload, download, FormatSpeed, 80, 2)
	centerLine := findChartCenterLine(t, result)
	leftHalf := centerLine[:len(centerLine)/2]

	if !strings.Contains(leftHalf, "█") {
		t.Fatalf("expected short traffic history to be visible before the chart midpoint, got center line %q", centerLine)
	}
}

func findChartCenterLine(t *testing.T, chart string) string {
	t.Helper()

	for _, line := range strings.Split(chart, "\n") {
		if strings.Contains(line, "┼") {
			return line
		}
	}
	t.Fatalf("expected chart to contain a center axis, got %q", chart)
	return ""
}
