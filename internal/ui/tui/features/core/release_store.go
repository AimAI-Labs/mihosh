package core

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
)

const releaseCacheSize = 64

// releaseStore keeps the full lazily loaded catalogue on disk. The fixed cache
// bounds memory independently of the number of releases in the upstream history.
// All access is on the Bubble Tea update/view goroutine.
type releaseStore struct {
	root   string
	count  int
	closed bool
	cache  [releaseCacheSize]struct {
		index   int
		valid   bool
		release coreinfra.Release
	}
}

func newReleaseStore() (*releaseStore, error) {
	root, err := os.MkdirTemp("", "mihosh-release-catalog-*")
	if err != nil {
		return nil, err
	}
	return &releaseStore{root: root}, nil
}

func (c *releaseStore) close() {
	if c != nil && !c.closed {
		c.closed = true
		_ = os.RemoveAll(c.root)
		c.cache = [releaseCacheSize]struct {
			index   int
			valid   bool
			release coreinfra.Release
		}{}
	}
}

func (c *releaseStore) tagPath(tag string) string {
	return filepath.Join(c.root, fmt.Sprintf("tag-%x", sha256.Sum256([]byte(tag))))
}

func (c *releaseStore) find(tag string) (int, bool) {
	if c.closed {
		return 0, false
	}
	data, err := os.ReadFile(c.tagPath(tag))
	if err != nil {
		return 0, false
	}
	index, err := strconv.Atoi(string(data))
	return index, err == nil && index >= 0 && index < c.count
}

func (c *releaseStore) at(index int) (coreinfra.Release, error) {
	if c.closed || index < 0 || index >= c.count {
		return coreinfra.Release{}, errors.New("release record unavailable")
	}
	slot := &c.cache[index%releaseCacheSize]
	if slot.valid && slot.index == index {
		return slot.release, nil
	}
	data, err := os.ReadFile(filepath.Join(c.root, strconv.Itoa(index)+".json"))
	if err != nil {
		return coreinfra.Release{}, err
	}
	var r coreinfra.Release
	if err := json.Unmarshal(data, &r); err != nil {
		return coreinfra.Release{}, err
	}
	slot.index, slot.valid, slot.release = index, true, r
	return r, nil
}

func (c *releaseStore) put(index int, r coreinfra.Release) error {
	if c.closed {
		return errors.New("release catalogue is closed")
	}
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	path := filepath.Join(c.root, strconv.Itoa(index)+".json")
	if err := os.WriteFile(path+".tmp", data, 0600); err != nil {
		return err
	}
	if err := os.Rename(path+".tmp", path); err != nil {
		return err
	}
	if err := os.WriteFile(c.tagPath(r.Tag), []byte(strconv.Itoa(index)), 0600); err != nil {
		return err
	}
	slot := &c.cache[index%releaseCacheSize]
	slot.index, slot.valid, slot.release = index, true, r
	return nil
}

func (c *releaseStore) append(releases []coreinfra.Release) error {
	for _, r := range releases {
		if index, found := c.find(r.Tag); found {
			if err := c.put(index, r); err != nil {
				return err
			}
			continue
		}
		if err := c.put(c.count, r); err != nil {
			return err
		}
		c.count++
	}
	return nil
}
