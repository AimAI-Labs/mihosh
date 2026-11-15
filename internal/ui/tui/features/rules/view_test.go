package rules

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
)

// stripANSI 移除 ANSI 转义序列，便于对纯文本断言。
func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inEscape := false
	for _, r := range s {
		switch {
		case r == 0x1b:
			inEscape = true
		case inEscape:
			// CSI 序列以字母（@~）结束
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func TestRenderRuleList_AppendsScrollbarWithoutGap(t *testing.T) {
	rules := make([]filteredRule, 0, 8)
	for i := 0; i < 8; i++ {
		rules = append(rules, filteredRule{
			Index: i,
			Rule: model.Rule{
				Type:    "DOMAIN",
				Payload: "example.com",
				Proxy:   "Proxy",
			},
		})
	}

	rendered := renderRuleList(rules, 0, 0, 4, 80, 0, 0)
	lines := strings.Split(rendered, "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 rendered rows, got %d", len(lines))
	}

	for i, line := range lines {
		plain := lipgloss.NewStyle().Render(line)
		if !strings.HasSuffix(plain, common.SymbolScrollbarTrack) && !strings.HasSuffix(plain, common.SymbolScrollbarThumb) {
			t.Fatalf("line %d should end with scrollbar glyph, got %q", i, plain)
		}
	}

	// 所有行宽度应一致，确保滚动条不会因行宽变化而坍缩
	for i := 1; i < len(lines); i++ {
		if lipgloss.Width(lines[i]) != lipgloss.Width(lines[0]) {
			t.Fatalf("line %d width (%d) differs from line 0 width (%d)", i, lipgloss.Width(lines[i]), lipgloss.Width(lines[0]))
		}
	}
}

func TestRenderRuleList_HidesScrollbarWhenAllRulesFit(t *testing.T) {
	rules := []filteredRule{
		{
			Index: 0,
			Rule: model.Rule{
				Type:    "DOMAIN",
				Payload: "example.com",
				Proxy:   "Proxy",
			},
		},
	}

	rendered := renderRuleList(rules, 0, 0, 4, 80, 0, 0)
	if strings.Contains(rendered, common.SymbolScrollbarTrack) || strings.Contains(rendered, common.SymbolScrollbarThumb) {
		t.Fatalf("expected no scrollbar when all rules fit, got %q", rendered)
	}
}

// TestBuildAddRuleModal_RendersProxyAsSelector 验证策略行渲染为 NAME + (Enter 改) 提示，
// 且不出现旧的 ◀ ▶ 包裹或 textinput 占位。
func TestBuildAddRuleModal_RendersProxyAsSelector(t *testing.T) {
	form := newAddForm("") // 兆底 groups=[DIRECT, REJECT]，默认 DIRECT
	state := PageState{ShowAddForm: true, AddForm: form}

	modal := buildAddRuleModal(state, 80, 24)
	plain := stripANSI(modal)

	if !strings.Contains(plain, "DIRECT") {
		t.Fatalf("expected proxy row rendered with 'DIRECT', got:\n%s", plain)
	}
	// 不应出现旧的 ◀ ▶ 包裹
	if strings.Contains(plain, "◀ DIRECT ▶") {
		t.Fatalf("stale '◀ DIRECT ▶' format should not be rendered, got:\n%s", plain)
	}
}

// TestBuildAddRuleModal_ProxyRowReflectsSelection 验证通过 picker 选中后弹窗显示新策略名。
func TestBuildAddRuleModal_ProxyRowReflectsSelection(t *testing.T) {
	form := newAddForm("") // [DIRECT, REJECT]
	form.proxySelected = "REJECT"
	state := PageState{ShowAddForm: true, AddForm: form}

	modal := buildAddRuleModal(state, 80, 24)
	plain := stripANSI(modal)

	if !strings.Contains(plain, "REJECT") {
		t.Fatalf("expected proxy row to show 'REJECT' after selection, got:\n%s", plain)
	}
}

// TestBuildAddRuleModal_TypeUsesArrowsProxyDoesNot 验证类型行仍使用 ◀ ▶ 包裹，
// 策略行不再使用 ◀ ▶ 而是直接显示名称。
func TestBuildAddRuleModal_TypeUsesArrowsProxyDoesNot(t *testing.T) {
	form := newAddForm("")
	state := PageState{ShowAddForm: true, AddForm: form}

	modal := buildAddRuleModal(state, 80, 24)
	plain := stripANSI(modal)

	// 默认类型 DOMAIN-SUFFIX 仍用 ◀ ▶
	if !strings.Contains(plain, "◀ DOMAIN-SUFFIX ▶") {
		t.Fatalf("expected type row '◀ DOMAIN-SUFFIX ▶', got:\n%s", plain)
	}
	// 策略行直接显示名称
	if !strings.Contains(plain, "DIRECT") {
		t.Fatalf("expected proxy row 'DIRECT', got:\n%s", plain)
	}
}

// TestAddAndEditRuleModalsDoNotShiftRightOnMouseScroll 验证添加和编辑规则弹窗在鼠标滚轮切换焦点时
// 所有字段行的起始列位置与弹窗宽度保持不变（不右移）。
func TestAddAndEditRuleModalsDoNotShiftRightOnMouseScroll(t *testing.T) {
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })

	fieldColumn := func(plainModal, prefix string) int {
		for _, line := range strings.Split(plainModal, "\n") {
			if idx := strings.Index(line, prefix); idx >= 0 {
				return common.DisplayWidth(line[:idx])
			}
		}
		return -1
	}

	assertStableColumns := func(t *testing.T, state State, build func(PageState, int, int) string) {
		t.Helper()
		prefixes := []string{
			i18n.T("rules.add_field_type"),
			i18n.T("rules.add_field_payload"),
			i18n.T("rules.add_field_proxy"),
			i18n.T("rules.add_field_index"),
		}
		if state.addForm.isIPType() {
			prefixes = append(prefixes, i18n.T("rules.add_field_noresolve"))
		}
		baselineModal := stripANSI(build(state.ToPageState(80, 24), 80, 24))
		baselineWidth := lipgloss.Width(baselineModal)
		wantCols := make([]int, len(prefixes))
		for i, p := range prefixes {
			wantCols[i] = fieldColumn(baselineModal, p)
			if wantCols[i] < 0 {
				t.Fatalf("missing field %q in modal:\n%s", p, baselineModal)
			}
		}

		for step := 0; step < addFieldCount*2; step++ {
			state = state.HandleMouseScroll(false)
			plain := stripANSI(build(state.ToPageState(80, 24), 80, 24))
			if gotW := lipgloss.Width(plain); gotW != baselineWidth {
				t.Fatalf("step %d: modal width changed from %d to %d", step, baselineWidth, gotW)
			}
			for i, p := range prefixes {
				if gotCol := fieldColumn(plain, p); gotCol != wantCols[i] {
					t.Fatalf("step %d: field %q shifted horizontally from col %d to %d\n%s", step, p, wantCols[i], gotCol, plain)
				}
			}
		}
	}

	for _, lang := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(lang)
		addState := State{showAddForm: true, addForm: newAddForm("")}
		assertStableColumns(t, addState, buildAddRuleModal)

		editState := State{
			showEditForm: true,
			addForm: newEditForm("", model.Rule{
				Type:      "IP-CIDR",
				Payload:   "10.0.0.0/8",
				Proxy:     "DIRECT",
				NoResolve: true,
			}, 0),
		}
		assertStableColumns(t, editState, buildEditRuleModal)
	}
}
