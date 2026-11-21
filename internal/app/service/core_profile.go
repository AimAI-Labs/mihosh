package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/charmbracelet/x/ansi"
	"gopkg.in/yaml.v3"
)

type profileRecovery struct {
	Config       []byte               `json:"config"`
	ActiveSub    string               `json:"active_sub"`
	HasMerge     bool                 `json:"has_merge,omitempty"`
	MergeExisted bool                 `json:"merge_existed,omitempty"`
	Merge        []byte               `json:"merge,omitempty"`
	RuleEdits    *profileFileRecovery `json:"rule_edits,omitempty"`
	ChainEdits   *profileFileRecovery `json:"chain_edits,omitempty"`
}

type profileFileRecovery struct {
	Existed bool   `json:"existed"`
	Data    []byte `json:"data,omitempty"`
}

var validationURL = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.-]*://[^\s"'<>]+`)

// Validation errors can quote subscription values; retain the reason without credentials.
func profileValidationOutput(output string, data []byte) string {
	var root yaml.Node
	if yaml.Unmarshal(data, &root) != nil {
		return ""
	}
	var sensitive []string
	var collect func(*yaml.Node, bool)
	collect = func(node *yaml.Node, secret bool) {
		if node.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(node.Content); i += 2 {
				key := strings.ToLower(strings.ReplaceAll(node.Content[i].Value, "_", "-"))
				credential := secret
				for _, part := range []string{"password", "secret", "token", "auth", "uuid", "private-key", "pre-shared-key", "username", "psk"} {
					credential = credential || strings.Contains(key, part)
				}
				collect(node.Content[i+1], credential)
			}
			return
		}
		if node.Kind == yaml.ScalarNode && node.Value != "" {
			value := node.Value
			if secret {
				quoted := strconv.Quote(value)
				sensitive = append(sensitive, value, quoted[1:len(quoted)-1])
			}
			for _, rawURL := range validationURL.FindAllString(value, -1) {
				sensitive = append(sensitive, rawURL)
				if parsed, err := url.Parse(rawURL); err == nil {
					sensitive = append(sensitive, parsed.RawQuery)
					if parsed.User != nil {
						password, _ := parsed.User.Password()
						for _, value := range []string{parsed.User.Username(), password} {
							quoted := strconv.Quote(value)
							sensitive = append(sensitive, value, quoted[1:len(quoted)-1], url.QueryEscape(value), url.PathEscape(value))
						}
					}
					for _, values := range parsed.Query() {
						for _, value := range values {
							sensitive = append(sensitive, value, url.QueryEscape(value))
						}
					}
				}
			}
		}
		for _, child := range node.Content {
			collect(child, secret)
		}
	}
	collect(&root, false)
	output = ansi.Strip(output)
	sort.Slice(sensitive, func(i, j int) bool { return len(sensitive[i]) > len(sensitive[j]) })
	var replacements []string
	for _, value := range sensitive {
		if value != "" {
			replacements = append(replacements, value, "[redacted]")
		}
	}
	output = strings.NewReplacer(replacements...).Replace(output)
	output = validationURL.ReplaceAllString(output, "[redacted URL]")
	output = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' {
			return ' '
		}
		return r
	}, output))
	const limit = 4096
	if len(output) > limit {
		output = "…" + strings.ToValidUTF8(output[len(output)-limit:], "")
	}
	return output
}

type validationOutput struct {
	buffer      coreinfra.LogBuffer
	interrupt   func(string) error
	stop        context.CancelFunc
	once        sync.Once
	interrupted error
}

func (w *validationOutput) Write(p []byte) (int, error) {
	n, err := w.buffer.Write(p)
	if w.interrupt != nil {
		if reason := w.interrupt(w.buffer.String()); reason != nil {
			w.once.Do(func() { w.interrupted = reason; w.stop() })
		}
	}
	return n, err
}

