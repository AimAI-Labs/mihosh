package components

import (
	"strings"
	"testing"
)

func TestRenderTopNSection(t *testing.T) {
	t.Run("empty items", func(t *testing.T) {
		result := RenderTopNSection(nil, 60)
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("with items", func(t *testing.T) {
		items := []TopNItem{
			{Name: "google.com", TotalBytes: 1024 * 1024},
			{Name: "github.com", TotalBytes: 512 * 1024},
		}

		result := RenderTopNSection(items, 60)
		// 原本应该包含标题，但不带边框
		if !strings.Contains(result, "Top 5 吞吐量排行") {
			t.Errorf("expected output to contain title, got %q", result)
		}
		if !strings.Contains(result, "google.com") {
			t.Errorf("expected output to contain google.com, got %q", result)
		}
	})
}
