package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"gopkg.in/yaml.v3"
)

// MihomoOverridesSaveResult reports the real persistence and live application outcome
// after writing the five Mihomo override fields to merge.yaml.
type MihomoOverridesSaveResult struct {
	Overrides          map[string]string
	WriteOK            bool
	Applied            bool
	Restarted          bool
	NeedsRestart       bool
	EndpointChanged    bool
	Remote             bool
	RemoteNotPersisted bool
	Restored           bool
	BackupName         string
	Err                error
}

// ReadMihomoOverrides reads the five editable override fields from merge.yaml.
func (s *ConfigService) ReadMihomoOverrides() (map[string]string, error) {
	return profile.ReadMihomoOverrides()
}

// ApplyMihomoOverrides persists the five overrides to merge.yaml and regenerates/applies
// the active configuration across managed, local external, and remote-only controllers.
func (s *ConfigService) ApplyMihomoOverrides(ctx context.Context, coreSvc *CoreService, client *api.Client, overrides map[string]string) MihomoOverridesSaveResult {
	return s.applyMergeMutation(ctx, coreSvc, client, mergeMutation{overrides: overrides})
}

func (s *ConfigService) ApplyRuleEdit(ctx context.Context, coreSvc *CoreService, client *api.Client, edit profile.RuleEdit) MihomoOverridesSaveResult {
	return s.applyMergeMutation(ctx, coreSvc, client, mergeMutation{rule: &edit})
}

// ReadProxyChains reads persisted proxy chain definitions from chain-edits.yaml.
func (s *ConfigService) ReadProxyChains() (profile.ChainEdits, error) {
	return profile.ReadChainEdits()
}

// ApplyProxyChain persists a group's proxy chain to chain-edits.yaml, regenerates and
// reloads the active configuration, and switches the target group to the chain's exit node.
func (s *ConfigService) ApplyProxyChain(ctx context.Context, coreSvc *CoreService, client *api.Client, chain profile.ProxyChain) MihomoOverridesSaveResult {
	res := s.applyMergeMutation(ctx, coreSvc, client, mergeMutation{chain: &chain})
	if res.Err != nil || !res.Applied || !chain.Enabled || len(chain.Nodes) < 2 || strings.TrimSpace(chain.Group) == "" {
		return res
	}
	targetClient := client
	if targetClient == nil {
		targetClient = api.NewClient(config.ResolveMihomoEndpoint(), 2000)
	}
	exitNode := strings.TrimSpace(chain.Nodes[len(chain.Nodes)-1])
	_ = targetClient.SelectProxy(strings.TrimSpace(chain.Group), exitNode)
	return res
}

func (s *ConfigService) applyMergeMutation(ctx context.Context, coreSvc *CoreService, client *api.Client, change mergeMutation) MihomoOverridesSaveResult {
	if ctx == nil {
		ctx = context.Background()
	}
	s.mergeMu.Lock()
	defer s.mergeMu.Unlock()
	cfg, err := loadCoreConfig()
	if err != nil {
		return MihomoOverridesSaveResult{Err: err}
	}
	if cfg.Core.Mode == "managed" {
		svc := coreSvc
		if svc == nil {
			svc = NewCoreService()
			defer svc.Close()
		}
		res, applyErr := svc.applyMergeMutation(ctx, change)
		if applyErr != nil && res.Err == nil {
			res.Err = applyErr
		}
		return res
	}
	if client != nil {
		client = client.Snapshot()
	}
	if cfg.Core.ExternalController != "" {
		return s.applyRemoteExternalOverrides(cfg, client, change)
	}
	return s.applyLocalExternalOverrides(cfg, client, change)
}

// ApplyOverrides updates merge.yaml and applies regenerated configuration for a managed core,
// forwarding to any live background core owner and rolling back on validation/reload/restart failure.
func (s *CoreService) ApplyOverrides(ctx context.Context, overrides map[string]string) (MihomoOverridesSaveResult, error) {
	return s.applyMergeMutation(ctx, mergeMutation{overrides: overrides})
}

