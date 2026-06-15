package rules

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
)

// FetchRules 获取规则列表
func FetchRules(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		rules, err := client.GetRules()
		if err != nil {
			return messages.ErrMsg{Err: err}
		}
		return messages.RulesMsg(rules.Rules)
	}
}

// AddRuleCmd 把一条自定义规则写入 Mihomo 配置文件的 rules 列表。
//
// ruleType / payload / proxy 为表单输入；index 为 1-based 插入位置，
// index <= 0 表示插入到顶部（最高优先级）。
// 成功返回 RuleAddedMsg，失败返回 RuleAddErrorMsg。
func AddRuleCmd(configPath, ruleType, payload, proxy string, index int) tea.Cmd {
	rule := buildRuleLine(ruleType, payload, proxy)
	return func() tea.Msg {
		if err := config.InsertRule(configPath, rule, index); err != nil {
			return messages.RuleAddErrorMsg{Err: err}
		}
		return messages.RuleAddedMsg{}
	}
}

// buildRuleLine 按 Mihomo 源格式 `TYPE,PAYLOAD,PROXY` 组装规则字符串。
// MATCH 类型无需 payload，输出 `MATCH,PROXY`。
// payload/proxy 两侧的空白被裁剪。
func buildRuleLine(ruleType, payload, proxy string) string {
	ruleType = strings.TrimSpace(ruleType)
	payload = strings.TrimSpace(payload)
	proxy = strings.TrimSpace(proxy)
	if strings.EqualFold(ruleType, "MATCH") {
		return fmt.Sprintf("MATCH,%s", proxy)
	}
	return fmt.Sprintf("%s,%s,%s", ruleType, payload, proxy)
}
