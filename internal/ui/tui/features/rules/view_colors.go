package rules

import (
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	"github.com/charmbracelet/lipgloss"
)

// ruleTypeColors 返回规则类型颜色映射（每次调用动态构建，支持主题热切换）
func ruleTypeColors() map[string]lipgloss.Color {
	return map[string]lipgloss.Color{
		// 标准格式
		"DOMAIN":         common.Secondary(),
		"DOMAIN-SUFFIX":  common.Secondary(),
		"DOMAIN-KEYWORD": common.Info(),
		"IP-CIDR":        common.Purple(),
		"IP-CIDR6":       common.Purple(),
		"GEOIP":          common.Danger(),
		"GEOSITE":        common.Orange(),
		"RULE-SET":       common.Success(),
		"MATCH":          common.Warning(),
		"DIRECT":         common.Gray(),
		// Clash Meta 驼峰格式
		"Domain":        common.Secondary(),
		"DomainSuffix":  common.Secondary(),
		"DomainKeyword": common.Info(),
		"IPCIDR":        common.Purple(),
		"IPCIDR6":       common.Purple(),
		"GeoIP":         common.Danger(),
		"GeoSite":       common.Orange(),
		"RuleSet":       common.Success(),
		"Match":         common.Warning(),
	}
}

// detectAndAdjustDomainColors 检测 Domain 和 DomainSuffix 是否颜色相同，如果是则调整它们。
// 每次调用动态构建（无缓存），以支持主题热切换。
func detectAndAdjustDomainColors(colorAdjustLight, colorAdjustDark float64) map[string]lipgloss.Color {
	// 如果调整参数未设置，使用默认值
	if colorAdjustLight <= 0 {
		colorAdjustLight = 0.25
	}
	if colorAdjustDark <= 0 {
		colorAdjustDark = 0.20
	}

	adjusted := make(map[string]lipgloss.Color)

	// 检查 Domain 和 DomainSuffix 的基础颜色是否相同
	colors := ruleTypeColors()
	domainBaseColor := colors[domainColorKey]
	domainSuffixBaseColor := colors[domainSuffixColorKey]

	if domainBaseColor == "" || domainSuffixBaseColor == "" {
		return adjusted
	}

	baseColorHex := string(domainBaseColor)
	baseColorSuffixHex := string(domainSuffixBaseColor)

	// 如果颜色相同，进行调整
	if utils.ColorStringsEqual(baseColorHex, baseColorSuffixHex) {
		// 生成较浅和较深的变体
		lighterHex, err := utils.LighterColor(baseColorHex, colorAdjustLight)
		if err == nil {
			adjusted[domainColorKey] = lipgloss.Color(lighterHex)
		}

		darkerHex, err := utils.DarkerColor(baseColorSuffixHex, colorAdjustDark)
		if err == nil {
			adjusted[domainSuffixColorKey] = lipgloss.Color(darkerHex)
		}

		// 同时处理大写格式
		adjusted["DOMAIN"] = adjusted[domainColorKey]
		adjusted["DOMAIN-SUFFIX"] = adjusted[domainSuffixColorKey]
	}

	return adjusted
}

// getAdjustedRuleTypeColor 获取调整后的规则类型颜色
func getAdjustedRuleTypeColor(ruleType string, adjustedColors map[string]lipgloss.Color) lipgloss.Color {
	if adjustedColor, ok := adjustedColors[ruleType]; ok {
		return adjustedColor
	}
	if baseColor, ok := ruleTypeColors()[ruleType]; ok {
		return baseColor
	}
	return common.Gray()
}