func (s *CoreService) applyMergeMutation(ctx context.Context, mutation mergeMutation) (MihomoOverridesSaveResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var forwardedRes MihomoOverridesSaveResult
	if forwarded, err := s.forwardControlRequest(ctx, mutation.controlRequest(), nil, &forwardedRes); forwarded {
		if err != nil && forwardedRes.Err == nil {
			forwardedRes.Err = err
		}
		return forwardedRes, err
	}
	ctx, cancelOperation := s.operationContext(ctx)
	defer cancelOperation()
	if !s.mu.TryLock() {
		err := errors.New("another core operation is active")
		return MihomoOverridesSaveResult{Err: err}, err
	}
	defer s.mu.Unlock()

	root, err := config.GetConfigDir()
	if err != nil {
		return MihomoOverridesSaveResult{Err: err}, err
	}
	path := filepath.Join(root, "runtime", "config.yaml")
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return MihomoOverridesSaveResult{Err: err}, err
	}
	unlock, err := coreinfra.LockRuntime(filepath.Join(root, "runtime", "config.lock"))
	if err != nil {
		return MihomoOverridesSaveResult{Err: err}, err
	}
	defer unlock()
	if err = recoverManagedProfile(root); err != nil {
		return MihomoOverridesSaveResult{Err: err}, err
	}
	cfg, err := loadCoreConfig()
	if err != nil {
		return MihomoOverridesSaveResult{Err: err}, err
	}
	if cfg.Core.Mode != "managed" {
		err = errors.New("managed core mode is not enabled")
		return MihomoOverridesSaveResult{Err: err}, err
	}
	if s.process == nil {
		unlockRuntime, e := coreinfra.LockRuntime(filepath.Join(root, "runtime", "process.lock"))
		if e != nil {
			return MihomoOverridesSaveResult{Err: e}, e
		}
		unlockRuntime()
	}
	oldConfig, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if s.process != nil {
			err = errors.New("managed configuration is missing; restart the core to create a basic configuration")
			return MihomoOverridesSaveResult{Err: err}, err
		}
		if _, err = prepareMissingManagedConfig(path); err != nil {
			return MihomoOverridesSaveResult{Err: err}, err
		}
		oldConfig, err = os.ReadFile(path)
	}
	if err != nil {
		return MihomoOverridesSaveResult{Err: err}, err
	}

	mergePath, err := profile.MergePath()
	if err != nil {
		return MihomoOverridesSaveResult{Err: err}, err
	}
	oldMerge, mergeExisted, err := readOptionalFile(mergePath)
	if err != nil {
		return MihomoOverridesSaveResult{Err: err}, err
	}
	uid := activeSubUID(cfg)
	next, err := mutation.prepare(oldMerge, uid, path, true)
	if err != nil {
		return MihomoOverridesSaveResult{Err: err}, err
	}
	savedOverrides := next.overrides
	generated, err := next.generate(uid, path, true)
	if err != nil {
		return MihomoOverridesSaveResult{Err: err}, err
	}
	data, err := sanitizeManagedInbound(generated)
	if err != nil {
		return MihomoOverridesSaveResult{Err: err}, err
	}
	data, err = applyManagedNetwork(data, cfg.Core.TUN, root)
	if err != nil {
		return MihomoOverridesSaveResult{Err: err}, err
	}
	candidate, err := os.CreateTemp(filepath.Dir(path), ".candidate-*.yaml")
	if err != nil {
		return MihomoOverridesSaveResult{Err: err}, err
	}
	defer func() { _ = os.Remove(candidate.Name()) }()
	if _, err = candidate.Write(data); err != nil {
		_ = candidate.Close()
		return MihomoOverridesSaveResult{Err: err}, err
	}
	if err = candidate.Close(); err != nil {
		return MihomoOverridesSaveResult{Err: err}, err
	}
	binary, err := coreinfra.BinaryPath(root, cfg.Core.Version, runtime.GOOS)
	if err != nil {
		return MihomoOverridesSaveResult{Err: err}, err
	}
	if err = s.validateConfig(ctx, cfg, binary, path, candidate.Name(), data); err != nil {
		return MihomoOverridesSaveResult{Err: err}, err
	}
	if err = ctx.Err(); err != nil {
		return MihomoOverridesSaveResult{Err: err}, err
	}

	backupDir := filepath.Join(root, "config-backups")
	if err = os.MkdirAll(backupDir, 0700); err != nil {
		return MihomoOverridesSaveResult{Err: err}, err
	}
	if err = os.Chmod(backupDir, 0700); err != nil {
		return MihomoOverridesSaveResult{Err: err}, err
	}
	backup, err := profile.BackupConfig(path)
	if err != nil {
		return MihomoOverridesSaveResult{Err: err}, err
	}
	if backup != "" {
		if err = os.Chmod(filepath.Join(backupDir, backup), 0600); err != nil {
			return MihomoOverridesSaveResult{Err: err}, err
		}
	}

	identityChanged := controllerIdentityChanged(oldConfig, data)
	portChanged := mixedPortChanged(oldConfig, data)
	recovery := profileRecovery{
		Config:     oldConfig,
		ActiveSub:  cfg.ActiveSub,
		RuleEdits:  next.ruleEdits,
		ChainEdits: next.chainEdits,
	}
	if next.ruleEdits == nil && next.chainEdits == nil {
		recovery.HasMerge, recovery.MergeExisted, recovery.Merge = true, mergeExisted, oldMerge
	}
	journal := filepath.Join(root, "runtime", "profile-recovery.json")
	running := s.process != nil

	// Mihomo hot-reload cannot rebuild external-controller or change secret;
	// SystemProxy also needs a controlled restart when mixed-port changes.
	if running && (identityChanged || (portChanged && cfg.Core.SystemProxy)) {
		change := &managedConfigReplacement{
			root:           root,
			path:           path,
			recovery:       recovery,
			candidate:      data,
			writeOverrides: next.write,
		}
		if err = s.replaceCore(ctx, change); err != nil {
			return MihomoOverridesSaveResult{Restored: change.restored, Err: err}, err
		}
		return MihomoOverridesSaveResult{
			Overrides:       savedOverrides,
			WriteOK:         true,
			Applied:         true,
			Restarted:       true,
			EndpointChanged: true,
			BackupName:      backup,
		}, nil
	}

	record, _ := json.Marshal(recovery)
	if err = privateWrite(journal, record); err != nil {
		return MihomoOverridesSaveResult{Err: err}, err
	}
	endpoint := config.ResolveMihomoEndpoint()
	client := api.NewClient(endpoint, 1000)
	rollback := func(cause error) (MihomoOverridesSaveResult, error) {
		restoreErr := recoverManagedProfile(root)
		var reloadErr error
		if restoreErr == nil && running {
			reloadErr = client.ReloadConfig(path)
			if reloadErr == nil {
				reloadErr = s.refreshNetworkTUN(context.Background(), root)
			}
		}
		joined := errors.Join(cause, restoreErr, reloadErr)
		return MihomoOverridesSaveResult{Restored: restoreErr == nil, Err: joined}, joined
	}
	if err = next.write(); err != nil {
		return rollback(err)
	}
	if err = privateWrite(path, data); err != nil {
		return rollback(err)
	}
	if running {
		if err = client.ReloadConfig(path); err != nil {
			return rollback(fmt.Errorf("managed override reload failed: %w", err))
		}
		if _, err = client.GetVersion(); err != nil {
			return rollback(errors.New("core did not become ready after override reload"))
		}
		if err = s.refreshNetworkTUN(ctx, root); err != nil {
			return rollback(err)
		}
	}
	if err = os.Remove(journal); err != nil {
		return rollback(err)
	}
	return MihomoOverridesSaveResult{
		Overrides:       savedOverrides,
		WriteOK:         true,
		Applied:         running,
		EndpointChanged: identityChanged || portChanged,
		BackupName:      backup,
	}, nil
}

