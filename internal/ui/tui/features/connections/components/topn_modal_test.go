package components

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/pkg/i18n"
)

func init() {
	i18n.Init()
}

func TestRenderTopNModal(t *testing.T) {
	items := []TopNItem{
		{Name: "google.com", TotalBytes: 1024 * 1024},
	}

	result := RenderTopNModal(items, 80, 24, 0)

	// 1. 验证输出中不含原先底部的操作提示信息
	if strings.Contains(result, "[↑/↓/k/j] 滚动") {
		t.Errorf("expected output to NOT contain helper text, but it did")
	}

	// 2. 验证输出包含基本标题
	if !strings.Contains(result, "吞吐量排行 (过去5分钟)") {
		t.Errorf("expected output to contain title, got %q", result)
	}
}

func TestResolveTopNModalBounds(t *testing.T) {
	items := []TopNItem{
		{Name: "google.com", TotalBytes: 1024 * 1024},
	}

	left, top, right, bottom := ResolveTopNModalBounds(items, 80, 24, 0)

	if left < 0 || top < 0 || right <= left || bottom <= top {
		t.Errorf("invalid modal bounds: left=%d, top=%d, right=%d, bottom=%d", left, top, right, bottom)
	}
}
