package components

import (
	"regexp"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/mattn/go-runewidth"
)

var ansiPattern = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]")

func TestDetailModalPanelsCloseRightBorderWithLongContent(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("zh-CN")
	conn := &model.Connection{
		ID:          strings.Repeat("conn-", 20),
		Rule:        "DomainSuffix",
		RulePayload: strings.Repeat("very-long-payload.", 12),
		Metadata: model.Metadata{
			Host:            strings.Repeat("zws5.web.telegram.org.", 8),
			SourceIP:        "172.18.0.6",
			SourcePort:      "51922",
			DestinationIP:   "108.160.165.147",
			DestinationPort: "443",
			Network:         "tcp",
			Type:            "HTTPS",
			ProcessPath:     strings.Repeat("C:/Program Files/Mihosh/", 5),
		},
		Chains: []string{
			"新加坡 1",
			strings.Repeat("proxy-chain-", 12),
		},
		Download: 37248572,
		Upload:   1399261,
	}
	ipInfo := &model.IPInfo{
		IP:         "108.160.165.147",
		Country:    "United States",
		RegionName: "Kansas",
		City:       "Wichita",
		ASN:        19679,
		ISP:        strings.Repeat("Dropbox Infrastructure ", 4),
		Timezone:   "America/Chicago",
		Latitude:   37.751,
		Longitude:  -97.822,
	}

	assertClosedPanelLines(t, RenderDetailModalLeft(conn, ipInfo, 64, 24, 0, true), 64)
	assertClosedPanelLines(t, RenderDetailModalRight(conn, 96, 24, 0, true), 96)
}

func TestConnectionDetailImmersiveDoesNotOverflowRequestedWidth(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("zh-CN")
	const width = 160

	conn := &model.Connection{
		ID:          strings.Repeat("conn-", 20),
		Rule:        "DomainSuffix",
		RulePayload: strings.Repeat("very-long-payload.", 12),
		Metadata: model.Metadata{
			Host:            strings.Repeat("zws5.web.telegram.org.", 8),
			SourceIP:        "172.18.0.6",
			SourcePort:      "51922",
			DestinationIP:   "108.160.165.147",
			DestinationPort: "443",
			Network:         "tcp",
			Type:            "HTTPS",
			ProcessPath:     strings.Repeat("C:/Program Files/Mihosh/", 5),
		},
		Chains: []string{
			"新加坡 1",
			strings.Repeat("proxy-chain-", 12),
		},
		Download: 37248572,
		Upload:   1399261,
	}

	view := RenderConnectionDetailImmersive(conn, nil, width, 24, 0, 0, 1)
	for i, raw := range strings.Split(view, "\n") {
		line := ansiPattern.ReplaceAllString(raw, "")
		if got := runewidth.StringWidth(line); got > width {
			t.Fatalf("line %d width = %d, want <= %d: %q", i, got, width, line)
		}
	}
}

func assertClosedPanelLines(t *testing.T, panel string, width int) {
	t.Helper()

	for i, raw := range strings.Split(panel, "\n") {
		line := ansiPattern.ReplaceAllString(raw, "")
		if line == "" {
			continue
		}

		first := []rune(line)[0]
		switch first {
		case '╭':
			assertPanelLine(t, i, line, width, '╮')
		case '│':
			assertPanelLine(t, i, line, width, '│')
		case '╰':
			assertPanelLine(t, i, line, width, '╯')
		}
	}
}

func assertPanelLine(t *testing.T, index int, line string, width int, wantLast rune) {
	t.Helper()

	if got := runewidth.StringWidth(line); got != width {
		t.Fatalf("line %d width = %d, want %d: %q", index, got, width, line)
	}

	runes := []rune(line)
	if got := runes[len(runes)-1]; got != wantLast {
		t.Fatalf("line %d ends with %q, want %q: %q", index, got, wantLast, line)
	}
}
