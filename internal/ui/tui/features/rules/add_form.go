package rules

import (
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
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
	"SRC-IP-CIDR",
	"GEOIP",
	"GEOSITE",
	"PROCESS-NAME",
	"PROCESS-PATH",
	"DST-PORT",
	"SRC-PORT",
	"MATCH",
}

// geoipCodeRe 匹配 ISO 3166-1 alpha-2 国家代码（两位大写字母）。
var geoipCodeRe = regexp.MustCompile(`^[A-Z]{2}$`)

// geositeNameRe 匹配 GEOSITE 站点标识符（字母数字、连字符、点、下划线，且以字母数字开头）。
var geositeNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// defaultProxyPolicy 是策略提取失败或列表为空时的兆底值。
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
	addFieldType      = 0 // 类型（只读选择器，←/→ 切换）
	addFieldPayload   = 1 // 匹配值
	addFieldProxy     = 2 // 策略
	addFieldIndex     = 3 // 插入位置
	addFieldNoResolve = 4 // no-resolve 复选框（仅 IP 类可见）
	addFieldCount     = 5 // 字段数
)

// editOrigin 保存编辑模式下原始规则的所有信息。
// 提交时需要先删除旧规则再插入新规则（ReplaceRule），
// 因此需要完整的原始键用于匹配删除。
type editOrigin struct {
	ruleType  string // 原始规则类型（配置文件格式，如 DOMAIN-SUFFIX）
	payload   string // 原始 payload
	proxy     string // 原始策略
	noResolve bool   // 原始 no-resolve 标记
	origIndex int    // 原始规则在 rules 列表中的 0-based 索引（用于位置回填）
}

// addForm 维护「添加/编辑自定义规则」弹窗的表单状态。
//
// textinput.Model 为值类型，State 在 Bubble Tea 中按值传递，每次更新都会
// 复制整个 form；因此本结构的方法均以值接收者返回新值。
//
// 策略(proxy)字段为只读回填：选中值存于 proxySelected，点击策略行聚焦后按
// Enter 可打开「选择策略」二级弹窗（picker 子状态）从中选定并回填。
// addFieldProxy 槽位仍保留一个 textinput.Model 仅作占位以维持字段索引与
// 布局高度一致，其值不参与提交。
// addFieldType / addFieldNoResolve 同样为只读选择器/复选框，槽位保留占位
// textinput.Model 以避免 Focus() 空指针。
//
// isEdit 为 true 时表示当前处于编辑模式（修改已有规则而非新增）。
// editOrigin 保存原始规则的完整信息，提交时走 ReplaceRule（删旧+插新）。
// 位置字段在编辑模式下预填充为原始索引+1，但用户可修改插入位置。
type addForm struct {
	fields      []textinput.Model // length == addFieldCount；type/proxy/noResolve 槽为占位
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

	// no-resolve：仅 IP-CIDR/IP-CIDR6/SRC-IP-CIDR 类型可见
	noResolve bool

	// 编辑模式：isEdit=true 时表示修改已有规则
	isEdit     bool       // 是否为编辑模式
	editOrigin editOrigin // 原始规则信息（仅编辑模式有意义）

	errMsg string // 行内校验/写入错误
}

// newAddForm 构造初始表单：默认类型 DOMAIN-SUFFIX，位置字段留空（默认顶部）。
// 策略分类候选由 configPath 指向的源配置文件提取（proxy-groups / proxies 名称 +
// 内置 DIRECT/REJECT）。proxySelected 默认 DIRECT。默认焦点在 payload（最常用
// 操作为输入匹配值）。
func newAddForm(configPath string) addForm {
	return newFormWithRule(configPath, model.Rule{}, -1, false)
}

// newEditForm 构造编辑模式表单：预填充已有规则的值。
// rule 为原始规则数据，origIndex 为其在 rules 列表中的 0-based 索引。
// 类型选择器定位到原始类型，payload/proxy/noResolve 回填原始值。
// 位置字段预填充为 origIndex+1（保持原位），用户可修改。
func newEditForm(configPath string, rule model.Rule, origIndex int) addForm {
	return newFormWithRule(configPath, rule, origIndex, true)
}

