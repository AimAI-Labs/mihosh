package nodes

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/pkg/i18n"
)



func TestBuildTestResultModal_FailureEntry(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("zh-CN")

	source := `Get "http://127.0.0.1:9097/Proxies/hy2%E5%8F%B0%E6%B9%BE05/delay?url=http%3A%2F%2Fwww.gstatic.com%2Fgenerate_204&timeout=5000": dial tcp 127.0.0.1:9097: connectex: No connection could be made`
	state := PageState{
		Width:  86,
		Height: 20,
		TestResults: []TestResultEntry{
			{Name: "hy2台湾05", Delay: -1, Error: source},
		},
	}

	modal := buildTestResultModal(state)
	if !strings.Contains(modal, "原因:") {
		t.Fatalf("expected reason section in modal")
	}

	if !strings.Contains(modal, "dial tcp 127.0.0.1:9097") {
		t.Fatalf("expected source detail retained, got %q", modal)
	}
}

func TestBuildTestResultModal_SuccessEntry(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("zh-CN")

	state := PageState{
		Width:  86,
		Height: 20,
		TestResults: []TestResultEntry{
			{Name: "HK-01", Delay: 120, Error: ""},
		},
	}

	modal := buildTestResultModal(state)
	if !strings.Contains(modal, "HK-01") {
		t.Fatalf("expected node name in modal")
	}
	if !strings.Contains(modal, "120ms") {
		t.Fatalf("expected delay value in modal")
	}
	if !strings.Contains(modal, "延迟:") {
		t.Fatalf("expected delay prefix in modal")
	}
}

func TestBuildTestResultModal_MixedEntries(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("zh-CN")

	state := PageState{
		Width:  86,
		Height: 30,
		TestResults: []TestResultEntry{
			{Name: "HK-01", Delay: 80, Error: ""},
			{Name: "JP-01", Delay: -1, Error: "context deadline exceeded"},
			{Name: "SG-01", Delay: 200, Error: ""},
		},
	}

	modal := buildTestResultModal(state)
	if !strings.Contains(modal, "80ms") {
		t.Fatalf("expected HK-01 delay in modal")
	}
	if !strings.Contains(modal, "200ms") {
		t.Fatalf("expected SG-01 delay in modal")
	}
	if !strings.Contains(modal, "原因:") {
		t.Fatalf("expected failure reason for JP-01")
	}
	// 标题应显示总数 3
	if !strings.Contains(modal, "3") {
		t.Fatalf("expected total count 3 in modal title")
	}
}
