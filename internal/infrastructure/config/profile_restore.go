package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	profileRestoreDir = ".sync-restore"
	restoreJournal    = "pending.json"
)

// ProfileFiles contains the portable profile overlays. A nil file is omitted unless
// its corresponding Remove* flag is set.
type ProfileFiles struct {
	Merge, Rules, Chains            []byte
	RemoveMerge, RemoveRules, RemoveChains bool
}

// ReadProfileFiles takes a consistent snapshot for sync export and upload.
func ReadProfileFiles() (*Config, ProfileFiles, error) {
	mu.Lock()
	defer mu.Unlock()
	unlock, err := lockAppConfig()
	if err != nil {
		return nil, ProfileFiles{}, err
	}
	defer unlock()
	cfg, err := loadOrDefaultLocked()
	if err != nil {
		return nil, ProfileFiles{}, err
	}
	root, err := GetConfigDir()
	if err != nil {
		return nil, ProfileFiles{}, err
	}
	var files ProfileFiles
	for _, file := range []struct {
		name string
		data *[]byte
	}{{"merge.yaml", &files.Merge}, {"rule-edits.yaml", &files.Rules}, {"chain-edits.yaml", &files.Chains}} {
		*file.data, err = os.ReadFile(filepath.Join(root, file.name))
		if err != nil && !os.IsNotExist(err) {
			return nil, ProfileFiles{}, err
		}
	}
	return cfg, files, nil
}

// RestoreProfileFiles stages all changes under the application configuration lock.
// Like Update, the callback must not call other functions that lock the config.
func RestoreProfileFiles(prepare func(*Config) (ProfileFiles, error)) error {
	return restoreProfileFiles(prepare, os.Rename)
}

func restoreProfileFiles(prepare func(*Config) (ProfileFiles, error), install func(string, string) error) error {
	mu.Lock()
	defer mu.Unlock()
	unlock, err := lockAppConfig()
	if err != nil {
		return err
	}
	defer unlock()
	cfg, err := loadOrDefaultLocked()
	if err != nil {
		return err
	}
	files, err := prepare(cfg)
	if err != nil {
		return err
	}
	root, err := GetConfigDir()
	if err != nil {
		return err
	}
	stage := filepath.Join(root, profileRestoreDir)
	if err := os.Mkdir(stage, 0700); err != nil {
		return err
	}
	var entries []restoreEntry
	defer func() {
		if _, err := os.Lstat(filepath.Join(stage, restoreJournal)); os.IsNotExist(err) {
			_ = os.RemoveAll(stage)
		}
	}()
	for _, file := range []struct {
		name   string
		data   []byte
		remove bool
	}{
		{"merge.yaml", files.Merge, files.RemoveMerge},
		{"rule-edits.yaml", files.Rules, files.RemoveRules},
		{"chain-edits.yaml", files.Chains, files.RemoveChains},
		{"config.yaml", nil, false},
	} {
		var err error
		dst := filepath.Join(stage, file.name+".new")
		switch {
		case file.name == "config.yaml":
			err = writeConfigFile(cfg, root, dst)
		case file.data != nil:
			err = writeConfigAtomically(dst, 0600, func(w io.Writer) error {
				_, err := w.Write(file.data)
				return err
			})
		case file.remove:
			// Back up existing file (if any) so rollback can restore it if a later step fails.
		default:
			continue
		}
		if err != nil {
			return fmt.Errorf("stage %s: %w", file.name, err)
		}
		existed, err := backupRestoreFile(filepath.Join(root, file.name), filepath.Join(stage, file.name+".old"))
		if err != nil {
			return fmt.Errorf("back up %s: %w", file.name, err)
		}
		if file.remove && !existed {
			continue
		}
		entries = append(entries, restoreEntry{Name: file.name, Existed: existed, Remove: file.remove})
	}
	journal := filepath.Join(stage, restoreJournal)
	if err := writeConfigAtomically(journal, 0600, func(w io.Writer) error { return json.NewEncoder(w).Encode(entries) }); err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Remove {
			if err := os.Remove(filepath.Join(root, entry.Name)); err != nil && !os.IsNotExist(err) {
				return errors.Join(fmt.Errorf("remove %s: %w", entry.Name, err), recoverProfileRestore(root))
			}
			continue
		}
		if err := install(filepath.Join(stage, entry.Name+".new"), filepath.Join(root, entry.Name)); err != nil {
			return errors.Join(fmt.Errorf("restore %s: %w", entry.Name, err), recoverProfileRestore(root))
		}
	}
	// Removing the journal commits the bundle. Recovery rolls back any earlier exit.
	if err := os.Remove(journal); err != nil {
		return errors.Join(err, recoverProfileRestore(root))
	}
	return nil
}

func backupRestoreFile(path, backup string) (bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(path)
		if err != nil {
			return false, err
		}
		return true, os.Symlink(link, backup)
	}
	if !info.Mode().IsRegular() {
		return false, errors.New("expected a regular file")
	}
	old, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer old.Close()
	return true, writeConfigAtomically(backup, info.Mode().Perm(), func(w io.Writer) error {
		_, err := io.Copy(w, old)
		return err
	})
}

type restoreEntry struct {
	Name    string `json:"name"`
	Existed bool   `json:"existed"`
	Remove  bool   `json:"remove,omitempty"`
}

func recoverProfileRestore(root string) error {
	stage := filepath.Join(root, profileRestoreDir)
	info, err := os.Lstat(stage)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("invalid sync restore directory")
	}
	journal := filepath.Join(stage, restoreJournal)
	data, err := os.ReadFile(journal)
	if os.IsNotExist(err) {
		return os.RemoveAll(stage)
	}
	if err != nil {
		return err
	}
	var entries []restoreEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("invalid sync restore journal: %w", err)
	}
	for _, entry := range entries {
		switch entry.Name {
		case "config.yaml", "merge.yaml", "rule-edits.yaml", "chain-edits.yaml":
		default:
			return errors.New("invalid file in sync restore journal")
		}
	}
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if !entry.Remove {
			if _, err := os.Lstat(filepath.Join(stage, entry.Name+".new")); err == nil {
				continue // The staged file was never installed.
			} else if !os.IsNotExist(err) {
				return err
			}
		}
		path := filepath.Join(root, entry.Name)
		if entry.Existed {
			backup := filepath.Join(stage, entry.Name+".old")
			if err := os.Rename(backup, path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("roll back %s: %w", entry.Name, err)
			}
		} else if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("roll back %s: %w", entry.Name, err)
		}
	}
	if err := os.Remove(journal); err != nil {
		return err
	}
	return os.RemoveAll(stage)
}
