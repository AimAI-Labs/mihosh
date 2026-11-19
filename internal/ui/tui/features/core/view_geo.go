package core

import (
	"path/filepath"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (s State) refreshGeoData() tea.Cmd {
	if s.svc == nil {
		return nil
	}
	return func() tea.Msg {
		files, err := s.svc.GeoDataFiles()
		return messages.CoreGeoDataMsg{Files: files, Err: err}
	}
}

func (s State) geoOverview(width int) string {
	rows := []string{wrap(i18n.T("core.geo.description"), width-4)}
	if s.Status.Mode != "managed" {
		rows = append(rows, "", wrap(i18n.T("core.geo.managed_only"), width-4))
	} else {
		if path, err := config.ManagedMihomoConfigPath(); err == nil {
			rows = append(rows, "", wrap(filepath.Dir(path), width-4))
		}
		rows = append(rows, "", wrap(i18n.T("core.geo.restart_hint"), width-4))
		if s.geoErr != nil {
			rows = append(rows, "", lipgloss.NewStyle().Foreground(theme.Resolve(theme.Current().Danger)).Render(wrap(s.geoErr.Error(), width-4)))
		}
	}
	return panel(i18n.T("core.tab.geo"), strings.Join(rows, "\n"), width)
}

func (s State) geoPanel(width int) (string, []actionHit) {
	t := theme.Current()
	muted := lipgloss.NewStyle().Foreground(theme.Resolve(t.Muted))
	selected := lipgloss.NewStyle().Foreground(theme.Resolve(t.Primary)).Background(theme.Resolve(t.Selected)).Bold(true)
	var rows []string
	var hits []actionHit
	for i, file := range s.geoFiles {
		prefix := "  "
		if s.tab == 3 && i == s.selected {
			prefix = "› "
		}
		label := ansi.Truncate(prefix+file.Name, max(1, width-4), "…")
		if s.tab == 3 && i == s.selected {
			label = selected.Render(common.PadString(label, max(1, width-4)))
		}
		detail := i18n.T("core.geo.missing")
		if file.Exists {
			detail = utils.FormatBytes(file.Size) + " · " + file.UpdatedAt.Local().Format("2006-01-02 15:04")
		}
		if !file.Updatable {
			detail += "\n" + i18n.T("core.geo.custom")
		}
		detail = "  " + strings.ReplaceAll(wrap(detail, width-6), "\n", "\n  ")
		hits = append(hits, actionHit{x: 1, y: len(rows) + 1, width: width - 2, height: 1 + lipgloss.Height(detail), index: i})
		rows = append(rows, label)
		rows = append(rows, strings.Split(muted.Render(detail), "\n")...)
	}
	if len(rows) == 0 {
		rows = append(rows, wrap(i18n.T("core.geo.no_files"), width-4))
	}
	return panel(i18n.T("core.geo.files"), strings.Join(rows, "\n"), width), hits
}
