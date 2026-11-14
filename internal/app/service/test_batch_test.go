package service

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
)

func TestTestBatchCompletionIdentityAndCancellation(t *testing.T) {
	names := make([]string, model.TestConcurrency*5)
	for i := range names {
		names[i] = "same name"
	}
	batch := NewTestBatch(names)
	tasks := batch.Take()
	if len(tasks) != model.TestConcurrency || len(batch.Take()) != 0 {
		t.Fatal("initial tasks exceeded capacity")
	}
	last := tasks[len(tasks)-1]
	if batch.Complete(last.BatchID+1, last.Index) || batch.Complete(last.BatchID, len(names)-1) {
		t.Fatal("accepted a foreign or unlaunched task")
	}
	if !batch.Complete(last.BatchID, last.Index) {
		t.Fatal("out-of-order completion was rejected")
	}
	if replacement := batch.Take(); len(replacement) != 1 || replacement[0].Index != model.TestConcurrency {
		t.Fatalf("completion did not fill exactly one slot: %+v", replacement)
	}
	if batch.Complete(last.BatchID, last.Index) || len(batch.Take()) != 0 {
		t.Fatal("duplicate completion changed capacity")
	}
	if done, total := batch.Progress(); done != 1 || total != len(names) {
		t.Fatalf("progress=%d/%d", done, total)
	}
	batch.Cancel()
	if batch.Active() || len(batch.Take()) != 0 || batch.Complete(tasks[0].BatchID, tasks[0].Index) {
		t.Fatal("cancelled work advanced the batch")
	}
	batch = NewTestBatch(names)
	current := batch.Take()
	if batch.Complete(tasks[0].BatchID, tasks[0].Index) || current[0].BatchID == tasks[0].BatchID {
		t.Fatal("an earlier run completed a new task")
	}
	if !batch.Complete(current[0].BatchID, current[0].Index) || !batch.Complete(current[1].BatchID, current[1].Index) {
		t.Fatal("tasks with the same name were not tracked independently")
	}
}

func TestEmptyTestBatch(t *testing.T) {
	batch := NewTestBatch(nil)
	if batch.Active() || len(batch.Take()) != 0 || batch.Current() != "" {
		t.Fatal("empty batch started work")
	}
}
