package nodes

import (
	"fmt"
	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"slices"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
)

func formatActiveChainSummary(chain service.ProxyChain, maxWidth int) string {
	if !chain.Enabled || len(chain.Nodes) < 2 {
		return ""
	}
	path := strings.Join(chain.Nodes, " → ")
	text := i18n.Tf("nodes.chain.active_badge", path)
	if maxWidth > 0 {
		text = common.TruncateDisplay(text, maxWidth)
	}
	return lipgloss.NewStyle().
		Foreground(theme.Resolve(common.TokyoCyan())).
		Bold(true).
		Render(text)
}

func buildChainModal(state PageState) string {
	m := state.ChainModal
	modalWidth := min(state.Width, max(36, min(96, state.Width-8)))
	innerWidth := max(20, modalWidth-4)

	modalHeight := state.Height - 6
	if modalHeight < 10 {
		modalHeight = 10
	}

	title := i18n.T("nodes.chain.modal_title")
	if m.Group != "" {
		title = i18n.Tf("nodes.chain.modal_title_group", m.Group)
	}

	subtitle := common.DimStyle().Render(common.TruncateDisplay(i18n.T("nodes.chain.modal_subtitle"), innerWidth))
	separator := common.DimStyle().Render(strings.Repeat("─", innerWidth))

	// 1. Chain path preview bar (Entry -> ... -> Exit)
	var pathLine string
	if len(m.DraftNodes) == 0 {
		pathLine = common.MutedStyle().Render(common.TruncateDisplay(i18n.T("nodes.chain.empty_hint"), innerWidth))
	} else {
		pathLine = renderChainPathSummary(m.DraftNodes, innerWidth)
	}

	// Reserve lines for: subtitle(1) + sep(1) + path(1) + sep(1) + hopHeader(1) + candHeader(1) + footer(1) + optional notice(1) = 8
	fixedOverhead := 8
	if m.Notice != "" {
		fixedOverhead++
	}
	availRows := max(4, modalHeight-fixedOverhead)

	hopMaxRows := min(max(2, len(m.DraftNodes)), max(2, availRows/3))
	if len(m.DraftNodes) == 0 {
		hopMaxRows = 1
	}
	candMaxRows := max(2, availRows-hopMaxRows)

	// 2. Chain Hops section
	hopHeaderStyle := common.MutedStyle()
	if m.Focus == chainFocusHops {
		hopHeaderStyle = lipgloss.NewStyle().Foreground(theme.Resolve(common.TokyoCyan())).Bold(true)
	}
	hopHeader := hopHeaderStyle.Render(i18n.Tf("nodes.chain.hops_header", len(m.DraftNodes)))
	hopLines := renderChainHopLines(m, innerWidth, hopMaxRows)

	// 3. Candidate Proxies section
	candHeaderStyle := common.MutedStyle()
	if m.Focus == chainFocusCandidates {
		candHeaderStyle = lipgloss.NewStyle().Foreground(theme.Resolve(common.TokyoCyan())).Bold(true)
	}
	candHeader := candHeaderStyle.Render(i18n.Tf("nodes.chain.candidates_header", len(m.Candidates)))
	candLines := renderChainCandidateLines(m, innerWidth, candMaxRows)

	sections := []string{
		subtitle,
		separator,
		pathLine,
		separator,
		hopHeader,
		strings.Join(hopLines, "\n"),
		candHeader,
		strings.Join(candLines, "\n"),
	}
	if m.Notice != "" {
		noticeStyle := common.ErrorStyle()
		if m.Applying {
			noticeStyle = lipgloss.NewStyle().Foreground(theme.Resolve(common.TokyoCyan()))
		}
		sections = append(sections, noticeStyle.Render(common.TruncateDisplay(m.Notice, innerWidth)))
	}
	footer := common.DimStyle().Render(common.TruncateDisplay(i18n.T("nodes.chain.modal_footer"), innerWidth))
	sections = append(sections, footer)

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return common.RenderBorderedPanel(title, content, modalWidth, common.TokyoBlue(), common.TokyoForeground())
}

func renderChainPathSummary(nodes []string, width int) string {
	accent := lipgloss.NewStyle().Foreground(theme.Resolve(common.TokyoCyan())).Bold(true)
	arrow := common.DimStyle().Render(" → ")
	parts := make([]string, len(nodes))
	for i, name := range nodes {
		role := fmt.Sprintf("[%d]", i+1)
		if i == 0 {
			role = fmt.Sprintf("[1 %s]", i18n.T("nodes.chain.role_entry"))
		} else if i == len(nodes)-1 {
			role = fmt.Sprintf("[%d %s]", i+1, i18n.T("nodes.chain.role_exit"))
		}
		parts[i] = accent.Render(role) + " " + name
	}
	joined := strings.Join(parts, arrow)
	if common.DisplayWidth(joined) > width {
		plain := strings.Join(nodes, " → ")
		return accent.Render(common.TruncateDisplay(plain, width))
	}
	return joined
}