func (s *ConfigService) applyLocalExternalOverrides(cfg *config.Config, client *api.Client, mutation mergeMutation) MihomoOverridesSaveResult {
	uid := activeSubUID(cfg)
	pathFn := s.localExternalPath
	if pathFn == nil {
		pathFn = config.GetMihomoConfigPath
	}
	path, err := pathFn()
	if err != nil {
		if uid != "" && s.mihomoConfigPath != nil {
			path, err = s.mihomoConfigPath()
		}
		if err != nil {
			return MihomoOverridesSaveResult{Err: err}
		}
	}

	mergePath, err := profile.MergePath()
	if err != nil {
		return MihomoOverridesSaveResult{Err: err}
	}
	oldMerge, mergeExisted, err := readOptionalFile(mergePath)
	if err != nil {
		return MihomoOverridesSaveResult{Err: err}
	}
	next, err := mutation.prepare(oldMerge, uid, path, false)
	if err != nil {
		return MihomoOverridesSaveResult{Err: err}
	}
	savedOverrides := next.overrides
	oldConfig, configExisted, err := readOptionalFile(path)
	if err != nil {
		return MihomoOverridesSaveResult{Err: err}
	}
	if uid == "" && !configExisted {
		return MihomoOverridesSaveResult{Err: os.ErrNotExist}
	}
	data, err := next.generate(uid, path, false)
	if err != nil {
		return MihomoOverridesSaveResult{Err: err}
	}

	identityChanged := localControllerIdentityChanged(oldConfig, data, client)
	portChanged := configExisted && mixedPortChanged(oldConfig, data)

	var backup string
	if configExisted {
		backup, err = profile.BackupConfig(path)
		if err != nil {
			return MihomoOverridesSaveResult{Err: fmt.Errorf("backup external config failed: %w", err)}
		}
	}
	restoreAll := func(cause error) MihomoOverridesSaveResult {
		restoreCfgErr := restoreConfigFile(path, configExisted, oldConfig)
		restoreMergeErr := next.restore(mergePath, mergeExisted, oldMerge)
		joined := errors.Join(cause, restoreCfgErr, restoreMergeErr)
		return MihomoOverridesSaveResult{
			Restored: restoreCfgErr == nil && restoreMergeErr == nil,
			Err:      joined,
		}
	}

	if err = next.write(); err != nil {
		return MihomoOverridesSaveResult{Err: err}
	}
	if err = profile.WriteFinal(path, data); err != nil {
		restoreMergeErr := next.restore(mergePath, mergeExisted, oldMerge)
		return MihomoOverridesSaveResult{
			Restored: restoreMergeErr == nil,
			Err:      errors.Join(fmt.Errorf("write external config failed: %w", err), restoreMergeErr),
		}
	}

	if client == nil {
		return MihomoOverridesSaveResult{
			Overrides:       savedOverrides,
			WriteOK:         true,
			Applied:         false,
			NeedsRestart:    identityChanged,
			EndpointChanged: identityChanged || portChanged,
			BackupName:      backup,
		}
	}

	if identityChanged {
		if s.restartMihomo != nil && s.restartMihomo() == nil {
			return MihomoOverridesSaveResult{
				Overrides:       savedOverrides,
				WriteOK:         true,
				Applied:         true,
				Restarted:       true,
				EndpointChanged: true,
				BackupName:      backup,
			}
		}
		// Automatic service restart unavailable: hot-reload other fields if the old controller is reachable.
		reloadErr := client.ReloadConfig(path)
		if reloadErr != nil {
			var apiErr *api.APIError
			if errors.As(reloadErr, &apiErr) {
				return restoreAll(fmt.Errorf("external reload rejected configuration: %w", reloadErr))
			}
			// Core unreachable/stopped: keep written files so manual start/restart applies them.
			return MihomoOverridesSaveResult{
				Overrides:       savedOverrides,
				WriteOK:         true,
				Applied:         false,
				NeedsRestart:    true,
				EndpointChanged: true,
				BackupName:      backup,
			}
		}
		return MihomoOverridesSaveResult{
			Overrides:       savedOverrides,
			WriteOK:         true,
			Applied:         true,
			NeedsRestart:    true,
			EndpointChanged: true,
			BackupName:      backup,
		}
	}

	if reloadErr := client.ReloadConfig(path); reloadErr != nil {
		return restoreAll(fmt.Errorf("external reload failed: %w", reloadErr))
	}
	return MihomoOverridesSaveResult{
		Overrides:       savedOverrides,
		WriteOK:         true,
		Applied:         true,
		EndpointChanged: portChanged,
		BackupName:      backup,
	}
}

