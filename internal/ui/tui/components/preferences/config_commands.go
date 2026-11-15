package preferences

import (
	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
)

func FetchMihomoConfig(svc *service.ConfigService, client *api.Client) tea.Cmd {
	return messages.CoreRequest(client, func(_ *api.Client) tea.Msg {
		return messages.MihomoConfigMsg{Config: svc.ReadMihomoConfig(), FromFile: true}
	})
}

func SaveMihomoConfigField(svc *service.ConfigService, client *api.Client, key string, value interface{}) tea.Cmd {
	// A completed file write must refresh the resolved endpoint even when reload fails.
	return func() tea.Msg {
		result := svc.SaveMihomoConfigField(client, key, value)
		return messages.MihomoConfigSavedMsg{WriteOK: result.WriteOK, Err: result.Err}
	}
}
