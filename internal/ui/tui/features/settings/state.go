package settings

import (
	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/preferences"
)

// Settings exposes only application preferences; Core owns the Mihomo editor.
type State = preferences.State

func NewState(svc *service.ConfigService, client *api.Client) State {
	return preferences.NewState(svc, client)
}

func RenderSettingsPage(state preferences.PageState, width, height int) string {
	return preferences.RenderSettingsPage(state, width, height)
}