func (s *ConfigService) applyRemoteExternalOverrides(cfg *config.Config, client *api.Client, mutation mergeMutation) MihomoOverridesSaveResult {
	uid := activeSubUID(cfg)
	if mutation.rule != nil && uid == "" {
		return MihomoOverridesSaveResult{Remote: true, Err: ErrRemoteRuleEditRequiresSubscription}
	}
	if mutation.chain != nil && uid == "" {
		return MihomoOverridesSaveResult{Remote: true, Err: ErrRemoteChainEditRequiresSubscription}
	}
	mergePath, err := profile.MergePath()
	if err != nil {
		return MihomoOverridesSaveResult{Err: err}
	}
	oldMerge, mergeExisted, err := readOptionalFile(mergePath)
	if err != nil {
		return MihomoOverridesSaveResult{Err: err}
	}
	next, err := mutation.prepare(oldMerge, uid, "", false)
	if err != nil {
		return MihomoOverridesSaveResult{Err: err}
	}
	savedOverrides := next.overrides
	identityPending := identityRequestedForRemote(savedOverrides, cfg.Core.ExternalController, cfg.Core.ExternalSecret)

	if uid != "" {
		data, genErr := next.generate(uid, "", false)
		if genErr != nil {
			return MihomoOverridesSaveResult{Remote: true, Err: genErr}
		}
		if err = next.write(); err != nil {
			return MihomoOverridesSaveResult{Remote: true, Err: err}
		}
		if client == nil {
			return MihomoOverridesSaveResult{
				Overrides:    savedOverrides,
				WriteOK:      true,
				Applied:      false,
				Remote:       true,
				NeedsRestart: identityPending,
			}
		}
		if reloadErr := client.ReloadConfigPayload(data); reloadErr != nil {
			restoreErr := next.restore(mergePath, mergeExisted, oldMerge)
			return MihomoOverridesSaveResult{
				Remote:   true,
				Restored: restoreErr == nil,
				Err:      errors.Join(fmt.Errorf("remote payload reload failed: %w", reloadErr), restoreErr),
			}
		}
		return MihomoOverridesSaveResult{
			Overrides:          savedOverrides,
			WriteOK:            true,
			Applied:            true,
			Remote:             true,
			RemoteNotPersisted: true,
			NeedsRestart:       identityPending,
		}
	}

	// Remote-only controller without an active subscription: never send a partial
	// PUT /configs payload (which would wipe remote proxies/rules). Patch only
	// explicit runtime fields via PATCH /configs.
	patch := buildRuntimePatch(savedOverrides)
	hasClearedRuntime := false
	for _, key := range next.cleared {
		if key == "mixed-port" || key == "allow-lan" || key == "log-level" {
			hasClearedRuntime = true
			break
		}
	}
	if err = next.write(); err != nil {
		return MihomoOverridesSaveResult{Remote: true, Err: err}
	}
	if client == nil {
		return MihomoOverridesSaveResult{
			Overrides:    savedOverrides,
			WriteOK:      true,
			Applied:      false,
			Remote:       true,
			NeedsRestart: identityPending || hasClearedRuntime,
		}
	}
	if len(patch) == 0 {
		return MihomoOverridesSaveResult{
			Overrides:    savedOverrides,
			WriteOK:      true,
			Applied:      false,
			Remote:       true,
			NeedsRestart: true,
		}
	}
	if patchErr := client.PatchConfig(patch); patchErr != nil {
		restoreErr := next.restore(mergePath, mergeExisted, oldMerge)
		return MihomoOverridesSaveResult{
			Remote:   true,
			Restored: restoreErr == nil,
			Err:      errors.Join(fmt.Errorf("remote config patch failed: %w", patchErr), restoreErr),
		}
	}
	return MihomoOverridesSaveResult{
		Overrides:          savedOverrides,
		WriteOK:            true,
		Applied:            true,
		Remote:             true,
		RemoteNotPersisted: true,
		NeedsRestart:       identityPending || hasClearedRuntime,
	}
}

