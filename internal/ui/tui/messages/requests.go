package messages

import (
	"github.com/AimAI-Labs/mihosh/internal/app/service"
	tea "github.com/charmbracelet/bubbletea"
)

// CoreRequest binds every step and the completion (including errors) to one target.
func CoreRequest(client *service.Client, request func(*service.Client) tea.Msg) tea.Cmd {
	if client == nil {
		return func() tea.Msg { return request(nil) }
	}
	snapshot := client.Snapshot()
	return func() tea.Msg {
		return CoreHTTPMsg{Generation: snapshot.Generation(), Payload: request(snapshot)}
	}
}
