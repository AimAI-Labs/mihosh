package core

import (
	"strings"
	"testing"
)

func TestLogBufferReadSince(t *testing.T) {
	var b LogBuffer
	if text, start := b.ReadSince(0); text != "" || start != 0 {
		t.Fatalf("empty buffer: text=%q start=%d", text, start)
	}
	_, _ = b.Write([]byte("first\n"))
	text, start := b.ReadSince(0)
	if text != "first\n" || start != 0 {
		t.Fatalf("first read: text=%q start=%d", text, start)
	}
	next := start + uint64(len(text))
	_, _ = b.Write([]byte("second\n"))
	text, start = b.ReadSince(next)
	if text != "second\n" || start != next {
		t.Fatalf("incremental read: text=%q start=%d", text, start)
	}
	next = start + uint64(len(text))
	for _, offset := range []uint64{next, next + 100} {
		if text, start := b.ReadSince(offset); text != "" || start != next {
			t.Fatalf("read beyond end: text=%q start=%d", text, start)
		}
	}
	if got := b.String(); got != "first\nsecond\n" {
		t.Fatalf("String changed: %q", got)
	}
}

func TestLogBufferReadSinceEviction(t *testing.T) {
	const limit = 64 << 10
	for _, writes := range [][]string{
		{strings.Repeat("x", limit-2), "ABCD"},
		{strings.Repeat("x", limit+2)},
	} {
		var b LogBuffer
		all := strings.Join(writes, "")
		for _, write := range writes {
			if n, err := b.Write([]byte(write)); err != nil || n != len(write) {
				t.Fatalf("write: n=%d err=%v", n, err)
			}
		}
		text, start := b.ReadSince(0)
		if text != all[len(all)-limit:] || start != uint64(len(all)-limit) || b.String() != text {
			t.Fatalf("eviction: length=%d start=%d", len(text), start)
		}
		text, start = b.ReadSince(uint64(len(all) - 1))
		if text != all[len(all)-1:] || start != uint64(len(all)-1) {
			t.Fatalf("retained suffix: text=%q start=%d", text, start)
		}
	}
}
