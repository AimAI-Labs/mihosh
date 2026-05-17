package nodes

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/pkg/i18n"
)

func TestSplitFailureEntry(t *testing.T) {
	node, raw := splitFailureEntry(`日本08aws: Get "http://127.0.0.1:9097/Proxies/%E6%97%A5%E6%9C%AC08aws/delay?url=http%3A%2F%2Fwww.gstatic.com%2Fgenerate_204&timeout=5000": EOF`)
	if node != "日本08aws" {
		t.Fatalf("expected node name parsed, got %q", node)
	}
	if !strings.Contains(raw, `http://127.0.0.1:9097/Proxies/`) {
		t.Fatalf("expected raw error preserved, got %q", raw)
	}
}

func TestSummarizeFailure_ExtractsRequestDetail(t *testing.T) {
	raw := `Get "http://127.0.0.1:9097/Proxies/hy2%E5%8F%B0%E6%B9%BE05/delay?url=http%3A%2F%2Fwww.gstatic.com%2Fgenerate_204&timeout=5000": dial tcp 127.0.0.1:9097: connectex: No connection could be made`
	summary := summarizeFailure(raw)
	if !strings.HasPrefix(summary, "dial tcp 127.0.0.1:9097") {
		t.Fatalf("expected concise detail extracted, got %q", summary)
	}
}

func TestBuildFailureModal_PreservesSourceInfo(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("zh-CN")

	source := `Get "http://127.0.0.1:9097/Proxies/hy2%E5%8F%B0%E6%B9%BE05/delay?url=http%3A%2F%2Fwww.gstatic.com%2Fgenerate_204&timeout=5000": dial tcp 127.0.0.1:9097: connectex: No connection could be made`
	state := PageState{
		Width:        86,
		Height:       20,
		TestFailures: []string{"hy2台湾05: " + source},
	}

	modal := buildFailureModal(state)
	if !strings.Contains(modal, "原因:") {
		t.Fatalf("expected reason section in modal")
	}
	if !strings.Contains(modal, "源信息:") {
		t.Fatalf("expected source section in modal")
	}
	if !strings.Contains(modal, "dial tcp 127.0.0.1:9097") {
		t.Fatalf("expected source detail retained, got %q", modal)
	}
	if strings.Contains(modal, "…") {
		t.Fatalf("expected no truncation ellipsis in modal, got %q", modal)
	}
}
