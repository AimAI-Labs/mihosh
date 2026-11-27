package tui

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/guide"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/logs"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/require"
)

func TestOverlaysClipCleanlyOnVeryNarrowTerminals(t *testing.T) {
	i18n.Init()
	const width, height = 18, 10
	base := strings.Repeat("x\n", height-1) + "x"

	g := guide.State{}.Activate("http://127.0.0.1:9090", "sec***", "connection refused")
	guideOut := g.Render(width, height)
	require.NotEmpty(t, guideOut)
	for _, line := range strings.Split(guideOut, "\n") {
		require.LessOrEqual(t, lipgloss.Width(line), width)
	}

	logOut := logs.OverlayLogDetailPopup(base, &model.LogEntry{Payload: "very long log payload message"}, nil, nil, false, width, height, 0)
	require.NotEmpty(t, logOut)
	for _, line := range strings.Split(logOut, "\n") {
		require.LessOrEqual(t, lipgloss.Width(line), width)
	}

	errOut := overlayErrorPopup(base, width, height, "very long error description text")
	require.NotEmpty(t, errOut)
	for _, line := range strings.Split(errOut, "\n") {
		require.LessOrEqual(t, lipgloss.Width(line), width)
	}

	updOut := overlayUpdateDialog(base, width, height, &model.UpdateInfo{
		CurrentVersion: "v2.0.0",
		Version:        "v2.1.0",
		ReleaseURL:     "https://github.com/AimAI-Labs/mihosh/releases/tag/v2.1.0",
		DownloadURL:    "https://github.com/AimAI-Labs/mihosh/releases/download/v2.1.0/mihosh-linux-amd64.tar.gz",
	}, false, nil)
	require.NotEmpty(t, updOut)
	for _, line := range strings.Split(updOut, "\n") {
		require.LessOrEqual(t, lipgloss.Width(line), width)
	}
}
