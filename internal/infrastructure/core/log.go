package core

import "sync"

// LogBuffer retains the last 64 KiB of child output, regardless of child lifetime.
type LogBuffer struct {
	mu   sync.Mutex
	data []byte
}

func (b *LogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	const limit = 64 << 10
	if len(p) >= limit {
		b.data = append(b.data[:0], p[len(p)-limit:]...)
		return n, nil
	}
	if excess := len(b.data) + len(p) - limit; excess > 0 {
		copy(b.data, b.data[excess:])
		b.data = b.data[:len(b.data)-excess]
	}
	b.data = append(b.data, p...)
	return n, nil
}
func (b *LogBuffer) String() string { b.mu.Lock(); defer b.mu.Unlock(); return string(b.data) }
