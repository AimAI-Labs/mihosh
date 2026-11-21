package sub

import (
	"errors"
	"strings"
	"unicode"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/dialog"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type fileMode int

const (
	fileClosed fileMode = iota
	fileMenu
	fileDiff
)

var fileOptions = []string{"sub.file.source", "sub.file.activated", "sub.file.diff"}

var diffOptions = []struct {
	key  string
	flag profile.FileDiffOptions
}{
	{"sub.file.diff_indent", profile.DiffIgnoreIndent},
	{"sub.file.diff_whitespace", profile.DiffIgnoreWhitespace},
	{"sub.file.diff_format", profile.DiffIgnoreFormatCharacters},
	{"sub.file.diff_punctuation", profile.DiffIgnorePunctuation},
}

type fileView struct {
	mode                  fileMode
	uid                   string
	request               uint64
	selected, top, column int
	loading               bool
	diff                  []profile.FileDiffRow
	err                   error
	optionsOpen           bool
	option                int
	ignore                profile.FileDiffOptions
}

func (s State) openFileMenu() State {
	if s.selected < 0 || s.selected >= len(s.filteredIdx) {
		return s
	}
	uid := s.subs[s.filteredIdx[s.selected]].UID
	ignore := service.LoadFileDiffOptions()
	s.file = fileView{mode: fileMenu, uid: uid, request: s.file.request + 1, ignore: ignore}
	return s
}

func (s State) loadFileView() (State, tea.Cmd) {
	s.file.err = nil
	if s.file.selected < len(fileOptions)-1 {
		s.file.request++
		file := s.file
		return s, tea.Exec(&vimPreview{uid: file.uid, activated: file.selected == 1}, func(err error) tea.Msg {
			return messages.SubFileViewerClosedMsg{Request: file.request, Err: err}
		})
	}
	s.file.mode = fileDiff
	return s.recomputeDiff()
}

func (s State) recomputeDiff() (State, tea.Cmd) {
	s.file.loading = true
	s.file.top = 0
	s.file.request++
	file := s.file
	return s, func() tea.Msg {
		diff, err := service.DiffRawWithPreview(file.uid, file.ignore)
		return messages.SubFileLoadedMsg{Request: file.request, Diff: diff, Err: err}
	}
}

func terminalFileText(data []byte) string {
	text := strings.ReplaceAll(ansi.Strip(string(data)), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\t", "    ")
	return strings.Map(func(r rune) rune {
		if r == '\n' || (!unicode.IsControl(r) && !unicode.Is(unicode.Cf, r)) {
			return r
		}
		return -1
	}, text)
}

func (f fileView) rowBudget(width, height int) int {
	rows := max(1, height-7)
	if f.mode == fileDiff && width < 100 {
		rows = max(1, (rows-3)/2)
	}
	return rows
}

func (f *fileView) scroll(delta, width, height int) {
	if f.optionsOpen {
		f.option = min(max(0, f.option+delta), len(diffOptions)-1)
		return
	}
	if f.mode == fileMenu {
		f.selected = min(max(0, f.selected+delta), len(fileOptions)-1)
		return
	}
	rows := len(f.diff)
	f.top = min(max(0, f.top+delta), max(0, rows-f.rowBudget(width, height)))
}

func (s State) handleFileKey(msg tea.KeyMsg) (State, tea.Cmd) {
	if s.file.optionsOpen {
		switch msg.String() {
		case "esc", "o":
			s.file.optionsOpen = false
		case "up", "k":
			s.file.scroll(-1, s.pageWidth, s.pageHeight)
		case "down", "j":
			s.file.scroll(1, s.pageWidth, s.pageHeight)
		case "enter", " ":
			return s.toggleDiffOption(s.file.option)
		}
		return s, nil
	}
	switch msg.String() {
	case "esc":
		mode := fileMenu
		if s.file.mode == fileMenu {
			mode = fileClosed
		}
		s.file = fileView{mode: mode, uid: s.file.uid, selected: s.file.selected, request: s.file.request + 1, ignore: s.file.ignore}
	case "enter":
		if s.file.mode == fileMenu {
			return s.loadFileView()
		}
	case "o":
		s.file.optionsOpen = s.file.mode == fileDiff
	case "up", "k":
		s.file.scroll(-1, s.pageWidth, s.pageHeight)
	case "down", "j":
		s.file.scroll(1, s.pageWidth, s.pageHeight)
	case "pgup":
		s.file.scroll(-s.file.rowBudget(s.pageWidth, s.pageHeight), s.pageWidth, s.pageHeight)
	case "pgdown":
		s.file.scroll(s.file.rowBudget(s.pageWidth, s.pageHeight), s.pageWidth, s.pageHeight)
	case "left":
		s.file.column = max(0, s.file.column-8)
	case "right":
		s.file.column += 8
	}
	return s, nil
}

func (s State) clickFile(msg messages.PageMouseClickMsg) (State, tea.Cmd) {
	left, top, right, bottom := s.file.dialog(msg.Width, msg.Height).Bounds()
	if msg.X < left || msg.X >= right || msg.Y < top || msg.Y >= bottom {
		if s.file.optionsOpen {
			s.file.optionsOpen = false
			return s, nil
		}
		s.file = fileView{request: s.file.request + 1}
		return s, nil
	}
	idx := msg.Y - top - 1
	if s.file.optionsOpen && idx >= 0 && idx < len(diffOptions) {
		return s.toggleDiffOption(idx)
	}
	if s.file.mode == fileMenu && idx >= 0 && idx < len(fileOptions) {
		s.file.selected = idx
		return s.loadFileView()
	}
	if s.file.mode == fileDiff && !s.file.optionsOpen && msg.Y == bottom-2 &&
		msg.X >= left+2 && msg.X < left+2+lipgloss.Width(diffOptionHint()) {
		s.file.optionsOpen = true
	}
	return s, nil
}

func (s State) toggleDiffOption(index int) (State, tea.Cmd) {
	s.file.option = index
	ignore := s.file.ignore ^ diffOptions[index].flag
	if err := service.SaveFileDiffOptions(ignore); err != nil {
		s.file.err = err
		return s, nil
	}
	s.file.ignore, s.file.err = ignore, nil
	return s.recomputeDiff()
}

func diffOptionHint() string {
	return "o " + i18n.T("sub.file.diff_options")
}

func (f fileView) dialog(width, height int) dialog.Dialog {
	w := max(24, width-4)
	title := i18n.T("sub.file.menu")
	hint := i18n.T("sub.file.hint")
	var body string
	if f.optionsOpen {
		w = min(52, w)
		title = i18n.T("sub.file.diff_options")
		hint = i18n.T("sub.file.diff_options_hint")
		rows := make([]string, 0, len(diffOptions))
		for i, option := range diffOptions {
			check := "[ ] "
			if f.ignore&option.flag != 0 {
				check = "[x] "
			}
			row := check + i18n.T(option.key)
			if i == f.option {
				row = common.SelectedStyle().Render(row)
			}
			rows = append(rows, row)
		}
		body = strings.Join(rows, "\n")
		if f.err != nil && !f.loading {
			body += "\n" + ansi.Wrap(terminalFileText([]byte(f.err.Error())), w-4, "")
		}
	} else if f.mode == fileMenu {
		w = min(48, w)
		items := make([]string, len(fileOptions))
		for i, key := range fileOptions {
			items[i] = "  " + i18n.T(key)
			if i == f.selected {
				items[i] = common.SelectedStyle().Render("› " + i18n.T(key))
			}
		}
		body = strings.Join(items, "\n")
		if f.err != nil {
			message := terminalFileText([]byte(f.err.Error()))
			if errors.Is(f.err, profile.ErrRawNotFound) {
				message = i18n.T("sub.err.raw_not_found")
			}
			body += "\n" + ansi.Wrap(message, w-4, "")
		}
	} else {
		title = i18n.T(fileOptions[f.selected])
		body = f.renderFile(w-4, width, height)
		hint = diffOptionHint() + " · " + hint
	}
	body += "\n" + common.MutedStyle().Render(hint)
	content := common.RenderBorderedPanel(title, body, w, common.Primary(), common.Bright())
	return dialog.New(content, width, height).Clipped()
}

func (f fileView) renderFile(inner, pageWidth, height int) string {
	if f.loading {
		return i18n.T("sub.file.loading")
	}
	if f.err != nil && len(f.diff) == 0 {
		if errors.Is(f.err, profile.ErrRawNotFound) {
			return i18n.T("sub.err.raw_not_found")
		}
		return terminalFileText([]byte(f.err.Error()))
	}
	budget := f.rowBudget(pageWidth, height)
	if len(f.diff) == 0 {
		return i18n.T("sub.file.same")
	}
	from := min(f.top, max(0, len(f.diff)-budget))
	rows := f.diff[from:min(len(f.diff), from+budget)]
	if pageWidth >= 100 {
		left := inner * 43 / 100
		return lipgloss.JoinHorizontal(lipgloss.Top, renderDiffPane(rows, true, f.column, left), " ", renderDiffPane(rows, false, f.column, inner-left-1))
	}
	return renderDiffPane(rows, true, f.column, inner) + "\n" + renderDiffPane(rows, false, f.column, inner)
}

func renderDiffPane(rows []profile.FileDiffRow, before bool, column, width int) string {
	label, color := "sub.file.activated", common.Success()
	if before {
		label, color = "sub.file.source", common.Danger()
	}
	style := lipgloss.NewStyle().Foreground(color)
	lines := []string{common.TableHeaderStyle().Render(i18n.T(label))}
	for _, row := range rows {
		text := row.After
		if before {
			text = row.Before
		}
		text = common.PadString(ansi.Cut(terminalFileText([]byte(text)), column, column+width), width)
		if row.Changed {
			text = style.Render(text)
		}
		lines = append(lines, text)
	}
	return strings.Join(lines, "\n")
}
