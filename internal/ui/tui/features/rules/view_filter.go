package rules

import (
	"slices"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Built-in Mihomo routing types, plus any additional types advertised by the controller.
var builtinFilterTypes = []string{
	"DOMAIN", "DOMAIN-SUFFIX", "DOMAIN-KEYWORD", "DOMAIN-WILDCARD", "DOMAIN-REGEX", "GEOSITE",
	"IP-CIDR", "IP-SUFFIX", "IP-ASN", "GEOIP", "SRC-GEOIP", "SRC-IP-ASN", "SRC-IP-CIDR", "SRC-IP-SUFFIX",
	"DST-PORT", "SRC-PORT", "IN-PORT", "IN-TYPE", "IN-USER", "IN-NAME", "REMATCH-NAME",
	"PROCESS-PATH", "PROCESS-PATH-WILDCARD", "PROCESS-PATH-REGEX", "PROCESS-NAME", "PROCESS-NAME-WILDCARD", "PROCESS-NAME-REGEX",
	"UID", "NETWORK", "DSCP", "RULE-SET", "AND", "OR", "NOT", "SUB-RULE", "MATCH",
}

var filterTypeNames = func() map[string]string {
	names := make(map[string]string, len(builtinFilterTypes))
	for _, name := range builtinFilterTypes {
		names[strings.ReplaceAll(name, "-", "")] = name
	}
	// Mihomo reports IP-CIDR6 as IPCIDR and SUB-RULE as SubRules.
	names["IPCIDR6"] = "IP-CIDR"
	names["SUBRULES"] = "SUB-RULE"
	return names
}()

func filterTypeName(name string) string {
	name = strings.ToUpper(name)
	if label, ok := filterTypeNames[strings.ReplaceAll(name, "-", "")]; ok {
		return label
	}
	return name
}

func ruleFilterTypes(present []string) []string {
	types := builtinFilterTypes
	for _, name := range present {
		if !slices.Contains(builtinFilterTypes, name) {
			if len(types) == len(builtinFilterTypes) {
				types = slices.Clone(builtinFilterTypes)
			}
			types = append(types, name)
		}
	}
	return types
}

type typeFilterHit struct {
	name               string
	index, x, y, width int
	enabled            bool
}

func typeFilterLayout(state PageState) ([]string, []typeFilterHit) {
	if !state.ShowTypeFilter {
		return nil, nil
	}
	var rows []string
	var hits []typeFilterHit
	var line string
	x, focusRow := 0, 0
	width := max(1, state.Width)
	for i, name := range ruleFilterTypes(state.AvailableTypes) {
		label := ansi.Truncate(" "+name+" ", width, "…")
		span := lipgloss.Width(label)
		if x > 0 && x+1+span > width {
			rows = append(rows, line)
			line, x = "", 0
		}
		if x > 0 {
			line += " "
			x++
		}
		present := slices.Contains(state.AvailableTypes, name)
		style := common.MutedStyle()
		if present {
			style = common.TableHeaderStyle()
			if slices.Contains(state.SelectedTypes, name) {
				style = common.SelectedStyle().Bold(true)
			}
		}
		if i == state.TypeFilterCursor {
			focusRow = len(rows)
			style = style.Underline(true)
		}
		hits = append(hits, typeFilterHit{name: name, index: i, x: x, y: len(rows), width: span, enabled: present})
		line += style.Render(label)
		x += span
	}
	rows = append(rows, line)
	count := min(len(rows), max(1, state.Height-rulesFixedLines-rulesMinHeight))
	start := min(len(rows)-count, max(0, focusRow-count+1))
	visible := hits[:0]
	for _, hit := range hits {
		if hit.y >= start && hit.y < start+count {
			hit.y += rulesHeaderHeight - start
			visible = append(visible, hit)
		}
	}
	return rows[start : start+count], visible
}
