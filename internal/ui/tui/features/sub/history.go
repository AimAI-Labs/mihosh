package sub

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/dialog"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type historyMode int

const (
	historyClosed historyMode = iota
	historyList
	historyDiff
)

type historyView struct {
	mode      historyMode
	kind      profile.SnapshotKind
	uid       string
	subName   string
	request   uint64
	loading   bool
	snapshots []profile.Snapshot
	selected  int
	diff      []profile.FileDiffRow
	top       int
	err       error
	ignore    profile.FileDiffOptions
}

type historyDialog struct {
	dialog.Dialog
	rawWidth, mergeWidth int
	listStart, listEnd   int
	hintTop, innerWidth  int
	hints                [][]common.InlineHelpHint
}

func (s State) openHistoryModal() (State, tea.Cmd) {
	var uid, name string
	kind := profile.SnapshotMerge
	if len(s.filteredIdx) > 0 && s.selected >= 0 && s.selected < len(s.filteredIdx) {
		p := s.subs[s.filteredIdx[s.selected]]
		uid = p.UID
		name = p.Name
		kind = profile.SnapshotRaw
	}
	ignore := profile.DefaultFileDiffOptions
	if cfg, err := config.Load(); err == nil && cfg.FileDiffOptions != nil {
		ignore = *cfg.FileDiffOptions
	}
	req := s.history.request + 1
	s.history = historyView{
		mode:    historyList,
		kind:    kind,
		uid:     uid,
		subName: name,
		request: req,
		loading: true,
		ignore:  ignore,
	}
	return s, s.loadHistoryListCmd()
}

func (s State) loadHistoryListCmd() tea.Cmd {
	h := s.history
	svc := s.svc
	return func() tea.Msg {
		if svc == nil {
			return messages.SubHistoryLoadedMsg{Request: h.request, Kind: h.kind, UID: h.uid, Err: errNoService}
		}
		var (
			snaps []profile.Snapshot
			err   error
		)
		if h.kind == profile.SnapshotRaw {
			snaps, err = svc.ListRawSnapshots(h.uid)
		} else {
			snaps, err = svc.ListMergeSnapshots()
		}
		return messages.SubHistoryLoadedMsg{
			Request:   h.request,
			Kind:      h.kind,
			UID:       h.uid,
			Snapshots: snaps,
			Err:       err,
		}
	}
}

func (s State) loadHistoryDiffCmd() tea.Cmd {
	h := s.history
	svc := s.svc
	if h.selected < 0 || h.selected >= len(h.snapshots) {
		return nil
	}
	snapID := h.snapshots[h.selected].ID
	return func() tea.Msg {
		if svc == nil {
			return messages.SubHistoryDiffMsg{Request: h.request, Err: errNoService}
		}
		rows, err := svc.DiffSnapshotWithCurrent(h.kind, h.uid, snapID, h.ignore)
		return messages.SubHistoryDiffMsg{Request: h.request, Rows: rows, Err: err}
	}
}

func (s State) rollbackSelectedSnapshot() (State, tea.Cmd) {
	if s.history.selected < 0 || s.history.selected >= len(s.history.snapshots) {
		return s, nil
	}
	snap := s.history.snapshots[s.history.selected]
	kind := s.history.kind
	uid := s.history.uid
	svc := s.svc
	s.history.mode = historyClosed
	return s, func() tea.Msg {
		if svc == nil {
			return messages.SubRollbackDoneMsg{Kind: kind, UID: uid, SnapshotID: snap.ID, Err: errNoService}
		}
		res, err := svc.RollbackSnapshot(kind, uid, snap.ID)
		msg := messages.SubRollbackDoneMsg{
			Kind:       kind,
			UID:        uid,
			SnapshotID: snap.ID,
			Err:        err,
		}
		if res != nil {
			msg.MergeErr = res.MergeErr
			msg.Restarted = res.Restarted
			msg.NeedsRestart = res.NeedsRestart
			msg.Remote = res.Remote
		}
		return msg
	}
}

