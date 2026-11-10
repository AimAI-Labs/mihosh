package messages

import (
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	tea "github.com/charmbracelet/bubbletea"
)

// CoreRequest binds every step and the completion (including errors) to one target.
func CoreRequest(client *api.Client, request func(*api.Client) tea.Msg) tea.Cmd {
	if client == nil {
		return func() tea.Msg { return request(nil) }
	}
	snapshot := client.Snapshot()
	return func() tea.Msg {
		return CoreHTTPMsg{Generation: snapshot.Generation(), Payload: request(snapshot)}
	}
}
