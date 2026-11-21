package preferences

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type syncModalMode int

const (
	syncModeMenu syncModalMode = iota
	syncModeExportPath
	syncModeImportPath
	syncModePGConfig
)

const syncMenuItemCount = 6

// SyncModalState holds page-local state for Settings -> Backup & Cloud Sync modal.
type SyncModalState struct {
	Open       bool
	Mode       syncModalMode
	Cursor     int
	Busy       bool
	StatusText string
	StatusErr  bool

	// Single-line path input (export/import)
	InputValue  string
	InputCursor int

	// PG config form (DSN + ProfileKey)
	PGField    int // 0 = DSN, 1 = ProfileKey
	DSNValue   string
	KeyValue   string
	FormCursor int
}

func (s State) syncService() *service.SyncService {
	if s.syncSvc != nil {
		return s.syncSvc
	}
	return service.NewSyncService()
}

// SetSyncService injects a custom SyncService (primarily for unit tests).
func (s State) SetSyncService(svc *service.SyncService) State {
	s.syncSvc = svc
	return s
}

// OpenSyncModal opens the Backup & Cloud Sync modal on the Mihosh settings tab.
func (s State) OpenSyncModal() State {
	s.syncModal = SyncModalState{
		Open: true,
		Mode: syncModeMenu,
	}
	return s
}

func (s State) handleSyncModalKey(msg tea.KeyMsg) (State, tea.Cmd) {
	if s.syncModal.Busy {
		return s, nil
	}
	m := &s.syncModal

	switch m.Mode {
	case syncModeMenu:
		return s.handleSyncMenuKey(msg)
	case syncModeExportPath, syncModeImportPath:
		return s.handleSyncPathKey(msg)
	case syncModePGConfig:
		return s.handleSyncPGConfigKey(msg)
	}
	return s, nil
}

func (s State) handleSyncMenuKey(msg tea.KeyMsg) (State, tea.Cmd) {
	m := &s.syncModal
	switch {
	case key.Matches(msg, common.Keys.Escape), msg.String() == "q":
		s.syncModal = SyncModalState{}
		return s, nil
	case key.Matches(msg, common.Keys.Up):
		if m.Cursor > 0 {
			m.Cursor--
		}
		return s, nil
	case key.Matches(msg, common.Keys.Down):
		if m.Cursor < syncMenuItemCount-1 {
			m.Cursor++
		}
		return s, nil
	case key.Matches(msg, common.Keys.Enter):
		m.StatusText = ""
		m.StatusErr = false
		switch m.Cursor {
		case 0: // Export local archive
			defPath, _ := s.syncService().DefaultArchivePath()
			m.Mode = syncModeExportPath
			m.InputValue = defPath
			m.InputCursor = len([]rune(defPath))
			return s, nil
		case 1: // Import local archive
			m.Mode = syncModeImportPath
			m.InputValue = ""
			m.InputCursor = 0
			return s, nil
		case 2: // PG Push
			m.Busy = true
			m.StatusText = i18n.T("settings.sync.running_push")
			return s, runSyncPushCmd(s.syncService())
		case 3: // PG Pull
			m.Busy = true
			m.StatusText = i18n.T("settings.sync.running_pull")
			return s, runSyncPullCmd(s.syncService())
		case 4: // PG Status
			m.Busy = true
			m.StatusText = i18n.T("settings.sync.running_status")
			return s, runSyncStatusCmd(s.syncService())
		case 5: // Configure PG DSN & Key
			dsn, profileKey := s.syncService().ResolveSettings("", "")
			m.Mode = syncModePGConfig
			m.PGField = 0
			m.DSNValue = dsn
			m.KeyValue = profileKey
			m.FormCursor = len([]rune(dsn))
			return s, nil
		}
	}
	return s, nil
}

func (s State) handleSyncPathKey(msg tea.KeyMsg) (State, tea.Cmd) {
	m := &s.syncModal
	switch {
	case key.Matches(msg, common.Keys.Escape):
		m.Mode = syncModeMenu
		return s, nil
	case key.Matches(msg, common.Keys.Enter):
		p := strings.TrimSpace(m.InputValue)
		if m.Mode == syncModeImportPath && p == "" {
			m.StatusText = i18n.T("settings.sync.path_required")
			m.StatusErr = true
			return s, nil
		}
		m.Busy = true
		m.StatusErr = false
		if m.Mode == syncModeExportPath {
			m.StatusText = i18n.T("settings.sync.running_export")
			return s, runSyncExportCmd(s.syncService(), p)
		}
		m.StatusText = i18n.T("settings.sync.running_import")
		return s, runSyncImportCmd(s.syncService(), p)
	default:
		editTextInput(&m.InputValue, &m.InputCursor, msg)
		return s, nil
	}
}

