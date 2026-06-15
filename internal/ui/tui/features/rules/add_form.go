package rules

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// 预定义规则类型（与 Mihomo 一致）。MATCH 为特殊项，无 payload。
var ruleTypePresets = []string{
	"DOMAIN",
	"DOMAIN-SUFFIX",
	"DOMAIN-KEYWORD",
	"IP-CIDR",
	"IP-CIDR6",
	"GEOIP",
	"GEOSITE",
	"PROCESS-NAME",
	"PROCESS-PATH",
	"SRC-IP-CIDR",
	"DST-PORT",
	"SRC-PORT",
	"MATCH",
}

// 表单字段索引（同时也是 Tab 循环顺序）。
const (
	addFieldPayload = 0 // 匹配值
	addFieldProxy   = 1 // 策略
	addFieldIndex   = 2 // 插入位置
	addFieldCount   = 3 // 字段数
)

// addForm 维护「添加自定义规则」弹窗的表单状态。
//
// textinput.Model 为值类型，State 在 Bubble Tea 中按值传递，每次更新都会
// 复制整个 form；因此本结构的方法均以值接收者返回新值。
type addForm struct {
	fields      []textinput.Model // length == addFieldCount
	fieldCursor int               // 当前聚焦字段
	typeCursor  int               // 类型选择器光标（ruleTypePresets 索引）
	errMsg      string            // 行内校验/写入错误
}

// newAddForm 构造初始表单：默认类型 DOMAIN-SUFFIX，位置字段留空（默认顶部）。
func newAddForm() addForm {
	fields := make([]textinput.Model, addFieldCount)

	payload := textinput.New()
	payload.Placeholder = "example.com"
	payload.CharLimit = 256
	payload.Prompt = ""
	fields[addFieldPayload] = payload

	proxy := textinput.New()
	proxy.Placeholder = "DIRECT"
	proxy.CharLimit = 64
	proxy.Prompt = ""
	fields[addFieldProxy] = proxy

	idx := textinput.New()
	idx.Placeholder = "1"
	idx.CharLimit = 6
	idx.Prompt = ""
	fields[addFieldIndex] = idx

	form := addForm{fields: fields, fieldCursor: addFieldPayload}
	// 默认类型选择 DOMAIN-SUFFIX
	form.typeCursor = indexOfPreset("DOMAIN-SUFFIX")
	form.focusCurrent()
	return form
}

// indexOfPreset 在 ruleTypePresets 中查找（不区分大小写）；未命中返回 0。
func indexOfPreset(name string) int {
	for i, t := range ruleTypePresets {
		if strings.EqualFold(t, name) {
			return i
		}
	}
	return 0
}

// currentType 返回当前选中的规则类型字符串。
func (f addForm) currentType() string {
	if f.typeCursor < 0 || f.typeCursor >= len(ruleTypePresets) {
		return ruleTypePresets[0]
	}
	return ruleTypePresets[f.typeCursor]
}

// isMatchType 当前类型是否为 MATCH（无 payload）。
func (f addForm) isMatchType() bool {
	return strings.EqualFold(f.currentType(), "MATCH")
}

// focusCurrent 聚焦当前字段、其他字段失焦。
func (f *addForm) focusCurrent() {
	for i := range f.fields {
		if i == f.fieldCursor {
			f.fields[i].Focus()
		} else {
			f.fields[i].Blur()
		}
	}
}

// cycleField 切换聚焦字段（dir=1 向下，dir=-1 向上），循环。
func (f *addForm) cycleField(dir int) {
	n := addFieldCount
	f.fieldCursor = ((f.fieldCursor+dir)%n + n) % n
	f.focusCurrent()
}

// cycleType 循环切换类型（dir=1 下一项，dir=-1 上一项）。
func (f *addForm) cycleType(dir int) {
	n := len(ruleTypePresets)
	f.typeCursor = ((f.typeCursor+dir)%n + n) % n
}

// resolveIndex 解析位置字段：空或非法视为 1（顶部，最高优先级）。
func (f addForm) resolveIndex() int {
	raw := strings.TrimSpace(f.fields[addFieldIndex].Value())
	if raw == "" {
		return 1
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// validate 校验表单输入。返回 (ok, errorKey)，errorKey 为 i18n 键。
//
// 校验规则：
//   - 非 MATCH 类型 payload 必填；
//   - proxy 必填；
//   - index 字段为空或可被 strconv.Atoi 解析为 >=1 的整数。
func (f addForm) validate() (bool, string) {
	if !f.isMatchType() {
		if strings.TrimSpace(f.fields[addFieldPayload].Value()) == "" {
			return false, "rules.add_err_payload"
		}
	}
	if strings.TrimSpace(f.fields[addFieldProxy].Value()) == "" {
		return false, "rules.add_err_proxy"
	}
	if idxRaw := strings.TrimSpace(f.fields[addFieldIndex].Value()); idxRaw != "" {
		if n, err := strconv.Atoi(idxRaw); err != nil || n < 1 {
			return false, "rules.add_err_index"
		}
	}
	return true, ""
}

// isTextInputKey 判断按键是否应透传给 textinput（编辑类按键或可打印字符）。
func isTextInputKey(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyBackspace, tea.KeyDelete, tea.KeyCtrlA, tea.KeyCtrlE,
		tea.KeyCtrlW, tea.KeyCtrlU:
		return true
	case tea.KeyRunes:
		// 接受任意数量的可打印 rune（粘贴/快速输入可能一次产生多个）
		for _, r := range msg.Runes {
			if r < 32 {
				return false
			}
		}
		return len(msg.Runes) > 0
	}
	return false
}
