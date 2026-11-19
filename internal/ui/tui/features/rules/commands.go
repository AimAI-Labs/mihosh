package rules

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
)

// FetchRules 获取规则列表，并从配置文件合并 no-resolve 标记。
func FetchRules(client *api.Client) tea.Cmd {
	return messages.CoreRequest(client, func(client *api.Client) tea.Msg {
		rules, err := client.GetRules()
		if err != nil {
			return messages.ErrMsg{Err: err}
		}

		// 从配置文件解析 no-resolve 信息（API 不返回此字段）
		nrMap := ruleNoResolveFlags()
		for i := range rules.Rules {
			r := &rules.Rules[i]
			key := normalizeRuleType(r.Type) + "," + r.Payload + "," + r.Proxy
			r.NoResolve = nrMap[key]
		}

		return messages.RulesMsg(rules.Rules)
	})
}

func ruleNoResolveFlags() map[string]bool {
	path, err := config.GetMihomoConfigPath()
	if err == nil {
		return config.ParseRulesNoResolve(path)
	}
	if errors.Is(err, config.ErrExternalConfig) {
		if cfg, loadErr := config.Load(); loadErr == nil && cfg.ActiveSub != "" {
			if generated, generateErr := profile.GenerateForUID(cfg.ActiveSub, ""); generateErr == nil {
				return config.ParseRulesNoResolveYAML(generated.Data)
			}
		}
	}
	return nil
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

func (s State) applyRuleEdit(noticeKey string, edit profile.RuleEdit) tea.Cmd {
	configSvc := s.configSvc
	if configSvc == nil {
		configSvc = service.NewConfigService()
	}
	return func() tea.Msg {
		result := configSvc.ApplyRuleEdit(context.Background(), s.coreSvc, s.client, edit)
		err := result.Err
		switch {
		case errors.Is(err, profile.ErrRuleEditTarget):
			err = errors.New(i18n.T("rules.write.not_found"))
		case errors.Is(err, service.ErrRemoteRuleEditRequiresSubscription):
			err = errors.New(i18n.T("rules.write.remote_needs_subscription"))
		case err != nil && result.Restored:
			err = fmt.Errorf("%s (%w)", i18n.T("settings.toast.mihomo_override_restored"), err)
		}
		return messages.RuleEditAppliedMsg{
			NoticeKey: noticeKey, Applied: result.Applied, Remote: result.Remote,
			NeedsRestart: result.NeedsRestart, Restarted: result.Restarted,
			EndpointChanged: result.EndpointChanged, Err: err,
		}
	}
}

func originalRuleLine(ruleType, payload, proxy string, noResolve bool) string {
	line := buildRuleLine(ruleType, payload, proxy, false)
	if noResolve && !strings.EqualFold(ruleType, "MATCH") {
		line += ",no-resolve"
	}
	return line
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