// newFormWithRule 构造表单的通用入口：add 模式传空 rule + origIndex=-1，
// edit 模式传原始规则 + origIndex。
func newFormWithRule(configPath string, rule model.Rule, origIndex int, isEdit bool) addForm {
	fields := make([]textinput.Model, addFieldCount)

	// type 槽位保留 textinput.Model 仅作占位，实际值由 typeCursor + cycleType 切换。
	typeSlot := textinput.New()
	typeSlot.Prompt = ""
	fields[addFieldType] = typeSlot

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

	// noResolve 槽位保留 textinput.Model 仅作占位，避免 Focus() 空指针。
	noRes := textinput.New()
	noRes.Prompt = ""
	fields[addFieldNoResolve] = noRes

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
		isEdit:        isEdit,
	}

	if isEdit && rule.Type != "" {
		// 预填充原始规则值
		normalizedType := normalizeRuleType(rule.Type)
		form.typeCursor = indexOfPreset(normalizedType)
		form.proxySelected = rule.Proxy
		form.noResolve = rule.NoResolve
		form.fields[addFieldPayload].SetValue(rule.Payload)
		// 编辑模式：位置预填充为原索引+1（保持原位），不填则默认原位
		if origIndex >= 0 {
			form.fields[addFieldIndex].SetValue(fmt.Sprintf("%d", origIndex+1))
		}
		form.editOrigin = editOrigin{
			ruleType:  normalizedType,
			payload:   rule.Payload,
			proxy:     rule.Proxy,
			noResolve: rule.NoResolve,
			origIndex: origIndex,
		}
	} else {
		// 添加模式：默认类型 DOMAIN-SUFFIX
		form.typeCursor = indexOfPreset("DOMAIN-SUFFIX")
	}

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
// 非 IP 类型时自动跳过 addFieldNoResolve 字段；MATCH 类型无 payload 时跳过。
func (f *addForm) cycleField(dir int) {
	n := addFieldCount
	next := ((f.fieldCursor+dir)%n + n) % n
	// 非 IP 类型跳过 noResolve 字段
	if next == addFieldNoResolve && !f.isIPType() {
		next = ((next+dir)%n + n) % n
	}
	// MATCH 类型无 payload，跳过
	if next == addFieldPayload && f.isMatchType() {
		next = ((next+dir)%n + n) % n
	}
	f.fieldCursor = next
	f.focusCurrent()
}

// cycleType 循环切换类型（dir=1 下一项，dir=-1 上一项）。
// 切换后若新类型非 IP 类，自动重置 noResolve 状态。
func (f *addForm) cycleType(dir int) {
	n := len(ruleTypePresets)
	f.typeCursor = ((f.typeCursor+dir)%n + n) % n
	if !f.isIPType() {
		f.noResolve = false
	}
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
		if common.FuzzyMatch(f.pickerSearch, c) {
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

// isTypeField 当前聚焦字段是否为类型选择器。
func (f addForm) isTypeField() bool {
	return f.fieldCursor == addFieldType
}

// isIPType 当前类型是否为 IP 类（IP-CIDR / IP-CIDR6 / SRC-IP-CIDR）。
func (f addForm) isIPType() bool {
	t := strings.ToUpper(f.currentType())
	return t == "IP-CIDR" || t == "IP-CIDR6" || t == "SRC-IP-CIDR"
}

// isNoResolveField 当前聚焦字段是否为 no-resolve 复选框。
func (f addForm) isNoResolveField() bool {
	return f.fieldCursor == addFieldNoResolve
}

// toggleNoResolve 切换 no-resolve 复选框状态。
func (f *addForm) toggleNoResolve() {
	f.noResolve = !f.noResolve
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
//   - 非 MATCH 类型 payload 必填，且不允许包含逗号；
//   - IP 类必须为合法的 IP 或 CIDR；
//   - PORT 类必须为合法的端口或范围；
//   - DOMAIN 类不得包含空格；
//   - 策略由选择器提供，proxyPolicies 为空时才报错（理论上不会发生）；
//   - index 字段为空或可被 strconv.Atoi 解析为 >=1 的整数。
func (f addForm) validate() (bool, string) {
	ruleType := strings.ToUpper(f.currentType())
	payload := strings.TrimSpace(f.fields[addFieldPayload].Value())

	if !f.isMatchType() {
		if payload == "" {
			return false, "rules.add_err_payload"
		}
	}

	// 1. 防逗号注入
	if strings.Contains(payload, ",") {
		return false, "rules.add_err_comma"
	}

	// 2. IP 格式强校验
	if strings.HasSuffix(ruleType, "IP-CIDR") || ruleType == "IP-CIDR6" {
		if _, err := netip.ParsePrefix(payload); err != nil {
			return false, "rules.add_err_ip"
		}
	}

	// 3. 端口格式校验
	if ruleType == "DST-PORT" || ruleType == "SRC-PORT" {
		re := regexp.MustCompile(`^(\d+)(?:[-\/](\d+))?$`)
		matches := re.FindStringSubmatch(payload)
		if len(matches) == 0 {
			return false, "rules.add_err_port"
		}
		p1, _ := strconv.Atoi(matches[1])
		if p1 < 0 || p1 > 65535 {
			return false, "rules.add_err_port"
		}
		if matches[2] != "" {
			p2, _ := strconv.Atoi(matches[2])
			if p2 < 0 || p2 > 65535 {
				return false, "rules.add_err_port"
			}
		}
	}

	// 4. 域名/关键字格式校验
	if strings.HasPrefix(ruleType, "DOMAIN") {
		if strings.ContainsAny(payload, " \t") {
			return false, "rules.add_err_domain"
		}
	}

	// 5. GEOIP 国家代码校验（两位大写字母）
	if ruleType == "GEOIP" {
		if !geoipCodeRe.MatchString(strings.ToUpper(payload)) {
			return false, "rules.add_err_geoip"
		}
	}

	// 6. GEOSITE 站点标识符校验
	if ruleType == "GEOSITE" {
		if !geositeNameRe.MatchString(payload) {
			return false, "rules.add_err_geosite"
		}
	}

	// 7. 进程名/路径校验（不允许空格）
	if ruleType == "PROCESS-NAME" || ruleType == "PROCESS-PATH" {
		if strings.ContainsAny(payload, " \t") {
			return false, "rules.add_err_process"
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
