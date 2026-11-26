package tui

import (
	"context"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	wsLogBatchDelay = 50 * time.Millisecond
	maxWSLogBytes   = 16 << 10
)

// wsInbox coalesces each live stream independently and keeps logs in arrival order.
// One Bubble Tea listener owns delivery, while the four streams can publish concurrently.
type wsInbox struct {
	mu      sync.Mutex
	wake    chan struct{}
	pending messages.CoreWSBatchMsg
	logs    [common.LogsCap]model.LogEntry
	logHead int
	logSize int
	logDue  time.Time
}

func newWSInbox() *wsInbox { return &wsInbox{wake: make(chan struct{}, 1)} }

func (b *wsInbox) reset(generation uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.advanceGeneration(generation)
}

func (b *wsInbox) advanceGeneration(generation uint64) {
	if generation > b.pending.Generation {
		b.pending = messages.CoreWSBatchMsg{Generation: generation}
		b.clearLogs()
	}
}

func (b *wsInbox) publish(generation uint64, msg tea.Msg) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if generation < b.pending.Generation {
		return
	}
	b.advanceGeneration(generation)
	switch msg := msg.(type) {
	case messages.MemoryWSMsg:
		b.pending.Memory = &msg
	case messages.TrafficWSMsg:
		b.pending.Traffic = &msg
	case messages.ConnectionsWSMsg:
		b.pending.Connections = &msg
	case messages.LogsWSMsg:
		now := time.Now()
		b.logs[b.logHead] = model.LogEntry{Type: msg.LogType, Payload: boundLogPayload(msg.Payload), Timestamp: now}
		b.logHead = (b.logHead + 1) % len(b.logs)
		if b.logSize == len(b.logs) {
			b.pending.DroppedLogs++
		} else {
			b.logSize++
		}
		if b.logSize > 1 {
			return
		}
		b.logDue = now.Add(wsLogBatchDelay)
	}
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

func boundLogPayload(payload string) string {
	if len(payload) <= maxWSLogBytes {
		return payload
	}
	end := maxWSLogBytes - len("…")
	for end > 0 && !utf8.RuneStart(payload[end]) {
		end--
	}
	return payload[:end] + "…"
}

func (b *wsInbox) clearLogs() {
	clear(b.logs[:])
	b.logHead, b.logSize = 0, 0
	b.logDue = time.Time{}
}

func (b *wsInbox) take(now time.Time) (tea.Msg, time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	batch := b.pending
	b.pending.Memory, b.pending.Traffic, b.pending.Connections = nil, nil, nil
	batch.DroppedLogs = 0
	var delay time.Duration
	if b.logSize > 0 {
		delay = b.logDue.Sub(now)
		if delay <= 0 {
			batch.Logs = make([]model.LogEntry, b.logSize)
			start := (b.logHead - b.logSize + len(b.logs)) % len(b.logs)
			n := copy(batch.Logs, b.logs[start:])
			copy(batch.Logs[n:], b.logs[:b.logHead])
			batch.DroppedLogs = b.pending.DroppedLogs
			b.pending.DroppedLogs = 0
			b.clearLogs()
		}
	}
	if batch.Memory != nil || batch.Traffic != nil || batch.Connections != nil || len(batch.Logs) > 0 {
		return batch, 0
	}
	return nil, delay
}

func listenWSMessages(ctx context.Context, inbox *wsInbox) tea.Cmd {
	if inbox == nil {
		return nil
	}
	return func() tea.Msg {
		for ctx.Err() == nil {
			msg, delay := inbox.take(time.Now())
			if msg != nil {
				return msg
			}
			var timer *time.Timer
			var elapsed <-chan time.Time
			if delay > 0 {
				timer = time.NewTimer(delay)
				elapsed = timer.C
			}
			select {
			case <-ctx.Done():
			case <-inbox.wake:
			case <-elapsed:
			}
			if timer != nil {
				timer.Stop()
			}
		}
		return nil
	}
}
