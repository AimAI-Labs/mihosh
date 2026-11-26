package tui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/logs"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestWebSocketLogBurstKeepsNewestSnapshotsAndExposesDrops(t *testing.T) {
	i18n.Init()
	const total = 3 * common.LogsCap
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for i := 1; i <= total; i++ {
			switch r.URL.Path {
			case "/memory":
				err = conn.WriteJSON(api.MemoryData{Inuse: int64(i)})
			case "/traffic":
				err = conn.WriteJSON(api.TrafficData{Up: int64(i), Down: int64(i * 2)})
			case "/connections":
				err = conn.WriteJSON(service.ConnectionsData{DownloadTotal: int64(i)})
			case "/logs":
				err = conn.WriteJSON(api.LogData{Type: "info", Payload: fmt.Sprintf("entry-%d", i)})
			}
			if err != nil {
				return
			}
		}
		_, _, _ = conn.ReadMessage()
	}))
	defer server.Close()
	m := Model{
		currentPage: layout.PageLogs, chartData: model.NewChartData(10),
		logsState: logs.NewState(nil), wsClient: api.NewWSClient(server.URL, ""),
		wsInbox: newWSInbox(), wsCtx: t.Context(),
	}
	defer m.wsClient.Stop()
	require.Nil(t, startWSStreams(t.Context(), m.wsClient, m.wsInbox)())
	require.Eventually(t, func() bool {
		b := m.wsInbox
		b.mu.Lock()
		defer b.mu.Unlock()
		return !time.Now().Before(b.logDue) && b.pending.DroppedLogs == total-common.LogsCap &&
			b.pending.Memory != nil && b.pending.Memory.Memory == total &&
			b.pending.Traffic != nil && b.pending.Traffic.Up == total &&
			b.pending.Connections != nil && b.pending.Connections.Data.DownloadTotal == total
	}, 5*time.Second, 2*time.Millisecond)
	msg := listenWSMessages(t.Context(), m.wsInbox)()
	next, cmd := m.Update(msg)
	m = next.(Model)
	require.NotNil(t, cmd, "one listener should be re-armed after the whole batch")
	require.Equal(t, []int64{total}, m.chartData.MemoryHistory)
	require.Equal(t, []int64{total}, m.chartData.SpeedUpHistory)
	require.Equal(t, int64(total), m.connsState.Connections.DownloadTotal)
	state := m.logsState.ToPageState(100, 20)
	require.Len(t, state.Logs, common.LogsCap)
	require.Equal(t, fmt.Sprintf("entry-%d", total), state.Logs[0].Payload)
	require.Equal(t, fmt.Sprintf("entry-%d", total-common.LogsCap+1), state.Logs[common.LogsCap-1].Payload)
	require.Contains(t, logs.RenderLogsPage(state), i18n.Tf("logs.dropped", total-common.LogsCap))
}

func TestWSInboxDelaysOnlyLogsAndDiscardsPreviousTarget(t *testing.T) {
	b := newWSInbox()
	long := strings.Repeat("界", maxWSLogBytes)
	b.publish(0, messages.LogsWSMsg{LogType: "info", Payload: long})
	b.publish(0, messages.MemoryWSMsg{Memory: 4})
	msg, _ := b.take(b.logDue.Add(-time.Millisecond))
	batch := msg.(messages.CoreWSBatchMsg)
	require.Equal(t, int64(4), batch.Memory.Memory)
	require.Empty(t, batch.Logs)
	msg, _ = b.take(b.logDue)
	payload := msg.(messages.CoreWSBatchMsg).Logs[0].Payload
	require.True(t, utf8.ValidString(payload))
	require.LessOrEqual(t, len(payload), maxWSLogBytes)
	require.True(t, strings.HasSuffix(payload, "…"))
	b.publish(0, messages.LogsWSMsg{Payload: "waiting from old endpoint"})
	b.reset(1)
	b.publish(0, messages.TrafficWSMsg{Up: 99})
	b.publish(1, messages.MemoryWSMsg{Memory: 5})
	msg, _ = b.take(time.Now().Add(time.Second))
	batch = msg.(messages.CoreWSBatchMsg)
	require.Equal(t, uint64(1), batch.Generation)
	require.Nil(t, batch.Traffic)
	require.Empty(t, batch.Logs)
}
