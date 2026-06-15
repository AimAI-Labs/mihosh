package rules

import (
	"strconv"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
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

// defaultProxyPolicy 是策略提取失败或列表为空时的兜底值。
const defaultProxyPolicy = "DIRECT"

// addPickerTab 是策略选择二级弹窗的分类索引。
const (
	addPickerTabGroup = 0 // 策略组（含内置 DIRECT/REJECT）
	addPickerTabNode  = 1 // 具体节点
	addPickerTabCount = 2
)

// addPickerListMax 单屏最多展示的列表项数，超出则滚动。
const addPickerListMax = 14

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
//
// 策略(proxy)字段为只读回填：选中值存于 proxySelected，点击策略行聚焦后按
// Enter 可打开「选择策略」二级弹窗（picker 子状态）从中选定并回填。
// addFieldProxy 槽位仍保留一个 textinput.Model 仅作占位以维持字段索引与
// 布局高度一致，其值不参与提交。
type addForm struct {
	fields      []textinput.Model // length == addFieldCount；proxy 槽为占位
	fieldCursor int               // 当前聚焦字段
	typeCursor  int               // 类型选择器光标（ruleTypePresets 索引）

	// 策略：分类候选 + 当前回填选中名
	proxyGroups   []string // [DIRECT, REJECT, ...proxy-groups]
	proxyNodes    []string // [...proxies]
	proxySelected string   // 当前回填的选中策略名（始终非空：至少 DIRECT）

	// 策略选择二级弹窗子状态（仅 showProxyPicker 为 true 时有意义）
	showProxyPicker bool
	pickerTab       int    // 0=策略组, 1=具体节点
	pickerSearch    string // 模糊搜索文本
	pickerCursor    int    // 在「过滤后」列表中的索引
	pickerScrollTop int

	errMsg string // 行内校验/写入错误
}

// newAddForm 构造初始表单：默认类型 DOMAIN-SUFFIX，位置字段留空（默认顶部）。
// 策略分类候选由 configPath 指向的源配置文件提取（proxy-groups / proxies 名称 +
// 内置 DIRECT/REJECT）。proxySelected 默认 DIRECT。
func newAddForm(configPath string) addForm {
	fields := make([]textinput.Model, addFieldCount)

	payload := textinput.New()
	payload.Placeholder = "example.com"
	payload.CharLimit = 256
	payload.Prompt = ""
	fields[addFieldPayload] = payload

	// proxy 槽位保留 textinput.Model 仅作占位，实际值由二级弹窗回填。
	proxy := textinput.New()
	proxy.Prompt = ""
	fields[addFieldProxy] = proxy

	idx := textinput.New()
	idx.Placeholder = "1"
	idx.CharLimit = 6
	idx.Prompt = ""
	fields[addFieldIndex] = idx

	groups, nodes, _ := config.ExtractProxyPolicies(configPath)
	if len(groups) == 0 {
		groups = []string{defaultProxyPolicy, "REJECT"}
	}

	form := addForm{
		fields:        fields,
		fieldCursor:   addFieldPayload,
		proxyGroups:   groups,
		proxyNodes:    nodes,
		proxySelected: defaultProxyPolicy,
	}
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

// currentProxy 返回当前选中的策略字符串（始终非空：至少含 DIRECT 兜底）。
func (f addForm) currentProxy() string {
	if f.proxySelected == "" {
		return defaultProxyPolicy
	}
	return f.proxySelected
}

// isProxyPickerOpen 策略选择二级弹窗是否打开。
func (f addForm) isProxyPickerOpen() bool { return f.showProxyPicker }

// pickerCandidates 返回当前 Tab 的全量候选列表。
func (f addForm) pickerCandidates() []string {
	if f.pickerTab == addPickerTabNode {
		return f.proxyNodes
	}
	return f.proxyGroups
}

// pickerFiltered 对当前 Tab 候选按 pickerSearch 做模糊匹配过滤。
// 空搜索返回全量。
func (f addForm) pickerFiltered() []string {
	candidates := f.pickerCandidates()
	if f.pickerSearch == "" {
		return candidates
	}
	var result []string
	for _, c := range candidates {
		if fuzzyMatch(f.pickerSearch, c) {
			result = append(result, c)
		}
	}
	return result
}

// pickerChoose 选中指定策略，关闭弹窗并清空搜索/光标。
func (f *addForm) pickerChoose(name string) {
	f.proxySelected = name
	f.showProxyPicker = false
	f.pickerSearch = ""
	f.pickerCursor = 0
	f.pickerScrollTop = 0
}

// pickerClose 仅关闭弹窗（保留已选策略），不清空 proxySelected。
func (f *addForm) pickerClose() {
	f.showProxyPicker = false
	f.pickerSearch = ""
	f.pickerCursor = 0
	f.pickerScrollTop = 0
}

// cyclePickerTab 切换策略弹窗 Tab（dir=1 下一个，dir=-1 上一个）。
func (f *addForm) cyclePickerTab(dir int) {
	f.pickerTab = ((f.pickerTab+dir)%addPickerTabCount + addPickerTabCount) % addPickerTabCount
	f.pickerCursor = 0
	f.pickerScrollTop = 0
}

// movePickerCursor 在过滤后列表中移动光标（dir=1 向下，dir=-1 向上），含滚动修正。
func (f *addForm) movePickerCursor(dir int) {
	filtered := f.pickerFiltered()
	n := len(filtered)
	if n == 0 {
		return
	}
	f.pickerCursor = ((f.pickerCursor+dir)%n + n) % n
	// 滚动修正
	if f.pickerCursor < f.pickerScrollTop {
		f.pickerScrollTop = f.pickerCursor
	}
	if f.pickerCursor >= f.pickerScrollTop+addPickerListMax {
		f.pickerScrollTop = f.pickerCursor - addPickerListMax + 1
	}
}

// pickerTypeChar 向搜索框追加一个字符。
func (f *addForm) pickerTypeChar(r rune) {
	f.pickerSearch += string(r)
	f.pickerCursor = 0
	f.pickerScrollTop = 0
}

// pickerBackspace 删除搜索框最后一个字符。
func (f *addForm) pickerBackspace() {
	runes := []rune(f.pickerSearch)
	if len(runes) > 0 {
		f.pickerSearch = string(runes[:len(runes)-1])
		f.pickerCursor = 0
		f.pickerScrollTop = 0
	}
}

// openProxyPicker 打开策略选择二级弹窗，重置搜索/光标。
func (f *addForm) openProxyPicker() {
	f.showProxyPicker = true
	f.pickerTab = addPickerTabGroup
	f.pickerSearch = ""
	f.pickerCursor = 0
	f.pickerScrollTop = 0
}

// isProxyGroupSelected 当前选中策略是否来自策略组（含内置 DIRECT/REJECT）。
func (f addForm) isProxyGroupSelected() bool {
	for _, g := range f.proxyGroups {
		if g == f.proxySelected {
			return true
		}
	}
	return false
}

// isProxyField 当前聚焦字段是否为策略选择器。
func (f addForm) isProxyField() bool {
	return f.fieldCursor == addFieldProxy
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
//   - 策略由选择器提供，proxyPolicies 为空时才报错（理论上不会发生）；
//   - index 字段为空或可被 strconv.Atoi 解析为 >=1 的整数。
func (f addForm) validate() (bool, string) {
	if !f.isMatchType() {
		if strings.TrimSpace(f.fields[addFieldPayload].Value()) == "" {
			return false, "rules.add_err_payload"
		}
	}
	if f.currentProxy() == "" {
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
