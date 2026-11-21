package rules

import "strings"

// Edit represents a single declarative mutation against a Mihomo rules list.
// Match identifies the first matching rule to delete or replace.
// Index is one-based, with zero retaining an edited rule's position (or inserting at top for additions).
type Edit struct {
	Match string `yaml:"match,omitempty" json:"match,omitempty"`
	Rule  string `yaml:"rule,omitempty" json:"rule,omitempty"`
	Index int    `yaml:"index,omitempty" json:"index,omitempty"`
}

// camelToConfigType maps Mihomo API CamelCase rule type names to config-file uppercase hyphen format.
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

// NormalizeType maps known Mihomo API rule type names (e.g. "DomainSuffix", "IPCIDR")
// to their canonical config-file form (e.g. "DOMAIN-SUFFIX", "IP-CIDR").
func NormalizeType(apiType string) (string, bool) {
	if strings.EqualFold(apiType, "IP-CIDR6") {
		return "IP-CIDR6", true
	}
	mapped, ok := camelToConfigType[apiType]
	return mapped, ok
}

// IsIPRuleType reports whether ruleType is an IP-based rule that supports the no-resolve option.
func IsIPRuleType(ruleType string) bool {
	switch strings.ToUpper(strings.TrimSpace(ruleType)) {
	case "IP-CIDR", "IP-CIDR6", "SRC-IP-CIDR":
		return true
	}
	return false
}

// Canonicalize normalizes a Mihomo rule string for content-based comparison:
// trims whitespace around comma-separated segments, uppercases the rule type,
// and normalizes a trailing no-resolve flag to lowercase.
func Canonicalize(rule string) string {
	parts := strings.Split(rule, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	parts[0] = strings.ToUpper(parts[0])
	if last := len(parts) - 1; last > 0 && strings.EqualFold(parts[last], "no-resolve") {
		parts[last] = "no-resolve"
	}
	return strings.Join(parts, ",")
}

// ParseKey parses a Mihomo rule line into a normalized lookup key ("TYPE,PAYLOAD,PROXY"
// or "MATCH,,PROXY") and its no-resolve flag.
func ParseKey(line string) (key string, noResolve bool) {
	parts := strings.Split(line, ",")
	if len(parts) < 2 {
		return "", false
	}

	ruleType := strings.ToUpper(strings.TrimSpace(parts[0]))
	if ruleType == "MATCH" {
		proxy := strings.TrimSpace(parts[1])
		return ruleType + ",," + proxy, false
	}

	if len(parts) < 3 {
		return "", false
	}
	payload := strings.TrimSpace(parts[1])
	proxy := strings.TrimSpace(parts[2])
	noResolve = len(parts) >= 4 && strings.EqualFold(strings.TrimSpace(parts[3]), "no-resolve")
	return ruleType + "," + payload + "," + proxy, noResolve
}

// BuildLine formats a new or edited Mihomo rule line.
// MATCH has no payload ("MATCH,PROXY"); no-resolve is appended only for IP rule types.
func BuildLine(ruleType, payload, proxy string, noResolve bool) string {
	ruleType = strings.TrimSpace(ruleType)
	payload = strings.TrimSpace(payload)
	proxy = strings.TrimSpace(proxy)

	if strings.EqualFold(ruleType, "MATCH") {
		return "MATCH," + proxy
	}
	line := ruleType + "," + payload + "," + proxy
	if noResolve && IsIPRuleType(ruleType) {
		line += ",no-resolve"
	}
	return line
}

// BuildMatchLine formats a match key line for an existing rule.
// Unlike BuildLine, it preserves a trailing no-resolve flag on any non-MATCH rule.
func BuildMatchLine(ruleType, payload, proxy string, noResolve bool) string {
	line := BuildLine(ruleType, payload, proxy, false)
	if noResolve && !strings.EqualFold(strings.TrimSpace(ruleType), "MATCH") {
		line += ",no-resolve"
	}
	return line
}

// NormalizeInsertIndex converts a 1-based rule insertion index into a clamped 0-based slice index.
func NormalizeInsertIndex(index, length int) int {
	if index < 1 {
		return 0
	}
	if index > length {
		return length
	}
	return index - 1
}
