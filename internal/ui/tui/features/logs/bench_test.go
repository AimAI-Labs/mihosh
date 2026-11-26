package logs

import (
	"fmt"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
)

func BenchmarkLogBurst(b *testing.B) {
	i18n.Init()
	var entries []model.LogEntry
	for i := range 500 {
		entries = append(entries, model.LogEntry{Type: "info", Payload: fmt.Sprintf("proxy update %d", i), Timestamp: time.Unix(0, 0)})
	}
	for _, render := range []bool{false, true} {
		for _, batch := range []bool{false, true} {
			b.Run(fmt.Sprintf("render=%t/batch=%t", render, batch), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					state := NewState(nil)
					state.logFilter = "proxy"
					if batch {
						state = state.AppendLogs(entries, 0)
						if render {
							_ = RenderLogsPage(state.ToPageState(100, 20))
						}
					} else {
						for i := range entries {
							state = state.AppendLogs(entries[i:i+1], 0)
							if render {
								_ = RenderLogsPage(state.ToPageState(100, 20))
							}
						}
					}
				}
			})
		}
	}
}
