package layout

import (
	"fmt"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/pkg/i18n"
)

func TestRenderStatusBar_TestingWithTarget(t *testing.T) {
	initStatusBarTestI18n()

	bar := RenderStatusBar(120, nil, true, "HK-01", "", nil, 0, 0, "", "", "", 0)
	if !strings.Contains(bar, "正在测速: HK-01") {
		t.Fatalf("expected testing target in status bar, got: %q", bar)
	}
}

func TestRenderStatusBar_TestingWithoutTarget(t *testing.T) {
	initStatusBarTestI18n()

	bar := RenderStatusBar(120, nil, true, "", "", nil, 0, 0, "", "", "", 0)
	if !strings.Contains(bar, "正在测速") {
		t.Fatalf("expected generic testing text in status bar, got: %q", bar)
	}
}

func TestRenderStatusBar_AutoRefreshNotice(t *testing.T) {
	initStatusBarTestI18n()

	bar := RenderStatusBar(120, nil, false, "", "配置已同步：检测到外部节点/模式变化", nil, 0, 0, "", "", "", 0)
	if !strings.Contains(bar, "配置已同步：检测到外部节点/模式变化") {
		t.Fatalf("expected auto refresh notice in status bar, got: %q", bar)
	}
}

func TestRenderNodeInfo_Normal(t *testing.T) {
	result := renderNodeInfo("Rule", "GLOBAL", "HK-01", 120, 120)
	if !strings.Contains(result, "Rule") {
		t.Errorf("expected 'Rule' in node info, got: %q", result)
	}
	if !strings.Contains(result, "GLOBAL") {
		t.Errorf("expected 'GLOBAL' in node info, got: %q", result)
	}
	if !strings.Contains(result, "HK-01") {
		t.Errorf("expected 'HK-01' in node info, got: %q", result)
	}
	if !strings.Contains(result, "120ms") {
		t.Errorf("expected '120ms' in node info, got: %q", result)
	}
	if !strings.Contains(result, "●") {
		t.Errorf("expected '●' marker in node info, got: %q", result)
	}
}

func TestRenderNodeInfo_NoData(t *testing.T) {
	result := renderNodeInfo("", "", "", 0, 120)
	if !strings.Contains(result, "--") {
		t.Errorf("expected '--' placeholder when no data, got: %q", result)
	}
}

func TestRenderNodeInfo_NoDelay(t *testing.T) {
	result := renderNodeInfo("Rule", "GLOBAL", "HK-01", 0, 120)
	if !strings.Contains(result, "--") {
		t.Errorf("expected '--' for no delay, got: %q", result)
	}
}

func TestRenderNodeInfo_LongNodeName(t *testing.T) {
	longName := "这是一个非常非常长的节点名称超过二十个字符"
	result := renderNodeInfo("Rule", "GLOBAL", longName, 100, 120)
	if strings.Contains(result, longName) {
		t.Errorf("expected long name to be truncated, got: %q", result)
	}
}

func TestRenderNodeInfo_NarrowWidth(t *testing.T) {
	result := renderNodeInfo("Rule", "GLOBAL", "HK-01", 120, 50)
	if strings.Contains(result, "GLOBAL") {
		t.Errorf("expected group name hidden on narrow width, got: %q", result)
	}
}

func TestRenderStatusBar_WithNodeInfo(t *testing.T) {
	initStatusBarTestI18n()

	bar := RenderStatusBar(120, nil, false, "", "", nil, 0, 0, "Rule", "GLOBAL", "HK-01", 120)
	if !strings.Contains(bar, "Rule") {
		t.Errorf("expected 'Rule' in status bar, got: %q", bar)
	}
	if !strings.Contains(bar, "HK-01") {
		t.Errorf("expected 'HK-01' in status bar, got: %q", bar)
	}
	if !strings.Contains(bar, "120ms") {
		t.Errorf("expected '120ms' in status bar, got: %q", bar)
	}
}

func TestRenderStatusBar_NodeInfoWithError(t *testing.T) {
	initStatusBarTestI18n()

	bar := RenderStatusBar(120, fmt.Errorf("connection refused"), false, "", "", nil, 0, 0, "Rule", "GLOBAL", "HK-01", 120)
	if !strings.Contains(bar, "HK-01") {
		t.Errorf("expected node info even with error, got: %q", bar)
	}
	// "connection refused" 会被翻译为中文友好提示
	if !strings.Contains(bar, "✗") {
		t.Errorf("expected error indicator '✗', got: %q", bar)
	}
}

func initStatusBarTestI18n() {
	i18n.Init()
	i18n.SetLanguageOverride("zh-CN")
}
