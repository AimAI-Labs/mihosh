package core

import (
	"fmt"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var actions = []struct{ key, action string }{
	{"enter", "setup"}, {"v", "check"}, {"i", "install"}, {"m", "configure"}, {"s", "start"}, {"x", "stop"}, {"e", "network"}, {"t", "version"}, {"l", "import"}, {"a", "activate"}, {"b", "rollback"}, {"z", "restart"}, {"u", "external"}, {"p", "ports"}, {"g", "upgrade"}, {"o", "profile"},
	{"h", "service"}, {"y", "system-proxy"}, {"n", "tun"}, {"c", "recover"},
}

func actionOrigin(width int) (int, int) {
	if width >= 100 {
		return width*43/100 + 2, 2
	}
	return 2, 8
}
func (s State) Render(width, height int) string {
	t := theme.Current()
	heading := lipgloss.NewStyle().Bold(true).Foreground(t.Primary)
	muted := lipgloss.NewStyle().Foreground(t.Muted)
	mode := s.Status.Mode
	if mode == "" {
		mode = "external"
	}
	version := s.Status.Version
	if version == "" {
		version = i18n.T("core.not_installed")
	}
	running := i18n.T("core.stopped")
	if s.Status.Running {
		running = i18n.T("core.running")
		if !s.Status.HasNodes {
			running = i18n.T("core.no_nodes")
		}
	}
	info := []string{heading.Render(i18n.T("core.heading")), "", i18n.T("core.intro"),
		fmt.Sprintf("%s: %s   %s: %s", i18n.T("core.mode"), mode, i18n.T("core.version"), version), running,
		fmt.Sprintf("%s: %s (%s)", i18n.T("core.available"), s.Release.Tag, s.Release.Route), muted.Render(i18n.T("core.route_order")), ""}
	buttons := []string{}
	for _, a := range actions {
		buttons = append(buttons, "["+a.key+"] "+i18n.T("core.action."+a.action))
	}
	details := []string{"", muted.Render(i18n.T("core.subscription_hint")), fmt.Sprintf("%s: %s · %s: %t · %s: %t", i18n.T("core.runtime"), s.Status.Runtime, i18n.T("core.system_proxy"), s.Status.SystemProxy, "TUN", s.Status.TUN), fmt.Sprintf("%s: %t · %s: %t", i18n.T("core.autostart"), s.Status.AutoStart, i18n.T("core.recovery_pending"), s.Status.RecoveryPending)}
	if !s.Release.FetchedAt.IsZero() {
		info[5] += " " + s.Release.FetchedAt.Format(time.DateOnly)
	}
	if s.Busy {
		progress := s.Progress.Route
		if s.Progress.Downloaded > 0 {
			progress += fmt.Sprintf("  %.1f MiB", float64(s.Progress.Downloaded)/(1<<20))
			if s.Progress.Total > 0 {
				progress += fmt.Sprintf(" / %.1f MiB", float64(s.Progress.Total)/(1<<20))
			}
		}
		if s.Progress.BytesPerSecond > 0 {
			progress += fmt.Sprintf("  %.1f KiB/s", float64(s.Progress.BytesPerSecond)/1024)
		}
		details = append(details, heading.Render(i18n.T("core.busy")+" "+progress), muted.Render(i18n.T("core.cancel")))
	}
	if s.Err != nil {
		details = append(details, lipgloss.NewStyle().Foreground(t.Danger).Render(s.Err.Error()))
	}
	fit := func(lines []string, w int) string {
		result := make([]string, len(lines))
		for i, line := range lines {
			result[i] = "  " + ansi.Truncate(line, max(1, w-4), "…")
		}
		return strings.Join(result, "\n")
	}
	var full string
	if width >= 100 {
		left := width * 43 / 100
		rightLines := append([]string{heading.Render(i18n.T("core.actions")), ""}, buttons...)
		leftLines := append(info, details...)
		full = lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().Width(left).Render(fit(leftLines, left)), fit(rightLines, width-left))
	} else {
		full = fit(append(append(info, buttons...), details...), width)
	}
	if s.Editing {
		labels := []string{"core.proxy", "core.accelerators", "core.download_timeout"}
		if s.form == "service" {
			labels = []string{"core.service.action"}

		}
		if s.form == "profile" {
			labels = []string{"core.profile_name", "core.profile_source"}
		}
		if s.form == "version" {
			labels = []string{"cli.core.version"}
		}
		if s.form == "import" {
			labels = []string{"cli.core.file", "cli.core.version", "cli.core.sha256"}
		}
		if s.form == "external" {
			labels = []string{"core.controller", "core.secret", "core.mixed_port"}
		}
		if s.form == "ports" {
			labels = []string{"core.controller_port", "core.mixed_port"}
		}
		lines := []string{heading.Render(i18n.T("core.action." + s.form)), ""}
		for i, label := range labels {
			lines = append(lines, i18n.T(label), s.fields[i].View(), "")
		}
		lines = append(lines, muted.Render(i18n.T("core.form_help")))
		if s.Err != nil {
			lines = append(lines, lipgloss.NewStyle().Foreground(t.Danger).Render(s.Err.Error()))
		}
		full = fit(lines, width)
	}
	lines := strings.Split(full, "\n")
	offset := min(s.scroll, max(0, len(lines)-height))
	if s.Editing {
		offset = 0
	}
	lines = lines[offset:]
	if len(lines) > height {
		lines = lines[:max(0, height)]
	}
	return strings.Join(lines, "\n")
}
