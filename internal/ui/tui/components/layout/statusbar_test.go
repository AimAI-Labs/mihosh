package layout

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/pkg/i18n"
)

func TestRenderStatusBar_TestingWithTarget(t *testing.T) {
	initStatusBarTestI18n()

	bar := RenderStatusBar(120, nil, true, "HK-01", "", nil, 0, 0)
	if !strings.Contains(bar, "正在测速: HK-01") {
		t.Fatalf("expected testing target in status bar, got: %q", bar)
	}
}

func TestRenderStatusBar_TestingWithoutTarget(t *testing.T) {
	initStatusBarTestI18n()

	bar := RenderStatusBar(120, nil, true, "", "", nil, 0, 0)
	if !strings.Contains(bar, "正在测速") {
		t.Fatalf("expected generic testing text in status bar, got: %q", bar)
	}
}

func TestRenderStatusBar_AutoRefreshNotice(t *testing.T) {
	initStatusBarTestI18n()

	bar := RenderStatusBar(120, nil, false, "", "配置已同步：检测到外部节点/模式变化", nil, 0, 0)
	if !strings.Contains(bar, "配置已同步：检测到外部节点/模式变化") {
		t.Fatalf("expected auto refresh notice in status bar, got: %q", bar)
	}
}

func initStatusBarTestI18n() {
	i18n.Init()
	i18n.SetLanguageOverride("zh-CN")
}
