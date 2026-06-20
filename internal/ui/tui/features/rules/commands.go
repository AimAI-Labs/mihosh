package rules

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
)

// FetchRules 获取规则列表，并从配置文件合并 no-resolve 标记。
func FetchRules(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		rules, err := client.GetRules()
		if err != nil {
			return messages.ErrMsg{Err: err}
		}

		// 从配置文件解析 no-resolve 信息（API 不返回此字段）
		if configPath, pathErr := config.GetMihomoConfigPath(); pathErr == nil {
			nrMap := config.ParseRulesNoResolve(configPath)
			for i := range rules.Rules {
				r := &rules.Rules[i]
				key := normalizeRuleType(r.Type) + "," + r.Payload + "," + r.Proxy
				if nrMap[key] {
					r.NoResolve = true
				}
			}
		}

		return messages.RulesMsg(rules.Rules)
	}
}

// camelToConfigType 将 Mihomo API 返回的 CamelCase 类型名转换为配置文件中使用的大写横杠分隔格式。
// 例如: "IPCIDR" → "IP-CIDR", "DomainSuffix" → "DOMAIN-SUFFIX"。
// ParseRulesNoResolve 使用 strings.ToUpper 标准化键，所以这里也必须输出大写格式。
var camelToConfigType = map[string]string{
	"Domain":        "DOMAIN",
	"DomainSuffix":  "DOMAIN-SUFFIX",
	"DomainKeyword": "DOMAIN-KEYWORD",
	"IPCIDR":        "IP-CIDR",
	"IPCIDR6":       "IP-CIDR6",
	"SrcIPCIDR":     "SRC-IP-CIDR",
	"GeoIP":         "GEOIP",
	"GeoSite":       "GEOSITE",
	"RuleSet":       "RULE-SET",
	"Match":         "MATCH",
}

// normalizeRuleType 将 API 返回的规则类型标准化为配置文件格式（大写横杠分隔）。
// 如果类型不在映射表中，回退为 strings.ToUpper。
func normalizeRuleType(apiType string) string {
	if mapped, ok := camelToConfigType[apiType]; ok {
		return mapped
	}
	return strings.ToUpper(apiType)
}

// AddRuleCmd 把一条自定义规则写入 Mihomo 配置文件的 rules 列表。
//
// ruleType / payload / proxy 为表单输入；index 为 1-based 插入位置，
// index <= 0 表示插入到顶部（最高优先级）。
// noResolve 为 true 时在规则末尾追加 ,no-resolve（仅 IP 类规则有效）。
// 成功返回 RuleAddedMsg，失败返回 RuleAddErrorMsg。
func AddRuleCmd(configPath, ruleType, payload, proxy string, index int, noResolve bool) tea.Cmd {
	rule := buildRuleLine(ruleType, payload, proxy, noResolve)
	return func() tea.Msg {
		if err := config.InsertRule(configPath, rule, index); err != nil {
			return messages.RuleAddErrorMsg{Err: err}
		}
		return messages.RuleAddedMsg{}
	}
}

// DeleteRuleCmd 从 Mihomo 配置文件的 rules 列表中删除指定规则。
//
// 匹配基于内容（type + payload + proxy + no-resolve），见 config.DeleteRule。
// ruleType 必须为配置文件格式（大写横杠分隔），调用方负责把 API 返回的
// CamelCase 类型经 normalizeRuleType 转换后传入。
// 成功返回 RuleDeletedMsg，未匹配或写盘失败返回 RuleDeleteErrorMsg。
func DeleteRuleCmd(configPath, ruleType, payload, proxy string, noResolve bool) tea.Cmd {
	return func() tea.Msg {
		if err := config.DeleteRule(configPath, ruleType, payload, proxy, noResolve); err != nil {
			return messages.RuleDeleteErrorMsg{Err: err}
		}
		return messages.RuleDeletedMsg{}
	}
}

// EditRuleCmd 修改一条自定义规则：先删除旧规则再插入新规则（ReplaceRule）。
//
// oldType/oldPayload/oldProxy/oldNoResolve 为旧规则的匹配键。
// newType/newPayload/newProxy 为新规则值。
// newIndex 为 1-based 插入位置。
// newNoResolve 为新规则是否带 no-resolve。
// 成功返回 RuleEditedMsg，未匹配或写盘失败返回 RuleEditErrorMsg。
func EditRuleCmd(configPath, oldType, oldPayload, oldProxy string, oldNoResolve bool,
	newType, newPayload, newProxy string, newIndex int, newNoResolve bool) tea.Cmd {
	newRule := buildRuleLine(newType, newPayload, newProxy, newNoResolve)
	return func() tea.Msg {
		if err := config.ReplaceRule(configPath, oldType, oldPayload, oldProxy, oldNoResolve, newRule, newIndex); err != nil {
			return messages.RuleEditErrorMsg{Err: err}
		}
		return messages.RuleEditedMsg{}
	}
}

// buildRuleLine 按 Mihomo 源格式 `TYPE,PAYLOAD,PROXY` 组装规则字符串。
// MATCH 类型无需 payload，输出 `MATCH,PROXY`。
// noResolve 为 true 时在规则末尾追加 `,no-resolve`。
// payload/proxy 两侧的空白被裁剪。
func buildRuleLine(ruleType, payload, proxy string, noResolve bool) string {
	ruleType = strings.TrimSpace(ruleType)
	payload = strings.TrimSpace(payload)
	proxy = strings.TrimSpace(proxy)

	suffix := ""
	if noResolve && isIPRuleType(ruleType) {
		suffix = ",no-resolve"
	}

	if strings.EqualFold(ruleType, "MATCH") {
		return fmt.Sprintf("MATCH,%s", proxy)
	}
	return fmt.Sprintf("%s,%s,%s%s", ruleType, payload, proxy, suffix)
}

// isIPRuleType 判断规则类型是否为基于 IP 的（支持 no-resolve 选项）。
func isIPRuleType(ruleType string) bool {
	switch strings.ToUpper(ruleType) {
	case "IP-CIDR", "IP-CIDR6", "SRC-IP-CIDR":
		return true
	}
	return false
}
