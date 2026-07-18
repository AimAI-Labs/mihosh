package nodes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	tea "github.com/charmbracelet/bubbletea"
)

// TestNodesState_TestAllDoesNotPanic 模拟真实 TUI 中按 a 全测的完整流程
func TestNodesState_TestAllDoesNotPanic(t *testing.T) {
	// 模拟 mihomo API server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, _ := w.(http.Hijacker)
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	defer ts.Close()

	endpoint := config.MihomoEndpoint{
		ExternalController: ts.URL,
		Secret:             "",
	}
	client := api.NewClient(endpoint, 2000)
	proxySvc := service.NewProxyService(client, "http://www.gstatic.com/generate_204", 2000)

	// 使用 NewState 初始化（和真实 TUI 一致）
	state := NewState(client, proxySvc, "http://www.gstatic.com/generate_204", 2000)

	// 模拟已经加载了节点数据
	state.GroupNames = []string{"Auto", "Proxy"}
	state.CurrentProxies = []string{"HK-01", "JP-01", "SG-01", "US-01", "KR-01"}
	state.OriginalProxies = append([]string(nil), state.CurrentProxies...)
	state.SelectedGroup = 0

	// 按 a 触发全测
	aKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	nextState, cmd := state.Update(aKey)

	if !nextState.Testing {
		t.Fatalf("expected Testing=true after pressing a")
	}
	if !nextState.TestAllActive {
		t.Fatalf("expected TestAllActive=true")
	}
	if nextState.TestAllTotal != 5 {
		t.Fatalf("expected TestAllTotal=5, got %d", nextState.TestAllTotal)
	}

	// 验证 cmd 不为 nil（应该产生了批量测速命令）
	if cmd == nil {
		t.Fatalf("expected non-nil cmd from TestAll")
	}

	// 实际执行所有 cmd 函数，验证没有 panic
	// tea.Batch 返回的 Cmd 会返回一个 BatchMsg
	msg := cmd()
	if msg == nil {
		t.Fatalf("cmd returned nil msg")
	}

	// BatchMsg 包含多个子 cmd
	batchMsg, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Logf("msg type: %T", msg)
		// 如果不是 BatchMsg，可能是单个 cmd 的结果
		return
	}

	// 执行每个子 cmd
	for i, subCmd := range batchMsg {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("sub cmd %d panicked: %v", i, r)
				}
			}()
			if subCmd != nil {
				subMsg := subCmd()
				t.Logf("sub cmd %d returned: %T", i, subMsg)
			}
		}()
	}

	// 模拟 TestDoneMsg 回来后的处理
	for _, name := range []string{"HK-01", "JP-01", "SG-01", "US-01", "KR-01"} {
		nextState = nextState.ApplyTestDone(name, 42, nil, "http://www.gstatic.com/generate_204")
	}

	if nextState.Testing {
		t.Fatalf("expected Testing=false after all tests done")
	}
	if nextState.TestAllActive {
		t.Fatalf("expected TestAllActive=false after all tests done")
	}

	// 验证测速结果
	results := nextState.TestResults()
	if len(results) != 5 {
		t.Fatalf("expected 5 test results, got %d", len(results))
	}

	// 验证渲染不 panic
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("RenderNodesPage panicked: %v", r)
			}
		}()
		pageState := nextState.ToPageState(120, 40)
		_ = RenderNodesPage(pageState)
	}()
}
