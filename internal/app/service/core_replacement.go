package service

import (
	"context"
	"errors"
	"fmt"
)

// Version and TUN changes retain their own durable recovery records.
// The runtime alone owns stopping, candidate startup and recovery restart ordering.
type coreReplacement interface {
	apply(running bool) error
	commit() error
	restore() error
}

// replaceCore runs with s.mu and the config mutation lock held by the caller.
func (s *CoreService) replaceCore(ctx context.Context, change coreReplacement) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	wasRunning := s.process != nil
	if wasRunning {
		select {
		case <-s.done:
			wasRunning = false
		default:
		}
		if err := s.stopLocked(); err != nil {
			return err
		}
	}
	rollback := func(cause error) error {
		var stopErr, restartErr error
		if s.process != nil {
			stopErr = s.stopLocked()
		}
		restoreErr := change.restore()
		if wasRunning && restoreErr == nil {
			// Cancellation of the replacement must not prevent recovery of the old selection.
			restartErr = s.start(context.Background(), true)
		}
		return errors.Join(cause, stopErr, restoreErr, restartErr)
	}
	if err := ctx.Err(); err != nil {
		return rollback(err)
	}
	if err := change.apply(wasRunning); err != nil {
		return rollback(err)
	}
	if wasRunning {
		if err := s.start(ctx, true); err != nil {
			return rollback(errors.Join(fmt.Errorf("candidate core failed: %w", err), ctx.Err()))
		}
	}
	if err := ctx.Err(); err != nil {
		return rollback(err)
	}
	if err := change.commit(); err != nil {
		return rollback(err)
	}
	return nil
}