func (s State) handleHistoryKey(msg tea.KeyMsg) (State, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if s.history.mode == historyDiff {
			s.history.mode = historyList
			s.history.diff = nil
			s.history.top = 0
			s.history.err = nil
			return s, nil
		}
		s.history.mode = historyClosed
		return s, nil
	case "tab", "left", "right":
		if s.history.mode == historyList {
			if s.history.kind == profile.SnapshotRaw {
				return s.selectHistoryKind(profile.SnapshotMerge)
			}
			return s.selectHistoryKind(profile.SnapshotRaw)
		}
	case "up", "k":
		if s.history.mode == historyList {
			if s.history.selected > 0 {
				s.history.selected--
			}
		} else if s.history.top > 0 {
			s.history.top--
		}
	case "down", "j":
		if s.history.mode == historyList {
			if s.history.selected < len(s.history.snapshots)-1 {
				s.history.selected++
			}
		} else {
			s.history.top++
		}
	case "enter", "d":
		if s.history.mode == historyList && len(s.history.snapshots) > 0 {
			s.history.mode = historyDiff
			s.history.loading = true
			s.history.top = 0
			s.history.diff = nil
			s.history.err = nil
			s.history.request++
			return s, s.loadHistoryDiffCmd()
		}
	case "r":
		if len(s.history.snapshots) > 0 {
			return s.rollbackSelectedSnapshot()
		}
	}
	return s, nil
}

func (s State) selectHistoryKind(kind profile.SnapshotKind) (State, tea.Cmd) {
	h := &s.history
	if h.kind == kind || (kind == profile.SnapshotRaw && h.uid == "") {
		return s, nil
	}
	h.kind = kind
	h.request++
	h.loading, h.selected, h.snapshots, h.err = true, 0, nil, nil
	return s, s.loadHistoryListCmd()
}

func (s State) clickHistory(msg messages.PageMouseClickMsg) (State, tea.Cmd) {
	d := s.history.dialog(msg.Width, msg.Height)
	left, top, right, bottom := d.Bounds()
	if msg.X < left || msg.X >= right || msg.Y < top || msg.Y >= bottom {
		s.history.mode = historyClosed
		return s, nil
	}
	x, y := msg.X-left-2, msg.Y-top
	if y == 1 && s.history.mode == historyList && x >= 0 {
		if x < d.rawWidth {
			return s.selectHistoryKind(profile.SnapshotRaw)
		}
		if x >= d.rawWidth+2 && x < d.rawWidth+2+d.mergeWidth {
			return s.selectHistoryKind(profile.SnapshotMerge)
		}
	}
	if row := y - 3; s.history.mode == historyList && row >= 0 && row < d.listEnd-d.listStart && x >= 0 && x < d.innerWidth {
		s.history.selected = d.listStart + row
	}
	if row := y - d.hintTop; row >= 0 && row < len(d.hints) {
		if hint, ok := common.InlineHintAt(d.hints[row], x, d.innerWidth); ok {
			key, _ := hint.KeyMsg()
			return s.handleHistoryKey(key)
		}
	}
	return s, nil
}