type managedConfigReplacement struct {
	root           string
	path           string
	recovery       profileRecovery
	candidate      []byte
	writeOverrides func() error
	afterSave      func() error
	journaled      bool
	restored       bool
}

func (c *managedConfigReplacement) apply(_ bool) error {
	record, err := json.Marshal(c.recovery)
	if err != nil {
		return err
	}
	if err = privateWrite(filepath.Join(c.root, "runtime", "profile-recovery.json"), record); err != nil {
		return err
	}
	c.journaled = true
	if c.writeOverrides != nil {
		if err = c.writeOverrides(); err != nil {
			return err
		}
	}
	if err = privateWrite(c.path, c.candidate); err != nil {
		return err
	}
	if c.afterSave != nil {
		return c.afterSave()
	}
	return nil
}

func (c *managedConfigReplacement) commit() error {
	return os.Remove(filepath.Join(c.root, "runtime", "profile-recovery.json"))
}

func (c *managedConfigReplacement) restore() error {
	if !c.journaled {
		return nil
	}
	err := recoverManagedProfile(c.root)
	c.restored = err == nil
	return err
}

func activeSubUID(cfg *config.Config) string {
	if cfg == nil || cfg.ActiveSub == "" {
		return ""
	}
	for _, p := range cfg.Subs {
		if p.UID == cfg.ActiveSub {
			return cfg.ActiveSub
		}
	}
	return ""
}

