package core

import (
	"fmt"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"os"
	"testing"
)

func TestReleaseStoreCachesBoundedRecordsAndRetainsAllHistory(t *testing.T) {
	store, err := newReleaseStore()
	if err != nil {
		t.Fatal(err)
	}
	defer store.close()
	for page := 0; page < 30; page++ {
		var rows []coreinfra.Release
		for i := 0; i < 10; i++ {
			rows = append(rows, coreinfra.Release{Tag: fmt.Sprintf("v1.%d.%d", page, i), Trusted: true})
		}
		if err := store.append(rows); err != nil {
			t.Fatal(err)
		}
	}
	if store.count != 300 {
		t.Fatal("not all release records were retained")
	}
	for _, index := range []int{0, 299, 63, 127, 128, 1} {
		r, err := store.at(index)
		if err != nil || r.Tag != fmt.Sprintf("v1.%d.%d", index/10, index%10) {
			t.Fatalf("record %d lost after cache eviction: %+v %v", index, r, err)
		}
	}
	cached := 0
	for _, slot := range store.cache {
		if slot.valid {
			cached++
		}
	}
	if cached > releaseCacheSize {
		t.Fatal("unbounded memory cache")
	}
	if err := store.append([]coreinfra.Release{{Tag: "v1.0.0", Trusted: false}}); err != nil {
		t.Fatal(err)
	}
	r, _ := store.at(0)
	if store.count != 300 || r.Trusted {
		t.Fatal("duplicate release was appended instead of updated")
	}
	root := store.root
	store.close()
	store.close()
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("temporary metadata not cleaned up")
	}
	if _, err := store.at(0); err == nil {
		t.Fatal("closed catalogue still readable")
	}
	if err := store.append([]coreinfra.Release{{Tag: "v2.0.0"}}); err == nil {
		t.Fatal("closed catalogue still writable")
	}
}
