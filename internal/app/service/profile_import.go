package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
)

// ImportRawDirectory discovers user-supplied YAML at the top level of raw/.
func (s *ProfileService) ImportRawDirectory() error {
	dir, err := profile.RawDirectory()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	profiles, _, err := s.ListProfiles()
	if err != nil {
		return err
	}
	var importErr error
	for _, entry := range entries {
		name := entry.Name()
		ext := filepath.Ext(name)
		label := strings.TrimSpace(strings.TrimSuffix(name, ext))
		if label == "" || !entry.Type().IsRegular() || (!strings.EqualFold(ext, ".yaml") && !strings.EqualFold(ext, ".yml")) {
			continue
		}
		path := filepath.Join(dir, name)
		if hasLocalSource(profiles, path) {
			continue
		}
		if err = importRawFile(path, label); err != nil {
			importErr = errors.Join(importErr, fmt.Errorf("%s: %w", name, err))
		}
	}
	return importErr
}

func hasLocalSource(profiles []profile.Profile, path string) bool {
	for _, current := range profiles {
		if current.Source.Kind != profile.SourceLocal {
			continue
		}
		abs, err := filepath.Abs(current.Source.Path)
		if err == nil && (abs == path || runtime.GOOS == "windows" && strings.EqualFold(abs, path)) {
			return true
		}
	}
	return false
}

func importRawFile(path, name string) error {
	uid, err := newUID()
	if err != nil {
		return err
	}
	entry := profile.Profile{
		UID: uid, Name: name, UpdatedAt: time.Now().Unix(),
		Source: profile.SubSource{Kind: profile.SourceLocal, Path: path},
	}
	if err = profile.Fetch(entry); err != nil {
		return err
	}
	alreadyImported := errors.New("profile is already imported")
	err = config.Update(func(cfg *config.Config) error {
		if hasLocalSource(cfg.Subs, path) {
			return alreadyImported
		}
		cfg.Subs = append(cfg.Subs, entry)
		return nil
	})
	if errors.Is(err, alreadyImported) {
		return profile.DeleteProfileDir(uid)
	}
	if err != nil {
		return errors.Join(err, profile.DeleteProfileDir(uid))
	}
	return err
}
