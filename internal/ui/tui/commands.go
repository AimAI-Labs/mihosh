package tui

import (
	"context"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	tea "github.com/charmbracelet/bubbletea"
)

// fetchMemory 获取当前内存使用量
func fetchMemory(client *api.Client) tea.Cmd {
	return messages.CoreRequest(client, func(client *api.Client) tea.Msg {
		mem, err := client.GetMemory()
		if err != nil {
			// 内存获取失败不报错，返回0
			return messages.MemoryWSMsg{Memory: 0}
		}
		return messages.MemoryWSMsg{Memory: mem.Inuse}
	})
}

// startWSStreams 启动WebSocket流
func startWSStreams(ctx context.Context, wsClient *api.WSClient, msgChan chan interface{}) tea.Cmd {
	return func() tea.Msg {
		if wsClient == nil {
			return nil
		}

		// 设置内存处理器
		wsClient.SetMemoryHandler(func(data api.MemoryData) {
			select {
			case msgChan <- messages.CoreWSMsg{Generation: data.Generation, Payload: messages.MemoryWSMsg{Memory: data.Inuse}}:
			default:
				// channel满了就丢弃
			}
		})

		// 设置流量处理器
		wsClient.SetTrafficHandler(func(data api.TrafficData) {
			select {
			case msgChan <- messages.CoreWSMsg{Generation: data.Generation, Payload: messages.TrafficWSMsg{Up: data.Up, Down: data.Down}}:
			default:
				// channel满了就丢弃
			}
		})

		// 设置连接处理器
		wsClient.SetConnectionsHandler(func(data api.ConnectionsData) {
			select {
			case msgChan <- messages.CoreWSMsg{Generation: data.Generation, Payload: messages.ConnectionsWSMsg{Data: data}}:
			default:
				// channel满了就丢弃
			}
		})

		// 设置日志处理器
		wsClient.SetLogsHandler(func(data api.LogData) {
			select {
			case msgChan <- messages.CoreWSMsg{Generation: data.Generation, Payload: messages.LogsWSMsg{LogType: data.Type, Payload: data.Payload}}:
			default:
				// channel满了就丢弃
			}
		})

		// 启动WebSocket连接
		if err := wsClient.Start(ctx); err != nil {
			return messages.ErrMsg{Err: err}
		}
		return nil
	}
}

// listenWSMessages 监听WebSocket消息
func listenWSMessages(ctx context.Context, msgChan chan interface{}) tea.Cmd {
	return func() tea.Msg {
		select {
		case <-ctx.Done():
			return nil
		case msg := <-msgChan:
			// 将interface{}转换为tea.Msg
			if teaMsg, ok := msg.(tea.Msg); ok {
				return teaMsg
			}
			return msg
		}
	}
}