func (s State) handleSyncPGConfigKey(msg tea.KeyMsg) (State, tea.Cmd) {
	m := &s.syncModal
	switch {
	case key.Matches(msg, common.Keys.Escape):
		m.Mode = syncModeMenu
		return s, nil
	case msg.String() == "tab", msg.String() == "shift+tab", key.Matches(msg, common.Keys.Up), key.Matches(msg, common.Keys.Down):
		if m.PGField == 0 {
			m.PGField = 1
			m.FormCursor = len([]rune(m.KeyValue))
		} else {
			m.PGField = 0
			m.FormCursor = len([]rune(m.DSNValue))
		}
		return s, nil
	case key.Matches(msg, common.Keys.Enter):
		if err := s.syncService().SaveSettings(m.DSNValue, m.KeyValue); err != nil {
			m.StatusText = err.Error()
			m.StatusErr = true
			return s, nil
		}
		if newCfg, err := service.Load(); err == nil {
			s.Config = newCfg
		}
		m.Mode = syncModeMenu
		m.StatusText = i18n.T("settings.sync.saved_pg")
		m.StatusErr = false
		return s, noticeCmd(i18n.T("settings.sync.saved_pg"), messages.NoticeSuccess)
	default:
		if m.PGField == 0 {
			editTextInput(&m.DSNValue, &m.FormCursor, msg)
		} else {
			editTextInput(&m.KeyValue, &m.FormCursor, msg)
		}
		return s, nil
	}
}

func editTextInput(val *string, cursor *int, msg tea.KeyMsg) {
	runes := []rune(*val)
	if *cursor < 0 {
		*cursor = 0
	}
	if *cursor > len(runes) {
		*cursor = len(runes)
	}
	switch {
	case msg.String() == "left":
		if *cursor > 0 {
			*cursor--
		}
	case msg.String() == "right":
		if *cursor < len(runes) {
			*cursor++
		}
	case key.Matches(msg, common.Keys.Home):
		*cursor = 0
	case key.Matches(msg, common.Keys.End):
		*cursor = len(runes)
	case key.Matches(msg, common.Keys.Backspace):
		if *cursor > 0 {
			*val = string(append(runes[:*cursor-1], runes[*cursor:]...))
			*cursor--
		}
	case key.Matches(msg, common.Keys.Delete):
		if *cursor < len(runes) {
			*val = string(append(runes[:*cursor], runes[*cursor+1:]...))
		}
	default:
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
			var in []rune
			if msg.Type == tea.KeySpace {
				in = []rune{' '}
			} else if len(msg.Runes) > 0 {
				in = msg.Runes
			} else {
				in = []rune(msg.String())
			}
			next := make([]rune, 0, len(runes)+len(in))
			next = append(next, runes[:*cursor]...)
			next = append(next, in...)
			next = append(next, runes[*cursor:]...)
			*val = string(next)
			*cursor += len(in)
		}
	}
}

func (s State) handleSyncModalScroll(up bool) State {
	if s.syncModal.Busy || s.syncModal.Mode != syncModeMenu {
		return s
	}
	if up && s.syncModal.Cursor > 0 {
		s.syncModal.Cursor--
	} else if !up && s.syncModal.Cursor < syncMenuItemCount-1 {
		s.syncModal.Cursor++
	}
	return s
}

func (s State) handleSyncModalClick(pageX, pageY, pageWidth, pageHeight int) (State, tea.Cmd) {
	if s.syncModal.Busy {
		return s, nil
	}
	modal := buildSyncModal(s.ToPageState(s.Config, nil), pageWidth, pageHeight)
	left, top, right, bottom := modal.Bounds()
	if pageY < top || pageY >= bottom || pageX < left || pageX >= right {
		s.syncModal = SyncModalState{}
		return s, nil
	}
	x, y := pageX-left-2, pageY-top
	if x < 0 || x >= modal.innerWidth {
		return s, nil
	}
	if row := y - modal.hintTop; row >= 0 && row < len(modal.hints) {
		if hint, ok := common.InlineHintAt(modal.hints[row], x, modal.innerWidth); ok {
			key, _ := hint.KeyMsg()
			return s.handleSyncModalKey(key)
		}
	}
	row := y - modal.bodyTop
	m := &s.syncModal
	switch m.Mode {
	case syncModeMenu:
		if row >= 0 && row < modal.menuCount {
			m.Cursor = modal.menuStart + row
			return s.handleSyncModalKey(tea.KeyMsg{Type: tea.KeyEnter})
		}
	case syncModeExportPath, syncModeImportPath:
		if row == 1 {
			m.InputCursor = modalInputCursor(m.InputValue, x-1, m.InputCursor)
		}
	case syncModePGConfig:
		if row >= 0 && row < 4 {
			field := row / 2
			value := m.DSNValue
			if field == 1 {
				value = m.KeyValue
			}
			cursor := -1
			if field == m.PGField {
				cursor = m.FormCursor
			}
			m.PGField, m.FormCursor = field, len([]rune(value))
			if row%2 == 1 {
				m.FormCursor = modalInputCursor(value, x-1, cursor)
			}
		}
	}
	return s, nil
}

