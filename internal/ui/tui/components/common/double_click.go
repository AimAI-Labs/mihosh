package common

import "time"

const DefaultDoubleClickThreshold = 350 * time.Millisecond

// DoubleClickDetector generic helper to detect double clicks.
type DoubleClickDetector[T comparable] struct {
	lastContext T
	lastIndex   int
	lastAt      time.Time
}

// NewDoubleClickDetector creates a new double click detector.
func NewDoubleClickDetector[T comparable]() *DoubleClickDetector[T] {
	return &DoubleClickDetector[T]{}
}

// IsDoubleClick checks if the current click is a double click using DefaultDoubleClickThreshold.
func (d *DoubleClickDetector[T]) IsDoubleClick(ctx T, index int, now time.Time) bool {
	return d.IsDoubleClickWithThreshold(ctx, index, now, DefaultDoubleClickThreshold)
}

// IsDoubleClickWithThreshold checks if the current click is a double click based on context, index and custom threshold.
func (d *DoubleClickDetector[T]) IsDoubleClickWithThreshold(ctx T, index int, now time.Time, threshold time.Duration) bool {
	isDouble := ctx == d.lastContext &&
		index == d.lastIndex &&
		!d.lastAt.IsZero() &&
		now.Sub(d.lastAt) <= threshold

	d.lastContext = ctx
	d.lastIndex = index
	d.lastAt = now

	return isDouble
}
