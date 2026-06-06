package components

import (
	"strings"
	"testing"
)

func TestRenderSymmetricBarChartSpreadsShortHistoryAcrossWideChart(t *testing.T) {
	upload := []int64{100, 200, 300, 400}
	download := []int64{400, 300, 200, 100}

	result := RenderSymmetricBarChart(upload, download, FormatSpeed, 80, 2)

	if !chartHasBarsBeforeMidpoint(result) {
		t.Fatalf("expected short traffic history to be visible before the chart midpoint, got chart %q", result)
	}
}

func TestRenderSymmetricBarChartOnlyShowsMaxYAxisLabel(t *testing.T) {
	result := RenderSymmetricBarChart(
		[]int64{1024, 2048, 4096},
		[]int64{4096, 2048, 1024},
		FormatSpeed,
		80,
		4,
	)

	if got := strings.Count(result, "4.0 KB/s"); got != 1 {
		t.Fatalf("expected max Y-axis label once, got %d in %q", got, result)
	}
	if strings.Contains(result, "2.0 KB/s") {
		t.Fatalf("expected half Y-axis label to be hidden, got %q", result)
	}
}

func TestRenderSymmetricBarChartUsesNeutralCenterAxis(t *testing.T) {
	result := RenderSymmetricBarChart(
		[]int64{1024, 2048, 4096},
		[]int64{4096, 2048, 1024},
		FormatSpeed,
		80,
		4,
	)

	centerLine := findChartCenterLine(t, result)
	if strings.Contains(centerLine, "█") {
		t.Fatalf("expected center axis to stay neutral without data bars, got %q", centerLine)
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

func chartHasBarsBeforeMidpoint(chart string) bool {
	for _, line := range strings.Split(chart, "\n") {
		if strings.Contains(line, "┼") {
			continue
		}
		leftHalf := line[:len(line)/2]
		if strings.Contains(leftHalf, "█") {
			return true
		}
	}
	return false
}
