package service

import (
	"sync/atomic"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
)

var testBatchSequence atomic.Uint64

type TestTask struct {
	BatchID uint64
	Index   int
	Name    string
}

// TestBatch owns capacity and completion identity; callers execute the returned tasks.
type TestBatch struct {
	id      uint64
	names   []string
	next    int
	running []int
	done    int
}

func NewTestBatch(names []string) TestBatch {
	return TestBatch{id: testBatchSequence.Add(1), names: append([]string(nil), names...)}
}

func (b *TestBatch) Take() []TestTask {
	var tasks []TestTask
	for b.next < len(b.names) && len(b.running) < model.TestConcurrency {
		index := b.next
		b.next++
		b.running = append(b.running, index)
		tasks = append(tasks, TestTask{BatchID: b.id, Index: index, Name: b.names[index]})
	}
	return tasks
}

// Complete accepts each launched task once, even if a name occurs more than once.
func (b *TestBatch) Complete(batchID uint64, index int) bool {
	if batchID == 0 || batchID != b.id {
		return false
	}
	for i, running := range b.running {
		if running == index {
			last := len(b.running) - 1
			b.running[i] = b.running[last]
			b.running = b.running[:last]
			b.done++
			return true
		}
	}
	return false
}

func (b *TestBatch) Cancel() { *b = TestBatch{} }

func (b TestBatch) Active() bool { return b.done < len(b.names) }

func (b TestBatch) Progress() (done, total int) { return b.done, len(b.names) }

func (b TestBatch) Current() string {
	if len(b.running) > 0 {
		return b.names[b.running[0]]
	}
	if b.next < len(b.names) {
		return b.names[b.next]
	}
	return ""
}
