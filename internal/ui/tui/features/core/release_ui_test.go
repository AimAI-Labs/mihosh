package core

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestReleaseColumnsStayAlignedWhenCurrentVersionScrollsAway(t *testing.T) {
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	for _, language := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(language)
		s := releaseFixture(t)
		s.Status = service.CoreStatus{Mode: "managed", Version: s.Release.Version()}
		first := s.releaseColumns(0, 4, 70)
		later := s.releaseColumns(1, 5, 70)
		if first != later {
			t.Fatalf("%s: columns shifted after current version scrolled away: %+v %+v", language, first, later)
		}
		current := s.releaseRow(s.Release, "› ", first, 70)
		stable, _ := s.releaseAt(1)
		row := s.releaseRow(stable, "  ", later, 70)
		for _, token := range []string{"[", "2026-10-", "12.0 MiB"} {
			left := lipgloss.Width(current[:strings.Index(current, token)])
			right := lipgloss.Width(row[:strings.Index(row, token)])
			if left != right {
				t.Fatalf("%s: %q misaligned: %q / %q", language, token, current, row)
			}
		}
	}
}

func TestReleaseHelpKeepsEnterAndScrollbarFollowsViewport(t *testing.T) {
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	for _, language := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(language)
		for _, width := range []int{40, 80, 120} {
			s := releaseFixture(t)
			lines := strings.Split(ansi.Strip(s.Render(width, 16)), "\n")
			if !strings.Contains(lines[15], "Enter "+i18n.T("core.inline.download")) {
				t.Fatalf("%s %d: download hint hidden: %q", language, width, lines[15])
			}
			page := s.layout(width, 12)
			rows := strings.Split(ansi.Strip(page.content), "\n")
			first := page.releases[0].y
			if !strings.Contains(rows[first], common.SymbolScrollbarThumb) || !strings.Contains(rows[first+page.listRows-1], common.SymbolScrollbarTrack) {
				t.Fatal("scrollbar does not show the first viewport")
			}
			s = s.selectRelease(s.releaseCount() - 1)
			page = s.layout(width, 12)
			rows = strings.Split(ansi.Strip(page.content), "\n")
			if !strings.Contains(rows[first], common.SymbolScrollbarTrack) || !strings.Contains(rows[first+page.listRows-1], common.SymbolScrollbarThumb) {
				t.Fatal("scrollbar did not follow the last selection")
			}
			for _, row := range rows {
				if lipgloss.Width(row) > width {
					t.Fatal("scrollbar overflowed the page")
				}
			}
		}
	}
}