func readOptionalFile(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

func restoreMergeFile(mergePath string, existed bool, oldData []byte) error {
	if !existed {
		if err := os.Remove(mergePath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(mergePath), 0755); err != nil {
		return err
	}
	return privateWrite(mergePath, oldData)
}

func restoreConfigFile(path string, existed bool, oldData []byte) error {
	if !existed {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return profile.WriteFinal(path, oldData)
}

func controllerIdentityFromYAML(data []byte) (controller, secret string, mixedPort int) {
	if len(data) == 0 {
		return "", "", 0
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return "", "", 0
	}
	var mapping *yaml.Node
	if root.Kind == yaml.DocumentNode {
		for _, child := range root.Content {
			if child.Kind == yaml.MappingNode {
				mapping = child
				break
			}
		}
	} else if root.Kind == yaml.MappingNode {
		mapping = &root
	}
	if mapping == nil {
		return "", "", 0
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		k := mapping.Content[i]
		v := mapping.Content[i+1]
		if k.Kind != yaml.ScalarNode || v.ShortTag() == "!!null" {
			continue
		}
		switch k.Value {
		case "external-controller":
			_ = v.Decode(&controller)
			controller = strings.TrimSpace(controller)
		case "secret":
			_ = v.Decode(&secret)
		case "mixed-port":
			var p int
			if v.Decode(&p) == nil {
				mixedPort = p
			}
		}
	}
	return controller, secret, mixedPort
}

func controllerIdentityChanged(oldData, newData []byte) bool {
	oldEC, oldSecret, _ := controllerIdentityFromYAML(oldData)
	newEC, newSecret, _ := controllerIdentityFromYAML(newData)
	return oldEC != newEC || oldSecret != newSecret
}

func localControllerIdentityChanged(oldData, newData []byte, client *api.Client) bool {
	if controllerIdentityChanged(oldData, newData) {
		return true
	}
	if client == nil {
		return false
	}
	controller, secret, _ := controllerIdentityFromYAML(newData)
	current := client.EndpointSnapshot()
	// The file may already contain an override while the running core still needs a restart.
	return controller != "" && (normalizeControllerHostPort(controller) != normalizeControllerHostPort(current.ExternalController) ||
		secret != current.Secret)
}

func mixedPortChanged(oldData, newData []byte) bool {
	_, _, oldPort := controllerIdentityFromYAML(oldData)
	_, _, newPort := controllerIdentityFromYAML(newData)
	return oldPort != newPort
}

func normalizeControllerHostPort(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") {
		if u, err := url.Parse(raw); err == nil && u.Host != "" {
			return strings.ToLower(u.Host)
		}
	}
	return strings.ToLower(raw)
}

func identityRequestedForRemote(overrides map[string]string, remoteController, remoteSecret string) bool {
	if ec := strings.TrimSpace(overrides["external-controller"]); ec != "" {
		if normalizeControllerHostPort(ec) != normalizeControllerHostPort(remoteController) {
			return true
		}
	}
	if secret := overrides["secret"]; secret != "" && secret != remoteSecret {
		return true
	}
	return false
}

func buildRuntimePatch(overrides map[string]string) map[string]any {
	patch := make(map[string]any)
	if rawPort := strings.TrimSpace(overrides["mixed-port"]); rawPort != "" {
		if port, err := strconv.Atoi(rawPort); err == nil && port > 0 {
			patch["mixed-port"] = port
		}
	}
	if allowLan := strings.TrimSpace(overrides["allow-lan"]); allowLan == "true" || allowLan == "false" {
		patch["allow-lan"] = allowLan == "true"
	}
	if logLevel := strings.TrimSpace(overrides["log-level"]); logLevel != "" {
		patch["log-level"] = logLevel
	}
	return patch
}