func renderChainHopLines(m ChainModalState, width, maxRows int) []string {
	if len(m.DraftNodes) == 0 {
		return []string{common.DimStyle().Render("  " + i18n.T("nodes.chain.no_hops"))}
	}
	win := resolveListWindow(m.HopCursor, m.HopScroll, maxRows, len(m.DraftNodes))
	selectedStyle := lipgloss.NewStyle().
		Background(theme.Resolve(common.TokyoSelected())).
		Foreground(theme.Resolve(common.TokyoCyan())).
		Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(theme.Resolve(common.TokyoForeground()))
	dimStyle := common.DimStyle()

	lines := make([]string, 0, win.End-win.ScrollTop)
	for i := win.ScrollTop; i < win.End; i++ {
		name := m.DraftNodes[i]
		role := fmt.Sprintf("#%d", i+1)
		switch {
		case i == 0:
			role = fmt.Sprintf("#1 %s", i18n.T("nodes.chain.role_entry"))
		case i == len(m.DraftNodes)-1:
			role = fmt.Sprintf("#%d %s", i+1, i18n.T("nodes.chain.role_exit"))
		}
		detail := ""
		if i > 0 {
			detail = fmt.Sprintf("  (dialer-proxy: %s)", m.DraftNodes[i-1])
		}
		prefix := "  "
		if m.Focus == chainFocusHops && i == m.HopCursor {
			prefix = "▸ "
		}
		raw := common.TruncateDisplay(fmt.Sprintf("%s[%s] %s%s", prefix, role, name, detail), width)
		if m.Focus == chainFocusHops && i == m.HopCursor {
			lines = append(lines, selectedStyle.Render(common.PadString(raw, width)))
		} else if i == 0 || i == len(m.DraftNodes)-1 {
			lines = append(lines, normalStyle.Render(raw))
		} else {
			lines = append(lines, dimStyle.Render(raw))
		}
	}
	return lines
}

func renderChainCandidateLines(m ChainModalState, width, maxRows int) []string {
	if len(m.Candidates) == 0 {
		return []string{common.DimStyle().Render("  " + i18n.T("nodes.chain.no_candidates"))}
	}
	win := resolveListWindow(m.CandidateCursor, m.CandidateScroll, maxRows, len(m.Candidates))
	selectedStyle := lipgloss.NewStyle().
		Background(theme.Resolve(common.TokyoSelected())).
		Foreground(theme.Resolve(common.TokyoCyan())).
		Bold(true)
	inChainStyle := lipgloss.NewStyle().Foreground(theme.Resolve(common.TokyoGreen()))
	normalStyle := lipgloss.NewStyle().Foreground(theme.Resolve(common.TokyoForeground()))

	lines := make([]string, 0, win.End-win.ScrollTop)
	for i := win.ScrollTop; i < win.End; i++ {
		cand := m.Candidates[i]
		hopIdx := slices.Index(m.DraftNodes, cand.Name)
		mark := "[ ]"
		if hopIdx >= 0 {
			mark = fmt.Sprintf("[%d]", hopIdx+1)
		}
		meta := cand.Type
		if cand.IsGroup {
			meta = i18n.T("nodes.chain.tag_group")
		}
		delayStr := ""
		if cand.Delay > 0 {
			delayStr = fmt.Sprintf("%dms", cand.Delay)
		}
		prefix := "  "
		if m.Focus == chainFocusCandidates && i == m.CandidateCursor {
			prefix = "▸ "
		}

		right := strings.TrimSpace(strings.Join([]string{meta, delayStr}, " "))
		leftPart := fmt.Sprintf("%s%s %s", prefix, mark, cand.Name)
		var row string
		if right != "" && width > common.DisplayWidth(right)+8 {
			leftMax := width - common.DisplayWidth(right) - 2
			leftTrunc := common.PadString(common.TruncateDisplay(leftPart, leftMax), leftMax)
			row = leftTrunc + "  " + right
		} else {
			row = common.TruncateDisplay(leftPart, width)
		}

		switch {
		case m.Focus == chainFocusCandidates && i == m.CandidateCursor:
			lines = append(lines, selectedStyle.Render(common.PadString(row, width)))
		case hopIdx >= 0:
			lines = append(lines, inChainStyle.Render(row))
		default:
			lines = append(lines, normalStyle.Render(row))
		}
	}
	return lines
}