func validateManagedProfile(ctx context.Context, binary, path, candidate string, data []byte, interrupt func(string) error) error {
	validateCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	command := exec.CommandContext(validateCtx, binary, "-t", "-d", filepath.Dir(path), "-f", candidate)
	coreinfra.ConfigureProcess(command)
	output := validationOutput{interrupt: interrupt, stop: cancel}
	command.Stdout, command.Stderr = &output, &output
	command.WaitDelay = time.Second
	err := command.Run()
	if ctx.Err() != nil {
		return fmt.Errorf("subscription validation interrupted: %w", ctx.Err())
	}
	if err == nil {
		return nil
	}
	if output.interrupted != nil {
		return fmt.Errorf("%w: %s", output.interrupted, profileValidationOutput(output.buffer.String(), data))
	}
	if validateCtx.Err() != nil {
		return fmt.Errorf("subscription validation interrupted: %w", validateCtx.Err())
	}
	var exited *exec.ExitError
	if !errors.As(err, &exited) {
		return fmt.Errorf("cannot start core validation: %w", err)
	}
	if detail := profileValidationOutput(output.buffer.String(), data); detail != "" {
		return fmt.Errorf("candidate subscription failed core validation: %w: %s", err, detail)
	}
	return fmt.Errorf("candidate subscription failed core validation: %w", err)
}

func privateWrite(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".core-write-")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(file.Name()) }()
	if _, err = file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		_ = file.Close()
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
	if recovery.HasMerge {
		if err = restoreMergeFile(filepath.Join(root, "merge.yaml"), recovery.MergeExisted, recovery.Merge); err != nil {
			return err
		}
	}
	if rules := recovery.RuleEdits; rules != nil {
		path, err := profile.RuleEditsPath()
		if err != nil {
			return err
		}
		if err = restoreMergeFile(path, rules.Existed, rules.Data); err != nil {
			return err
		}
	}
	if chains := recovery.ChainEdits; chains != nil {
		path, err := profile.ChainEditsPath()
		if err != nil {
			return err
		}
		if err = restoreMergeFile(path, chains.Existed, chains.Data); err != nil {
			return err
		}
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
	return s.applyProfile(ctx, uid, startIfStopped, false, nil)
}

func (s *CoreService) ApplyProfileCleared(ctx context.Context, uid string, startIfStopped bool, cleared []string) (ActivateResult, error) {
	return s.applyProfile(ctx, uid, startIfStopped, false, cleared)
}

func (s *CoreService) UseBasicConfig(ctx context.Context) (ActivateResult, error) {
	return s.applyProfile(ctx, "", false, true, nil)
}

