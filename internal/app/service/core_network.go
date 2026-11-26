package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/host"
	"gopkg.in/yaml.v3"
)

type networkRecovery struct {
	Before  host.ProxySettings `json:"before,omitempty"`
	Applied host.ProxySettings `json:"applied,omitempty"`
	TUN     *host.TUN          `json:"tun,omitempty"`
}

func networkJournal(root string) string {
	return filepath.Join(root, "runtime", "network-recovery.json")
}
func writeNetwork(root string, record networkRecovery) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return privateWrite(networkJournal(root), data)
}
func (s *CoreService) readNetwork(root string) (networkRecovery, error) {
	var record networkRecovery
	data, err := os.ReadFile(networkJournal(root))
	if err != nil {
		return record, err
	}
	if len(data) > 128<<10 || json.Unmarshal(data, &record) != nil {
		return record, errors.New("invalid network recovery record")
	}
	if record.TUN != nil {
		expected := host.TUNIdentity(root)
		if record.TUN.Device != expected.Device || record.TUN.Table != expected.Table || record.TUN.Rule != expected.Rule || record.TUN.Index < 0 {
			return record, errors.New("invalid TUN recovery identity")
		}
	}
	allowed := s.network.ProxyTarget(7890)
	if len(record.Before) != len(record.Applied) {
		return record, errors.New("invalid proxy recovery record")
	}
	for key := range record.Before {
		if _, ok := allowed[key]; !ok {
			return record, errors.New("invalid proxy recovery key")
		}
		if _, ok := record.Applied[key]; !ok {
			return record, errors.New("invalid proxy recovery value")
		}
	}
	return record, nil
}
func (s *CoreService) recoverNetworkLocked(ctx context.Context, root string) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	unlock, err := coreinfra.LockRuntime(filepath.Join(root, "runtime", "network.lock"))
	if err != nil {
		return err
	}
	defer unlock()
	record, err := s.readNetwork(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var proxyErr, tunErr error
	if len(record.Applied) > 0 {
		current, e := s.network.ReadProxy(ctx)
		proxyErr = e
		if e == nil {
			restore := host.ProxySettings{}
			for key, value := range record.Applied {
				if reflect.DeepEqual(current[key], value) {
					restore[key] = record.Before[key]
				}
			}
			proxyErr = s.network.WriteProxy(ctx, restore)
		}
	}
	if record.TUN != nil {
		tunErr = s.network.CleanupTUN(ctx, *record.TUN)
	}
	if err = errors.Join(proxyErr, tunErr); err != nil {
		return err
	}
	return os.Remove(networkJournal(root))
}
func applyManagedNetwork(data []byte, enabled bool, root string) ([]byte, error) {
	var mapping map[string]any
	if err := yaml.Unmarshal(data, &mapping); err != nil {
		return nil, err
	}
	if mapping == nil {
		mapping = make(map[string]any)
	}
	mapping["tun"] = map[string]any{"enable": false}
	if enabled {
		id := host.TUNIdentity(root)
		mapping["tun"] = map[string]any{"enable": true, "device": id.Device, "stack": "system", "auto-route": true, "auto-detect-interface": true, "auto-redirect": false, "strict-route": false, "dns-hijack": []string{"any:53", "tcp://any:53"}, "iproute2-table-index": id.Table, "iproute2-rule-index": id.Rule}
		dns, _ := mapping["dns"].(map[string]any)
		if dns == nil {
			dns = map[string]any{}
		}
		dns["enable"] = true
		delete(dns, "listen")
		if nameservers, ok := dns["nameserver"].([]any); !ok || len(nameservers) == 0 {
			dns["nameserver"] = []string{"https://1.1.1.1/dns-query", "https://dns.alidns.com/dns-query"}
			dns["default-nameserver"] = []string{"1.1.1.1", "223.5.5.5"}
		}
		mapping["dns"] = dns
	}
	return yaml.Marshal(mapping)
}
func (s *CoreService) prepareNetwork(ctx context.Context, root string, cfg *config.Config, binary string) error {

	if !cfg.Core.TUN && !cfg.Core.SystemProxy {
		return nil
	}
	record := networkRecovery{}
	if cfg.Core.TUN {
		identity := host.TUNIdentity(root)
		if err := s.network.CheckTUN(ctx, binary, identity); err != nil {
			return err
		}
		record.TUN = &identity
	}
	if cfg.Core.SystemProxy {
		if _, err := s.network.ReadProxy(ctx); err != nil {
			return err
		}
	}
	if err := writeNetwork(root, record); err != nil {
		return err
	}
	if err := s.startGuard(root); err != nil {
		_ = os.Remove(networkJournal(root))
		return err
	}
	return nil
}
func (s *CoreService) activateNetwork(ctx context.Context, root string, cfg *config.Config) error {
	if !cfg.Core.TUN && !cfg.Core.SystemProxy {
		return nil
	}
	record, err := s.readNetwork(root)
	if err != nil {
		return err
	}
	if cfg.Core.TUN {
		identity, e := s.network.CaptureTUN(ctx, *record.TUN)
		if e != nil {
			return e
		}
		record.TUN = &identity
	}
	if cfg.Core.SystemProxy {
		record.Before, err = s.network.ReadProxy(ctx)
		if err != nil {
			return err
		}
		record.Applied = s.network.ProxyTarget(config.ResolveMihomoEndpoint().MixedPort)
	}
	if err = writeNetwork(root, record); err != nil {
		return err
	}
	if cfg.Core.SystemProxy {
		return s.network.WriteProxy(ctx, record.Applied)
	}
	return nil
}

