package config

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
)

type unlockResultsCache struct {
	Scope   string                        `json:"scope"`
	Results map[string]model.UnlockResult `json:"results"`
}

func UnlockResultsScope(proxy string, endpoint MihomoEndpoint) string {
	identity := proxy + "\x00" + endpoint.ExternalController + "\x00" + endpoint.Secret
	return fmt.Sprintf("%x", sha256.Sum256([]byte(identity)))
}

func LoadUnlockResults(scope string) map[string]model.UnlockResult {
	dir, err := GetConfigDir()
	if err != nil {
		return nil
	}
	cache := readUnlockResults(filepath.Join(dir, "unlock-results.json"))
	if cache.Scope != scope {
		return nil
	}
	return cache.Results
}

// SaveUnlockResults merges a catalogue snapshot with newer results for the same proxy.
func SaveUnlockResults(scope string, results map[string]model.UnlockResult) error {
	mu.Lock()
	defer mu.Unlock()
	unlock, err := lockAppConfig()
	if err != nil {
		return err
	}
	defer unlock()

	dir, err := GetConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	path := filepath.Join(dir, "unlock-results.json")
	previous := readUnlockResults(path)
	if previous.Scope != scope && latestUnlockTime(previous.Results).After(latestUnlockTime(results)) {
		return nil
	}
	next := unlockResultsCache{Scope: scope, Results: make(map[string]model.UnlockResult, len(results))}
	for name, result := range results {
		if old := previous.Results[name]; previous.Scope == scope && old.CheckedAt.After(result.CheckedAt) {
			result = old
		}
		if !result.CheckedAt.IsZero() {
			next.Results[name] = result
		}
	}
	data, err := json.Marshal(next)
	if err != nil {
		return err
	}
	return atomicWriteFile(path, data, 0600)
}

func readUnlockResults(path string) unlockResultsCache {
	file, err := os.Open(path)
	if err != nil {
		return unlockResultsCache{}
	}
	defer func() { _ = file.Close() }()
	var cache unlockResultsCache
	if json.NewDecoder(io.LimitReader(file, 16<<10)).Decode(&cache) != nil {
		return unlockResultsCache{}
	}
	return cache
}

func latestUnlockTime(results map[string]model.UnlockResult) time.Time {
	var latest time.Time
	for _, result := range results {
		if result.CheckedAt.After(latest) {
			latest = result.CheckedAt
		}
	}
	return latest
}
