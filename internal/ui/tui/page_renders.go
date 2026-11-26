package tui

import (
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/notice"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/placeholder"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/logs"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/nodes"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/rules"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/settings"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/sub"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const pageTop = layout.TopNavHeight

type mainPageLayout struct {
	width, height int
	help          [][]common.InlineHelpHint
}

func (m *Model) mainPageLayout() mainPageLayout {
	width := max(m.width, common.MinMainWidth)
	height := max(m.height-pageTop-notice.Height-common.StatusBarHeight, common.MinContentHeight)
	firstWidth := width - 1
	var hints []common.InlineHelpHint
	if !m.coreUnavailable() {
		switch m.currentPage {
		case layout.PageNodes:
			hints = m.nodesState.InlineHelpHints()
			firstWidth = m.nodesState.InlineHelpFirstWidth(width)
		case layout.PageConnections:
			hints = m.connsState.InlineHelpHints(width)
		case layout.PageLogs:
			hints = m.logsState.InlineHelpHints()
		case layout.PageRules:
			hints = m.rulesState.InlineHelpHints()
		case layout.PageSub:
			hints = m.subState.InlineHelpHints(width)
		case layout.PageSettings:
			hints = m.settingsState.InlineHelpHints()
		case layout.PageCore:
			hints = m.coreState.InlineHelpHints()
			firstWidth = m.coreState.InlineHelpFirstWidth(width)
		}
	}
	rows := wrapPageHelp(hints, width-1, firstWidth)
	if len(rows) > height-common.MinContentHeight+1 {
		// In a very short terminal, keep the shortcuts accessible while reserving the page itself.
		// Clone hints first so shared/package-level hint slices are never mutated in place.
		compact := make([]common.InlineHelpHint, len(hints))
		copy(compact, hints)
		for i := range compact {
			compact[i].Desc = ""
		}
		rows = wrapPageHelp(compact, width-1, firstWidth)
	}
	return mainPageLayout{width: width, height: max(common.MinContentHeight, height-max(0, len(rows)-1)), help: rows}
}

func wrapPageHelp(hints []common.InlineHelpHint, width, firstWidth int) [][]common.InlineHelpHint {
	if len(hints) == 0 || firstWidth == width {
		return common.WrapInlineHints(hints, width)
	}
	first := common.WrapInlineHints(hints, firstWidth)[0]
	if lipgloss.Width(common.FormatInlineHintRow(first)) > firstWidth {
		first = nil
	}
	return append([][]common.InlineHelpHint{first}, common.WrapInlineHints(hints[len(first):], width)...)
}

func (p mainPageLayout) noticeRow() int { return pageTop + p.height - 1 }
func (p mainPageLayout) statusRow() int { return p.noticeRow() + notice.Height + max(1, len(p.help)) }

func (p mainPageLayout) renderFooter(base string) string {
	if len(p.help) == 0 {
		return base
	}
	rows := make([]string, len(p.help))
	for i := range p.help {
		help, left := p.helpLine(i)
		if i == 0 {
			rows[i] = common.PadString(ansi.Cut(base, 0, left), left) + help
		} else {
			rows[i] = strings.Repeat(" ", left) + help
		}
	}
	return strings.Join(rows, "\n")
}

func (p mainPageLayout) helpLine(row int) (text string, left int) {
	text = ansi.Truncate(common.FormatInlineHintRow(p.help[row]), p.width-1, "…")
	return text, p.width - 1 - lipgloss.Width(text)
}

func (m *Model) getPageSize() (pageWidth, pageHeight int) {
	page := m.mainPageLayout()
	return page.width, page.height
}

func (m *Model) renderPage(width, height int) string {
	if m.coreUnavailable() {
		return placeholder.Render(width, height, m.placeholderState.Frame())
	}
	switch m.currentPage {
	case layout.PageNodes:
		return nodes.RenderNodesPage(m.nodesState.ToPageState(width, height))
	case layout.PageConnections:
		return connections.RenderConnectionsPage(m.connsState.ToPageState(m.chartData, width, height))
	case layout.PageSettings:
		return settings.RenderSettingsPage(m.settingsState.ToPageState(m.config, m.logsState.GetSysLogs()), width, height)
	case layout.PageLogs:
		return logs.RenderLogsPage(m.logsState.ToPageState(width, height))
	case layout.PageRules:
		return rules.RenderRulesPage(m.rulesState.ToPageState(width, height))
	case layout.PageSub:
		return sub.RenderSubPage(m.subState.ToPageState(width, height))
	case layout.PageCore:
		return m.coreState.RenderPage(width, height)
	}
	return ""
}
