package preferences

import (
	"context"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
)

func FetchMihomoConfig(svc *service.ConfigService, client *service.Client) tea.Cmd {
	return messages.CoreRequest(client, func(_ *service.Client) tea.Msg {
		cfg, target := svc.ReadMihomoConfig()
		return messages.MihomoConfigMsg{Config: cfg, Target: &target, FromFile: true}
	})
}

func SaveMihomoConfigField(svc *service.ConfigService, client *service.Client, key string, value interface{}) tea.Cmd {
	// A completed file write must refresh the resolved endpoint even when reload fails.
	return func() tea.Msg {
		result := svc.SaveMihomoConfigField(client, key, value)
		return messages.MihomoConfigSavedMsg{WriteOK: result.WriteOK, Err: result.Err}
	}
}

// FetchMihomoOverrides loads the five editable Mihomo overrides from merge.yaml.
func FetchMihomoOverrides(svc *service.ConfigService) tea.Cmd {
	if svc == nil {
		svc = service.NewConfigService()
	}
	return func() tea.Msg {
		readAt := time.Now()
		overrides, err := svc.ReadMihomoOverrides()
		return messages.MihomoOverridesMsg{Overrides: overrides, ReadAt: readAt, Err: err}
	}
}

// ApplyMihomoOverrides persists the five overrides to merge.yaml and applies them to the active core.
func ApplyMihomoOverrides(svc *service.ConfigService, coreSvc *service.CoreService, client *service.Client, overrides map[string]string) tea.Cmd {
	if svc == nil {
		svc = service.NewConfigService()
	}
	copied := cloneOverridesMap(overrides)
	return func() tea.Msg {
		result := svc.ApplyMihomoOverrides(context.Background(), coreSvc, client, copied)
		return messages.MihomoOverridesSavedMsg{
			Overrides:          result.Overrides,
			WriteOK:            result.WriteOK,
			Applied:            result.Applied,
			Restarted:          result.Restarted,
			NeedsRestart:       result.NeedsRestart,
			EndpointChanged:    result.EndpointChanged,
			Remote:             result.Remote,
			RemoteNotPersisted: result.RemoteNotPersisted,
			Restored:           result.Restored,
			Err:                result.Err,
		}
	}
}
