package model

// Rule 规则信息
type Rule struct {
	Type      string `json:"type"`       // 规则类型 (DOMAIN, DOMAIN-SUFFIX, IP-CIDR, etc.)
	Payload   string `json:"payload"`    // 规则内容
	Proxy     string `json:"proxy"`      // 目标代理
	Size      int    `json:"size"`       // 规则集大小（仅 RULE-SET 类型）
	NoResolve bool   `json:"no_resolve"` // 是否跳过 DNS 解析（仅 IP 类规则）
}

// RulesResponse 规则列表响应
type RulesResponse struct {
	Rules []Rule `json:"rules"`
}
