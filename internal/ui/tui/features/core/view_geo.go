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

func (s State) geoPanel(width int) (string, []actionHit) {
	t := theme.Current()
	muted := lipgloss.NewStyle().Foreground(theme.Resolve(t.Muted))
	selected := lipgloss.NewStyle().Foreground(theme.Resolve(t.Primary)).Background(theme.Resolve(t.Selected)).Bold(true)
	var rows []string
	var hits []actionHit
	if s.Status.Mode == "managed" {
		path := s.Status.ConfigPath
		if path == "" {
			path, _ = config.ManagedMihomoConfigPath()
		}
		if path != "" {
			directory := i18n.T("core.geo.directory") + ": " + filepath.Dir(path)
			rows = append(rows, strings.Split(muted.Render(wrap(directory, width-4)), "\n")...)
		}
	}
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
	if len(s.geoFiles) == 0 {
		key := "core.geo.no_files"
		if s.Status.Mode != "managed" {
			key = "core.geo.managed_only"
		}
		rows = append(rows, wrap(i18n.T(key), width-4))
	}
	if s.Status.Mode == "managed" && s.geoErr != nil {
		rows = append(rows, common.ErrorStyle().Render(wrap(s.geoErr.Error(), width-4)))
	}
	return panel(i18n.T("core.geo.files"), strings.Join(rows, "\n"), width), hits
}