func (h historyView) dialog(width, height int) historyDialog {
	panelWidth := max(24, min(110, width-4))
	innerWidth := panelWidth - 4
	d := historyDialog{innerWidth: innerWidth}
	hints := []common.InlineHelpHint{
		{Key: "Tab", Desc: i18n.T("sub.history.hint_tab")},
		{Key: "↵/d", Desc: i18n.T("sub.history.hint_diff"), Action: "enter", Tone: common.InlineHelpToneInfo},
		{Key: "r", Desc: i18n.T("sub.history.hint_rollback"), Tone: common.InlineHelpToneWarning},
		{Key: "Esc", Desc: i18n.T("core.inline.cancel")},
	}
	if h.mode == historyDiff {
		hints = []common.InlineHelpHint{
			{Key: "↑↓", Desc: i18n.T("core.inline.scroll")},
			hints[2],
			{Key: "Esc", Desc: i18n.T("sub.history.hint_back")},
		}
	}
	d.hints = common.WrapInlineHints(hints, innerWidth)
	t := theme.Current()

	activeTab := lipgloss.NewStyle().Background(theme.Resolve(t.Primary)).Foreground(theme.Resolve(t.Background)).Bold(true)
	inactiveTab := lipgloss.NewStyle().Foreground(theme.Resolve(t.Muted))
	selectedRow := lipgloss.NewStyle().Background(theme.Resolve(t.Selected)).Foreground(theme.Resolve(t.Primary)).Bold(true)
	dim := common.DimStyle()
	errStyle := lipgloss.NewStyle().Foreground(theme.Resolve(t.Danger))
	addStyle := lipgloss.NewStyle().Foreground(theme.Resolve(t.Success))
	delStyle := lipgloss.NewStyle().Foreground(theme.Resolve(t.Danger))

	var body []string
	var title string
	if h.mode == historyDiff {
		title = i18n.T("sub.history.diff_title")
		if h.loading {
			body = append(body, dim.Render(i18n.T("sub.history.loading")))
		} else if h.err != nil {
			body = append(body, errStyle.Render(h.err.Error()))
		} else if len(h.diff) == 0 {
			body = append(body, dim.Render(i18n.T("sub.history.diff_identical")))
		} else {
			budget := max(1, height-5-len(d.hints))
			top := min(h.top, max(0, len(h.diff)-budget))
			end := min(len(h.diff), top+budget)
			half := (innerWidth - 3) / 2
			header := common.PadString(i18n.T("sub.history.col_before"), half) + " │ " + i18n.T("sub.history.col_after")
			body = append(body, lipgloss.NewStyle().Foreground(theme.Resolve(t.Primary)).Bold(true).Render(ansi.Truncate(header, innerWidth, "")))
			for _, row := range h.diff[top:end] {
				before := ansi.Truncate(terminalFileText([]byte(row.Before)), half, "…")
				after := ansi.Truncate(terminalFileText([]byte(row.After)), half, "…")
				bStr := common.PadString(before, half)
				aStr := common.PadString(after, half)
				if strings.HasPrefix(row.Before, "- ") {
					bStr = delStyle.Render(bStr)
				} else if strings.HasPrefix(row.Before, "@@") {
					bStr = dim.Render(bStr)
				}
				if strings.HasPrefix(row.After, "+ ") {
					aStr = addStyle.Render(aStr)
				} else if strings.HasPrefix(row.After, "@@") {
					aStr = dim.Render(aStr)
				}
				body = append(body, bStr+" │ "+aStr)
			}
		}
	} else {
		title = i18n.T("sub.history.title")
		rawTabLabel := " " + i18n.T("sub.history.tab_raw")
		if h.subName != "" {
			rawTabLabel += " (" + h.subName + ")"
		}
		rawTabLabel += " "
		mergeTabLabel := " " + i18n.T("sub.history.tab_merge") + " "
		if lipgloss.Width(rawTabLabel)+2+lipgloss.Width(mergeTabLabel) > innerWidth {
			rawTabLabel = ansi.Truncate(rawTabLabel, max((innerWidth-2)/2, innerWidth-2-lipgloss.Width(mergeTabLabel)), "…")
			mergeTabLabel = ansi.Truncate(mergeTabLabel, innerWidth-2-lipgloss.Width(rawTabLabel), "…")
		}
		d.rawWidth, d.mergeWidth = lipgloss.Width(rawTabLabel), lipgloss.Width(mergeTabLabel)

		var tabsLine string
		if h.kind == profile.SnapshotRaw {
			tabsLine = activeTab.Render(rawTabLabel) + "  " + inactiveTab.Render(mergeTabLabel)
		} else {
			rawStyle := inactiveTab
			if h.uid == "" {
				rawStyle = dim
			}
			tabsLine = rawStyle.Render(rawTabLabel) + "  " + activeTab.Render(mergeTabLabel)
		}
		body = append(body, tabsLine)
		body = append(body, dim.Render(strings.Repeat("─", innerWidth)))

		if h.loading {
			body = append(body, dim.Render(i18n.T("sub.history.loading")))
		} else if h.err != nil {
			body = append(body, errStyle.Render(h.err.Error()))
		} else if len(h.snapshots) == 0 {
			body = append(body, dim.Render(i18n.T("sub.history.empty")))
		} else {
			budget := max(1, height-6-len(d.hints))
			start := 0
			if h.selected >= budget {
				start = h.selected - budget + 1
			}
			end := min(len(h.snapshots), start+budget)
			d.listStart, d.listEnd = start, end
			for i := start; i < end; i++ {
				snap := h.snapshots[i]
				prefix := "  "
				if i == h.selected {
					prefix = "› "
				}
				line := fmt.Sprintf("%s#%02d  %s  (%s · %s)",
					prefix,
					i+1,
					snap.CreatedAt.Local().Format("2006-01-02 15:04:05"),
					utils.FormatBytes(snap.Size),
					snap.SHA256,
				)
				line = common.PadString(ansi.Truncate(line, innerWidth, "…"), innerWidth)
				if i == h.selected {
					line = selectedRow.Render(line)
				}
				body = append(body, line)
			}
		}

	}
	body = append(body, dim.Render(strings.Repeat("─", innerWidth)))
	d.hintTop = len(body) + 1
	for _, row := range d.hints {
		body = append(body, ansi.Truncate(common.FormatInlineHintRow(row), innerWidth, ""))
	}
	panel := common.RenderBorderedPanel(title, strings.Join(body, "\n"), panelWidth, common.TokyoBlue(), common.TokyoForeground())
	d.Dialog = dialog.New(panel, width, max(1, height-1)).Below(subHeaderHeight + 1).Clipped()
	return d
}
