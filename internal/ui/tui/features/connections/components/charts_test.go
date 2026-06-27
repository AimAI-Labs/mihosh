package components

import (
	"strings"
	"testing"
)

func TestRenderSpeedChartOnlyShowsMaxYAxisLabel(t *testing.T) {
	result := RenderSpeedChart(
		[]int64{1024, 2048, 4096},
		[]int64{4096, 2048, 1024},
		FormatSpeed,
		80,
		4,
	)

	// Since there are two charts (upload and download), the max label should appear twice.
	if got := strings.Count(result, "4.0 KB/s"); got != 2 {
		t.Fatalf("expected max Y-axis label twice (once per chart), got %d in \n%s", got, result)
	}
	if strings.Contains(result, "2.0 KB/s") {
		t.Fatalf("expected half Y-axis label to be hidden, got \n%s", result)
	}
}

func TestRenderSpeedChartUsesNeutralCenterAxis(t *testing.T) {
	result := RenderSpeedChart(
		[]int64{1024, 2048, 4096},
		[]int64{4096, 2048, 1024},
		FormatSpeed,
		80,
		4,
	)

	centerLine := findChartCenterLine(t, result)
	if strings.Contains(centerLine, "█") || strings.Contains(centerLine, "▄") || strings.Contains(centerLine, "▆") {
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
	t.Fatalf("expected chart to contain a center axis, got \n%s", chart)
	return ""
}
