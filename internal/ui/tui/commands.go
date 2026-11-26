package tui

import (
	"context"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"

	tea "github.com/charmbracelet/bubbletea"
)

// fetchMemory 获取当前内存使用量
func fetchMemory(client *service.Client) tea.Cmd {
	return messages.CoreRequest(client, func(client *service.Client) tea.Msg {
		mem, err := client.GetMemory()
		if err != nil {
			// 内存获取失败不报错，返回0
			return messages.MemoryWSMsg{Memory: 0}
		}
		return messages.MemoryWSMsg{Memory: mem.Inuse}
	})
}

// startWSStreams 启动WebSocket流
func startWSStreams(ctx context.Context, wsClient *service.WSClient, inbox *wsInbox) tea.Cmd {
	return func() tea.Msg {
		if wsClient == nil || inbox == nil {
			return nil
		}
		wsClient.SetMemoryHandler(func(data service.MemoryData) {
			inbox.publish(data.Generation, messages.MemoryWSMsg{Memory: data.Inuse})
		})
		wsClient.SetTrafficHandler(func(data service.TrafficData) {
			inbox.publish(data.Generation, messages.TrafficWSMsg{Up: data.Up, Down: data.Down})
		})
		wsClient.SetConnectionsHandler(func(data service.ConnectionsData) {
			inbox.publish(data.Generation, messages.ConnectionsWSMsg{Data: data})
		})
		wsClient.SetLogsHandler(func(data service.LogData) {
			inbox.publish(data.Generation, messages.LogsWSMsg{LogType: data.Type, Payload: data.Payload})
		})
		if err := wsClient.Start(ctx); err != nil {
			return messages.ErrMsg{Err: err}
		}
		return nil
	}
}
