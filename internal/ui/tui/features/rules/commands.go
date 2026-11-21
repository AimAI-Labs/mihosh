package rules

import (
	"context"
	"errors"
	"fmt"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	domainrules "github.com/AimAI-Labs/mihosh/internal/domain/rules"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
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
		nrMap := service.RuleNoResolveFlags()
		for i := range rules.Rules {
			r := &rules.Rules[i]
			key := normalizeRuleType(r.Type) + "," + r.Payload + "," + r.Proxy
			r.NoResolve = nrMap[key]
		}

		return messages.RulesMsg(rules.Rules)
	})
}

// normalizeRuleType 将 API 返回的规则类型标准化为配置文件格式（大写横杠分隔）。
// 其余已知类型使用控制器与类型筛选共用的名称映射。
func normalizeRuleType(apiType string) string {
	if mapped, ok := domainrules.NormalizeType(apiType); ok {
		return mapped
	}
	return filterTypeName(apiType)
}

func (s State) applyRuleEdit(noticeKey string, edit domainrules.Edit) tea.Cmd {
	configSvc := s.configSvc
	if configSvc == nil {
		configSvc = service.NewConfigService()
	}
	return func() tea.Msg {
		result := configSvc.ApplyRuleEdit(context.Background(), s.coreSvc, s.client, edit)
		err := result.Err
		switch {
		case errors.Is(err, service.ErrRuleEditTarget):
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
	return domainrules.BuildMatchLine(ruleType, payload, proxy, noResolve)
}

func buildRuleLine(ruleType, payload, proxy string, noResolve bool) string {
	return domainrules.BuildLine(ruleType, payload, proxy, noResolve)
}