func (s *CoreService) refreshNetworkTUN(ctx context.Context, root string) error {
	cfg, err := loadCoreConfig()
	if err != nil || !cfg.Core.TUN {
		return err
	}
	record, err := s.readNetwork(root)
	if err != nil {
		return err
	}
	if record.TUN == nil {
		return errors.New("running TUN has no recovery identity")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for {
		identity, e := s.network.CaptureTUN(ctx, *record.TUN)
		if e == nil {
			record.TUN = &identity
			return writeNetwork(root, record)
		}
		select {
		case <-ctx.Done():
			return errors.New("TUN did not become ready after profile reload")
		case <-time.After(100 * time.Millisecond):
		}
	}
}
func (s *CoreService) SetSystemProxy(ctx context.Context, enabled bool) error {
	if runtime.GOOS != "linux" {
		return errors.New("system proxy integration requires Linux/GNOME")
	}

	value := "off"
	if enabled {
		value = "on"
	}
	if forwarded, err := s.forward(ctx, "system-proxy", value); forwarded {
		return err
	}
	if !s.mu.TryLock() {
		return errors.New("another core operation is active")
	}
	defer s.mu.Unlock()
	cfg, err := loadCoreConfig()
	if err != nil {
		return err
	}
	if cfg.Core.Mode != "managed" {
		return errors.New("system proxy requires a managed core")
	}
	root, err := config.GetConfigDir()
	if err != nil {
		return err
	}
	unlock, err := s.lockCoreMutation(root)
	if err != nil {
		return err
	}
	defer unlock()
	if cfg.Core.SystemProxy == enabled {
		return nil
	}
	if enabled {
		if _, err = s.network.ReadProxy(ctx); err != nil {
			return err
		}
	}
	var prevRecord networkRecovery
	var hadPrevRecord bool
	if s.process != nil {
		if enabled {
			if err = s.startGuard(root); err != nil {
				return err
			}
			record, e := s.readNetwork(root)
			if e != nil && !os.IsNotExist(e) {
				return e
			}
			record.Before, err = s.network.ReadProxy(ctx)
			if err != nil {
				return err
			}
			record.Applied = s.network.ProxyTarget(config.ResolveMihomoEndpoint().MixedPort)
			if err = writeNetwork(root, record); err != nil {
				return err
			}
			if err = s.network.WriteProxy(ctx, record.Applied); err != nil {
				rbCtx, rbCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer rbCancel()
				return errors.Join(err, s.restoreProxyOnly(rbCtx, root))
			}
		} else {
			if rec, e := s.readNetwork(root); e == nil && len(rec.Applied) > 0 {
				prevRecord = rec
				hadPrevRecord = true
			}
			if err = s.restoreProxyOnly(ctx, root); err != nil {
				return err
			}
		}
	}
	if err = config.Update(func(latest *config.Config) error { latest.Core.SystemProxy = enabled; return nil }); err != nil {
		rbCtx, rbCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer rbCancel()
		if enabled {
			return errors.Join(err, s.restoreProxyOnly(rbCtx, root))
		}
		if hadPrevRecord {
			var reApplyErr error
			if wErr := writeNetwork(root, prevRecord); wErr != nil {
				reApplyErr = wErr
			} else {
				reApplyErr = s.network.WriteProxy(rbCtx, prevRecord.Applied)
			}
			return errors.Join(err, reApplyErr)
		}
		return err
	}
	return nil
}
func (s *CoreService) restoreProxyOnly(ctx context.Context, root string) error {
	record, err := s.readNetwork(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(record.Applied) == 0 {
		return nil
	}
	current, err := s.network.ReadProxy(ctx)
	if err != nil {
		return err
	}
	restore := host.ProxySettings{}
	for key, value := range record.Applied {
		if reflect.DeepEqual(current[key], value) {
			restore[key] = record.Before[key]
		}
	}
	if err = s.network.WriteProxy(ctx, restore); err != nil {
		return err
	}
	record.Before = nil
	record.Applied = nil
	if record.TUN == nil {
		return os.Remove(networkJournal(root))
	}
	return writeNetwork(root, record)
}

type networkChange struct {
	Config []byte `json:"config"`
	TUN    bool   `json:"tun"`
}

func recoverNetworkChange(root string) error {
	path := filepath.Join(root, "runtime", "network-change.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var previous networkChange
	if json.Unmarshal(data, &previous) != nil {
		return errors.New("invalid network change journal")
	}
	if err = privateWrite(filepath.Join(root, "runtime", "config.yaml"), previous.Config); err != nil {
		return err
	}
	if err = config.Update(func(cfg *config.Config) error { cfg.Core.TUN = previous.TUN; return nil }); err != nil {
		return err
	}
	return os.Remove(path)
}
func (s *CoreService) SetTUN(ctx context.Context, enabled bool) error {
	if runtime.GOOS != "linux" {
		return errors.New("managed TUN requires Linux")
	}
	value := "off"
	if enabled {
		value = "on"
	}
	if forwarded, err := s.forward(ctx, "tun", value); forwarded {
		return err
	}
	if !s.mu.TryLock() {
		return errors.New("another core operation is active")
	}
	defer s.mu.Unlock()
	cfg, err := loadCoreConfig()
	if err != nil {
		return err
	}
	if cfg.Core.Mode != "managed" {
		return errors.New("TUN requires a managed core")
	}
	root, err := config.GetConfigDir()
	if err != nil {
		return err
	}
	unlock, err := s.lockCoreMutation(root)
	if err != nil {
		return err
	}
	defer unlock()
	if err = recoverNetworkChange(root); err != nil {
		return err
	}
	cfg, err = loadCoreConfig()
	if err != nil {
		return err
	}
	if cfg.Core.TUN == enabled {
		return nil
	}
	if enabled && s.process != nil {
		binary, err := coreinfra.BinaryPath(root, cfg.Core.Version, runtime.GOOS)
		if err != nil {
			return err
		}
		if err = s.network.CheckTUN(ctx, binary, host.TUNIdentity(root)); err != nil {
			return err
		}
	}
	path := filepath.Join(root, "runtime", "config.yaml")
	old, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	candidate, err := applyManagedNetwork(old, enabled, root)
	if err != nil {
		return err
	}
	if err = validateNetworkCandidate(ctx, root, cfg.Core.Version, candidate); err != nil {
		return err
	}
	return s.replaceCore(ctx, &tunReplacement{root: root, previous: networkChange{Config: old, TUN: cfg.Core.TUN}, candidate: candidate, enabled: enabled})
}

type tunReplacement struct {
	root      string
	previous  networkChange
	candidate []byte
	enabled   bool
	journaled bool
}

func (c *tunReplacement) apply(_ bool) error {
	record, err := json.Marshal(c.previous)
	if err != nil {
		return err
	}
	if err = privateWrite(filepath.Join(c.root, "runtime", "network-change.json"), record); err != nil {
		return err
	}
	c.journaled = true
	if err = privateWrite(filepath.Join(c.root, "runtime", "config.yaml"), c.candidate); err != nil {
		return err
	}
	return config.Update(func(latest *config.Config) error { latest.Core.TUN = c.enabled; return nil })
}

func (c *tunReplacement) commit() error {
	return os.Remove(filepath.Join(c.root, "runtime", "network-change.json"))
}

func (c *tunReplacement) restore() error {
	if !c.journaled {
		return nil
	}
	return recoverNetworkChange(c.root)
}

func validateNetworkCandidate(ctx context.Context, root, version string, data []byte) error {
	binary, err := coreinfra.BinaryPath(root, version, runtime.GOOS)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Join(root, "runtime"), ".network-candidate-*.yaml")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(file.Name()) }()
	if _, err = file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "-t", "-d", filepath.Join(root, "runtime"), "-f", file.Name())
	coreinfra.ConfigureProcess(command)
	if err = command.Run(); err != nil {
		return errors.New("candidate network configuration failed core validation")
	}
	return nil
}

