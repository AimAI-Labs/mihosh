package i18n

import (
	"testing"
)

func TestGuideTranslationsExist(t *testing.T) {
	Init()
	SetLanguageOverride("zh-CN")
	if got := T("guide.title"); got == "guide.title" {
		t.Errorf("expected zh-CN guide.title translation, got fallback key")
	}
	SetLanguageOverride("en-US")
	if got := T("guide.title"); got == "guide.title" {
		t.Errorf("expected en-US guide.title translation, got fallback key")
	}
}