func modalInputCursor(value string, column, current int) int {
	runes := []rune(value)
	for i, r := range runes {
		if i == current {
			column--
		}
		column -= common.DisplayWidth(string(r))
		if column < 0 {
			return i
		}
	}
	return len(runes)
}

// CompleteSync applies the async completion of a backup/sync action.
func (s State) CompleteSync(msg messages.SyncDoneMsg) State {
	if msg.Reloaded {
		if loaded, err := service.Load(); err == nil {
			s.Config = loaded
		}
	}
	if !s.syncModal.Open {
		return s
	}
	s.syncModal.Busy = false
	s.syncModal.Mode = syncModeMenu
	if msg.Err != nil {
		s.syncModal.StatusText = msg.Err.Error()
		s.syncModal.StatusErr = true
	} else {
		s.syncModal.StatusText = msg.Summary
		s.syncModal.StatusErr = false
	}
	return s
}

func runSyncExportCmd(svc *service.SyncService, targetPath string) tea.Cmd {
	return func() tea.Msg {
		res, err := svc.ExportArchive(targetPath, true)
		if err != nil {
			return messages.SyncDoneMsg{Action: "export", Err: err}
		}
		summary := i18n.Tf("settings.sync.export_done", res.Path, res.SubsCount, res.RedactedCount)
		return messages.SyncDoneMsg{Action: "export", Summary: summary}
	}
}

func runSyncImportCmd(svc *service.SyncService, sourcePath string) tea.Cmd {
	return func() tea.Msg {
		res, err := svc.ImportArchive(sourcePath)
		if err != nil {
			return messages.SyncDoneMsg{Action: "import", Err: err}
		}
		summary := i18n.Tf("settings.sync.import_done", res.SubsCount, res.PreservedSecrets)
		return messages.SyncDoneMsg{Action: "import", Summary: summary, Reloaded: true}
	}
}

func runSyncPushCmd(svc *service.SyncService) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		res, err := svc.Push(ctx, "", "", true)
		if err != nil {
			return messages.SyncDoneMsg{Action: "push", Err: err}
		}
		summary := i18n.Tf("settings.sync.push_done", res.ProfileKey, res.SubsCount, res.RedactedCount)
		return messages.SyncDoneMsg{Action: "push", Summary: summary, Reloaded: true}
	}
}

func runSyncPullCmd(svc *service.SyncService) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		res, err := svc.Pull(ctx, "", "")
		if err != nil {
			return messages.SyncDoneMsg{Action: "pull", Err: err}
		}
		summary := i18n.Tf("settings.sync.pull_done", res.ProfileKey, res.SubsCount, res.PreservedSecrets)
		return messages.SyncDoneMsg{Action: "pull", Summary: summary, Reloaded: true}
	}
}

func runSyncStatusCmd(svc *service.SyncService) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		res, err := svc.Status(ctx, "", "")
		if err != nil {
			return messages.SyncDoneMsg{Action: "status", Err: err}
		}
		var summary string
		if res.State == "remote_empty" {
			summary = i18n.Tf("settings.sync.status_empty", res.ProfileKey)
		} else {
			summary = fmt.Sprintf("%s: %s (%s @ %s)",
				res.ProfileKey, res.State, res.RemoteDevice, res.RemoteUpdated.Local().Format("2006-01-02 15:04"))
		}
		return messages.SyncDoneMsg{Action: "status", Summary: summary}
	}
}

func formatSyncSummaryLine(cfg *service.Config, maxW int) string {
	dsn := ""
	key := "default"
	last := ""
	if cfg != nil {
		dsn = service.RedactDSN(cfg.Sync.DSN)
		if strings.TrimSpace(cfg.Sync.ProfileKey) != "" {
			key = cfg.Sync.ProfileKey
		}
		last = cfg.Sync.LastSyncAt
	}
	if dsn == "" {
		dsn = i18n.T("settings.sync.pg_unconfigured")
	}
	line := i18n.Tf("settings.sync.pg_summary", key, dsn)
	if last != "" {
		line += " · " + last
	}
	return common.TruncateDisplay(line, maxW)
}
