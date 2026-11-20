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
				s.history.kind = profile.SnapshotMerge
			} else if s.history.uid != "" {
				s.history.kind = profile.SnapshotRaw
			} else {
				return s, nil
			}
			s.history.request++
			s.history.loading = true
			s.history.selected = 0
			s.history.snapshots = nil
			s.history.err = nil
			return s, s.loadHistoryListCmd()
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

func (s State) clickHistory(msg messages.PageMouseClickMsg) (State, tea.Cmd) {
	d := s.history.dialog(msg.Width, msg.Height)
	left, top, right, bottom := d.Bounds()
	if msg.X < left || msg.X >= right || msg.Y < top || msg.Y >= bottom {
		s.history.mode = historyClosed
		return s, nil
	}
	if s.history.mode == historyList && len(s.history.snapshots) > 0 {
		// Row layout inside dialog: border(1) + tab row(1) + divider(1) = 3 lines before items
		row := msg.Y - top - 3
		if row >= 0 && row < len(s.history.snapshots) {
			s.history.selected = row
		}
	}
	return s, nil
}

func (h historyView) dialog(width, height int) dialog.Dialog {
	panelWidth := max(36, min(110, width-4))
	innerWidth := max(20, panelWidth-4)
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
			budget := max(4, height-9)
			top := min(h.top, max(0, len(h.diff)-budget))
			end := min(len(h.diff), top+budget)
			half := max(10, (innerWidth-3)/2)
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
		hints := common.FormatInlineHintRow([]common.InlineHelpHint{
			{Key: "↑↓", Desc: i18n.T("core.inline.scroll")},
			{Key: "r", Desc: i18n.T("sub.history.hint_rollback"), Tone: common.InlineHelpToneWarning},
			{Key: "Esc", Desc: i18n.T("sub.history.hint_back")},
		})
		body = append(body, dim.Render(strings.Repeat("─", innerWidth)), ansi.Truncate(hints, innerWidth, ""))
	} else {
		title = i18n.T("sub.history.title")
		rawTabLabel := " " + i18n.T("sub.history.tab_raw")
		if h.subName != "" {
			rawTabLabel += " (" + h.subName + ")"
		}
		rawTabLabel += " "
		mergeTabLabel := " " + i18n.T("sub.history.tab_merge") + " "

		var tabsLine string
		if h.kind == profile.SnapshotRaw {
			tabsLine = activeTab.Render(rawTabLabel) + "  " + inactiveTab.Render(mergeTabLabel)
		} else {
			tabsLine = inactiveTab.Render(rawTabLabel) + "  " + activeTab.Render(mergeTabLabel)
		}
		body = append(body, ansi.Truncate(tabsLine, innerWidth, "…"))
		body = append(body, dim.Render(strings.Repeat("─", innerWidth)))

		if h.loading {
			body = append(body, dim.Render(i18n.T("sub.history.loading")))
		} else if h.err != nil {
			body = append(body, errStyle.Render(h.err.Error()))
		} else if len(h.snapshots) == 0 {
			body = append(body, dim.Render(i18n.T("sub.history.empty")))
		} else {
			budget := max(3, height-10)
			start := 0
			if h.selected >= budget {
				start = h.selected - budget + 1
			}
			end := min(len(h.snapshots), start+budget)
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

		hints := common.FormatInlineHintRow([]common.InlineHelpHint{
			{Key: "Tab", Desc: i18n.T("sub.history.hint_tab")},
			{Key: "↵/d", Desc: i18n.T("sub.history.hint_diff"), Tone: common.InlineHelpToneInfo},
			{Key: "r", Desc: i18n.T("sub.history.hint_rollback"), Tone: common.InlineHelpToneWarning},
			{Key: "Esc", Desc: i18n.T("core.inline.cancel")},
		})
		body = append(body, dim.Render(strings.Repeat("─", innerWidth)), ansi.Truncate(hints, innerWidth, ""))
	}

	panel := common.RenderBorderedPanel(title, strings.Join(body, "\n"), panelWidth, common.TokyoBlue(), common.TokyoForeground())
	return dialog.New(panel, width, max(1, height-1)).Below(subHeaderHeight + 1).Clipped()
}