func (s *CoreService) applyProfile(ctx context.Context, uid string, startIfStopped, basic bool, cleared []string) (ActivateResult, error) {
	var result ActivateResult
	action := "profile"
	if basic {
		action = "basic-config"
	}
	if forwarded, err := s.forwardControlRequest(ctx, coreControlRequest{Action: action, Version: uid, Cleared: cleared}, &result, nil); forwarded {
		return result, err
	}
	if startIfStopped && s.usesService() {
		if err := s.Start(ctx); err != nil {
			return ActivateResult{}, err
		}
		forwarded, err := s.forwardControlRequest(ctx, coreControlRequest{Action: "profile", Version: uid, Cleared: cleared}, &result, nil)
		if !forwarded && err == nil {
			err = errors.New("background core owner disappeared before applying the subscription")
		}
		return result, err
	}
	ctx, cancelOperation := s.operationContext(ctx)
	defer cancelOperation()
	if !s.mu.TryLock() {
		return ActivateResult{}, errors.New("another core operation is active")
	}
	defer s.mu.Unlock()
	_, err := loadCoreConfig()
	if err != nil {
		return ActivateResult{}, err
	}
	root, err := config.GetConfigDir()
	if err != nil {
		return ActivateResult{}, err
	}
	path := filepath.Join(root, "runtime", "config.yaml")
	if basic {
		if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return ActivateResult{}, err
		}
	}
	unlock, err := coreinfra.LockRuntime(filepath.Join(root, "runtime", "config.lock"))
	if err != nil {
		return ActivateResult{}, err
	}
	defer unlock()
	if err = recoverManagedProfile(root); err != nil {
		return ActivateResult{}, err
	}
	cfg, err := loadCoreConfig()
	if err != nil {
		return ActivateResult{}, err
	}
	if basic && cfg.Core.Mode != "managed" {
		return ActivateResult{}, errors.New("a basic configuration requires a managed core")
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
	if basic && os.IsNotExist(err) {
		if s.process != nil {
			return ActivateResult{}, errors.New("managed configuration is missing; restart the core to create a basic configuration")
		}
		_, err := prepareMissingManagedConfig(path)
		return ActivateResult{}, err
	}
	if err != nil {
		return ActivateResult{}, err
	}
	var generated profile.GeneratedConfig
	if basic {
		data, e := managedBaseConfig()
		if e != nil {
			return ActivateResult{}, e
		}
		var base yaml.Node
		if err = yaml.Unmarshal(data, &base); err == nil {
			generated.Data, err = profile.GenerateYAMLWithPreserve(&base, nil, path)
		}
	} else {
		generated, err = profile.GenerateForUIDWithCleared(uid, path, cleared, true)
	}
	if err != nil {
		return ActivateResult{}, err
	}
	data, err := sanitizeManagedInbound(generated.Data)
	if err != nil {
		return ActivateResult{}, err
	}
	data, err = applyManagedNetwork(data, cfg.Core.TUN, root)
	if err != nil {
		return ActivateResult{}, err
	}
	candidate, err := os.CreateTemp(filepath.Dir(path), ".candidate-*.yaml")
	if err != nil {
		return ActivateResult{}, err
	}
	defer func() { _ = os.Remove(candidate.Name()) }()
	if _, err = candidate.Write(data); err != nil {
		_ = candidate.Close()
		return ActivateResult{}, err
	}
	if err = candidate.Close(); err != nil {
		return ActivateResult{}, err
	}
	binary, err := coreinfra.BinaryPath(root, cfg.Core.Version, runtime.GOOS)
	if err != nil {
		return ActivateResult{}, err
	}
	if err = s.validateConfig(ctx, cfg, binary, path, candidate.Name(), data); err != nil {
		return ActivateResult{}, err
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
	recovery := profileRecovery{Config: old, ActiveSub: cfg.ActiveSub}
	journal := filepath.Join(root, "runtime", "profile-recovery.json")
	running := s.process != nil
	if running && controllerIdentityChanged(old, data) {
		updateActiveSub := func() error {
			return config.Update(func(latest *config.Config) error {
				if basic {
					latest.ActiveSub = ""
					return nil
				}
				for _, p := range latest.Subs {
					if p.UID == uid {
						latest.ActiveSub = uid
						return nil
					}
				}
				return ErrSubNotFound
			})
		}
		change := &managedConfigReplacement{
			root:      root,
			path:      path,
			recovery:  recovery,
			candidate: data,
			afterSave: updateActiveSub,
		}
		if err = s.replaceCore(ctx, change); err != nil {
			return ActivateResult{}, err
		}
		return ActivateResult{BackupName: backup, MergeErr: generated.MergeErr, Restarted: true}, nil
	}
	record, _ := json.Marshal(recovery)
	if err = privateWrite(journal, record); err != nil {
		return ActivateResult{}, err
	}
	endpoint := config.ResolveMihomoEndpoint()
	client := api.NewClient(endpoint, 1000)
	started := false
	rollback := func(cause error) (ActivateResult, error) {
		var stopErr error
		if started {
			stopErr = s.stopLocked()
		}
		restoreErr := recoverManagedProfile(root)
		var reloadErr error
		if restoreErr == nil && running {
			reloadErr = client.ReloadConfig(path)
			if reloadErr == nil {
				reloadErr = s.refreshNetworkTUN(context.Background(), root)
			}
		}
		return ActivateResult{}, errors.Join(cause, stopErr, restoreErr, reloadErr)
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
		if err = s.refreshNetworkTUN(ctx, root); err != nil {
			return rollback(err)
		}
	} else if startIfStopped {
		if err = s.start(ctx, true); err != nil {
			return rollback(err)
		}
		started = true
	}
	if err = config.Update(func(latest *config.Config) error {
		if basic {
			latest.ActiveSub = ""
			return nil
		}
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
	return ActivateResult{BackupName: backup, MergeErr: generated.MergeErr}, nil
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
