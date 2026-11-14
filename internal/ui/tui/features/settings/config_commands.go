package settings

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