func startNetworkGuard(root string, service bool) error {
	var bytes [24]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return err
	}
	nonce := hex.EncodeToString(bytes[:])
	challenge := filepath.Join(root, "runtime", "guard-challenge")
	ready := filepath.Join(root, "runtime", "guard-ready")
	if err := privateWrite(challenge, []byte(nonce)); err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := host.GuardCommand(executable, root, service)
	if err = cmd.Start(); err != nil {
		return errors.New("cannot start independent network recovery guard")
	}
	go func() { _ = cmd.Wait() }()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline.C:
			_ = cmd.Process.Kill()
			return errors.New("network recovery guard did not become ready")
		case <-ticker.C:
			data, e := os.ReadFile(ready)
			if e == nil && string(data) == nonce {
				return nil
			}
		}
	}
}
func (s *CoreService) GuardNetwork(ctx context.Context) error {
	root, err := config.GetConfigDir()
	if err != nil {
		return err
	}
	unlock, err := coreinfra.LockRuntime(filepath.Join(root, "runtime", "guard.lock"))
	if err != nil {
		return nil
	}
	defer unlock()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	lastChallenge := ""
	for {
		if data, e := os.ReadFile(filepath.Join(root, "runtime", "guard-challenge")); e == nil {
			if string(data) != lastChallenge {
				if e = privateWrite(filepath.Join(root, "runtime", "guard-ready"), data); e == nil {
					lastChallenge = string(data)
				}
			}
		}
		if err = s.RecoverNetwork(ctx); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (s *CoreService) disableRunningTUN() {
	cfg, err := loadCoreConfig()
	if err == nil && cfg.Core.TUN && s.process != nil {
		_, _ = api.NewClient(s.endpoint, 1500).DoRequest("PATCH", "/configs", map[string]any{"tun": map[string]any{"enable": false}})
	}
}
