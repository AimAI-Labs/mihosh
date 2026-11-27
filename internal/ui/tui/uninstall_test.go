package tui

import (
	"errors"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func TestUninstallDoneMsgQuitsWithSummaryOrPublishesError(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	t.Cleanup(func() { i18n.SetLanguageOverride("auto") })

	m := newTestModel(nil, "http://example.com", 1000)

	// Failure keeps TUI open and publishes an error notice.
	errModel, cmd, handled := m.applyConfigEvent(messages.UninstallDoneMsg{Err: errors.New("permission denied")})
	require.True(t, handled)
	require.Nil(t, cmd)
	require.Equal(t, messages.NoticeError, errModel.notice.Message.Level)
	require.Contains(t, errModel.notice.Message.Detail, "permission denied")
	require.Empty(t, errModel.ExitNotice())

	// Success sets ExitNotice and triggers tea.Quit.
	okModel, cmd, handled := m.applyConfigEvent(messages.UninstallDoneMsg{
		Result: service.UninstallResult{
			ConfigDir:       "/tmp/.mihosh",
			ConfigPreserved: true,
			ExecutablePath:  "/usr/local/bin/mihosh",
			BinaryRemoved:   true,
		},
	})
	require.True(t, handled)
	require.NotNil(t, cmd)
	_, isQuit := cmd().(tea.QuitMsg)
	require.True(t, isQuit)
	require.Contains(t, okModel.ExitNotice(), "/tmp/.mihosh")
	require.Contains(t, okModel.ExitNotice(), "/usr/local/bin/mihosh")
}
