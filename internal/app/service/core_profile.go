package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"gopkg.in/yaml.v3"
)

type profileRecovery struct {
	Config    []byte `json:"config"`
	ActiveSub string `json:"active_sub"`
}

func privateWrite(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".core-write-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

// sanitizeManagedInbound prevents subscription data from silently exposing listeners
// or enabling privileged network capture. The managed baseline remains authoritative.
func sanitizeManagedInbound(data []byte) ([]byte, error) {
	var mapping map[string]any
	if err := yaml.Unmarshal(data, &mapping); err != nil {
		return nil, err
	}
	for _, key := range []string{"port", "socks-port", "redir-port", "tproxy-port", "listeners", "external-controller-tls", "external-controller-unix", "external-controller-pipe", "external-ui", "external-ui-url"} {
		delete(mapping, key)
	}
	if dns, ok := mapping["dns"].(map[string]any); ok {
		delete(dns, "listen")
	}
	mapping["tun"] = map[string]any{"enable": false}
	return yaml.Marshal(mapping)
}

func recoverManagedProfile(root string) error {
	journal := filepath.Join(root, "runtime", "profile-recovery.json")
	data, err := os.ReadFile(journal)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var recovery profileRecovery
	if err = json.Unmarshal(data, &recovery); err != nil {
		return errors.New("invalid managed profile recovery journal")
	}
	if err = privateWrite(filepath.Join(root, "runtime", "config.yaml"), recovery.Config); err != nil {
		return err
	}
	if err = config.Update(func(latest *config.Config) error {
		latest.ActiveSub = ""
		for _, p := range latest.Subs {
			if p.UID == recovery.ActiveSub {
				latest.ActiveSub = p.UID
				break
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return os.Remove(journal)
}

// ApplyProfile is serialized with core installation and process operations. A journal
// preserves the last committed configuration across interruption of the apply step.
func (s *CoreService) ApplyProfile(ctx context.Context, uid string, startIfStopped bool) (ActivateResult, error) {
	ctx, cancelOperation := s.operationContext(ctx)
	defer cancelOperation()
	if !s.mu.TryLock() {
		return ActivateResult{}, errors.New("another core operation is active")
	}
	defer s.mu.Unlock()
	cfg, err := loadCoreConfig()
	if err != nil {
		return ActivateResult{}, err
	}
	root, err := config.GetConfigDir()
	if err != nil {
		return ActivateResult{}, err
	}
	path := filepath.Join(root, "runtime", "config.yaml")
	unlock, err := coreinfra.LockRuntime(filepath.Join(root, "runtime", "config.lock"))
	if err != nil {
		return ActivateResult{}, err
	}
	defer unlock()
	if err = recoverManagedProfile(root); err != nil {
		return ActivateResult{}, err
	}
	cfg, err = loadCoreConfig()
	if err != nil {
		return ActivateResult{}, err
	}
	if s.process == nil {
		// Do not mutate a live managed core owned by another Mihosh process.
		unlockRuntime, e := coreinfra.LockRuntime(filepath.Join(root, "runtime", "process.lock"))
		if e != nil {
			return ActivateResult{}, e
		}
		unlockRuntime()
	}
	old, err := os.ReadFile(path)
	if err != nil {
		return ActivateResult{}, err
	}
	data, mergeErr := profile.GenerateAndWriteForUIDIgnoreMergeErrorWithPreserve(uid, path)
	if len(data) == 0 {
		return ActivateResult{}, profile.ErrRawNotFound
	}
	data, err = sanitizeManagedInbound(data)
	if err != nil {
		return ActivateResult{}, err
	}
	candidate, err := os.CreateTemp(filepath.Dir(path), ".candidate-*.yaml")
	if err != nil {
		return ActivateResult{}, err
	}
	defer os.Remove(candidate.Name())
	if _, err = candidate.Write(data); err != nil {
		candidate.Close()
		return ActivateResult{}, err
	}
	if err = candidate.Close(); err != nil {
		return ActivateResult{}, err
	}
	binary, err := coreinfra.BinaryPath(root, cfg.Core.Version, runtime.GOOS)
	if err != nil {
		return ActivateResult{}, err
	}
	validateCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	command := exec.CommandContext(validateCtx, binary, "-t", "-d", filepath.Dir(path), "-f", candidate.Name())
	coreinfra.ConfigureProcess(command)
	err = command.Run()
	cancel()
	if err != nil {
		return ActivateResult{}, errors.New("candidate subscription failed core validation")
	}
	if err = ctx.Err(); err != nil {
		return ActivateResult{}, err
	}
	backupDir := filepath.Join(root, "config-backups")
	if err = os.MkdirAll(backupDir, 0700); err != nil {
		return ActivateResult{}, err
	}
	if err = os.Chmod(backupDir, 0700); err != nil {
		return ActivateResult{}, err
	}
	backup, err := profile.BackupConfig(path)
	if err != nil {
		return ActivateResult{}, err
	}
	if backup != "" {
		if err = os.Chmod(filepath.Join(backupDir, backup), 0600); err != nil {
			return ActivateResult{}, err
		}
	}
	recovery := profileRecovery{old, cfg.ActiveSub}
	journal := filepath.Join(root, "runtime", "profile-recovery.json")
	record, _ := json.Marshal(recovery)
	if err = privateWrite(journal, record); err != nil {
		return ActivateResult{}, err
	}
	endpoint := config.ResolveMihomoEndpoint()
	client := api.NewClient(endpoint, 1000)
	running := s.process != nil
	started := false
	rollback := func(cause error) (ActivateResult, error) {
		if started {
			s.stopLocked()
		}
		restoreErr := recoverManagedProfile(root)
		var reloadErr error
		if restoreErr == nil && running {
			reloadErr = client.ReloadConfig(path)
		}
		return ActivateResult{}, errors.Join(cause, restoreErr, reloadErr)
	}
	if err = privateWrite(path, data); err != nil {
		return rollback(err)
	}
	if running {
		if err = client.ReloadConfig(path); err != nil {
			return rollback(fmt.Errorf("managed subscription reload failed: %w", err))
		}
		if _, err = client.GetVersion(); err != nil {
			return rollback(errors.New("core did not become ready after subscription reload"))
		}
	} else if startIfStopped {
		if err = s.start(ctx, true); err != nil {
			return rollback(err)
		}
		started = true
	}
	if err = config.Update(func(latest *config.Config) error {
		for _, p := range latest.Subs {
			if p.UID == uid {
				latest.ActiveSub = uid
				return nil
			}
		}
		return ErrSubNotFound
	}); err != nil {
		return rollback(err)
	}
	if err = os.Remove(journal); err != nil {
		return rollback(err)
	}
	return ActivateResult{BackupName: backup, MergeErr: mergeErr}, nil
}

// ImportProfile reuses the subscription store while keeping first-time configuration
// inside the Core menu. Failed fetches retain metadata for an explicit retry.
func (s *CoreService) ImportProfile(ctx context.Context, name, source string, startIfStopped bool) error {
	ctx, cancel := s.operationContext(ctx)
	defer cancel()
	cfg, err := loadCoreConfig()
	if err != nil {
		return err
	}
	if cfg.Core.Mode != "managed" {
		return errors.New("enable managed mode before applying a subscription from Core")
	}
	src := profile.SubSource{Kind: profile.ParseSourceKind(source)}
	if src.Kind == profile.SourceRemote {
		src.URL = source
	} else {
		src.Path = source
	}
	profiles := NewProfileService(nil, s)
	p, err := profiles.AddProfile(name, src)
	if err != nil {
		return err
	}
	if err = profiles.FetchProfileContext(ctx, p.UID); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("subscription fetch failed; the entry is retained for retry on the Sub page")
	}
	_, err = s.ApplyProfile(ctx, p.UID, startIfStopped)
	return err
}
