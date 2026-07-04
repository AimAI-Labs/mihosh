package rules

import (
	"fmt"
	"strings"

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

// animateColor 计算平滑过渡动画后的颜色
func animateColor(from, to lipgloss.Color, progress float64) lipgloss.Color {
	fromStr := string(from)
	toStr := string(to)

	if fromStr == toStr {
		return from
	}

	fromHex := strings.TrimPrefix(fromStr, "#")
	toHex := strings.TrimPrefix(toStr, "#")

	if len(fromHex) != 6 || len(toHex) != 6 {
		return to
	}

	fromR := hexToInt(fromHex[0:2])
	fromG := hexToInt(fromHex[2:4])
	fromB := hexToInt(fromHex[4:6])

	toR := hexToInt(toHex[0:2])
	toG := hexToInt(toHex[2:4])
	toB := hexToInt(toHex[4:6])

	newR := int(float64(fromR) + float64(toR-fromR)*progress)
	newG := int(float64(fromG) + float64(toG-fromG)*progress)
	newB := int(float64(fromB) + float64(toB-fromB)*progress)

	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", newR, newG, newB))
}

// hexToInt 将十六进制字符串转换为整数
func hexToInt(s string) int {
	var val int
	for _, c := range s {
		val *= 16
		switch {
		case c >= '0' && c <= '9':
			val += int(c - '0')
		case c >= 'A' && c <= 'F':
			val += int(c - 'A' + 10)
		case c >= 'a' && c <= 'f':
			val += int(c - 'a' + 10)
		}
	}
	return val
}

// interpolateColor 在两个颜色之间进行线性插值
func interpolateColor(color1, color2 string, t float64) string {
	c1 := strings.TrimPrefix(color1, "#")
	c2 := strings.TrimPrefix(color2, "#")

	if len(c1) != 6 || len(c2) != 6 {
		return color2
	}

	r1 := hexToInt(c1[0:2])
	g1 := hexToInt(c1[2:4])
	b1 := hexToInt(c1[4:6])

	r2 := hexToInt(c2[0:2])
	g2 := hexToInt(c2[2:4])
	b2 := hexToInt(c2[4:6])

	r := int(float64(r1) + float64(r2-r1)*t)
	g := int(float64(g1) + float64(g2-g1)*t)
	b := int(float64(b1) + float64(b2-b1)*t)

	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}
