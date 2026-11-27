//go:build !windows

package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManagedOverridesApplyClearAndPreserveInboundProtection(t *testing.T) {
	svc, uid, _ := prepareManagedProfile(t)
	rawBefore, err := profile.ReadRawFile(uid)
	require.NoError(t, err)
	require.NoError(t, config.Update(func(cfg *config.Config) error { cfg.ActiveSub = uid; return nil }))
	legacy := []byte("prepend-rules:\n  - DOMAIN,keep.example,DIRECT\n")
	mergePath, err := profile.MergePath()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(mergePath, legacy, 0600))

	// 1. Apply explicit allow-lan: false, log-level: debug, mixed-port: 7895 while offline.
	cfgSvc := NewConfigService()
	res := cfgSvc.ApplyMihomoOverrides(context.Background(), svc, nil, map[string]string{
		"mixed-port": "7895",
		"allow-lan":  "false",
		"log-level":  "debug",
	})
	require.NoError(t, res.Err)
	require.True(t, res.WriteOK)
	require.False(t, res.Applied)
	require.Equal(t, "false", res.Overrides["allow-lan"])
	require.Equal(t, "debug", res.Overrides["log-level"])
	require.Equal(t, "7895", res.Overrides["mixed-port"])

	mergeData, err := profile.ReadMerge()
	require.NoError(t, err)
	assert.Contains(t, string(mergeData), "allow-lan: false")
	assert.Contains(t, string(mergeData), "log-level: debug")
	assert.Contains(t, string(mergeData), "mixed-port: 7895")
	assert.NotContains(t, string(mergeData), "prepend-rules:")
	assert.NotContains(t, string(mergeData), "rules:")
	ruleEditsPath, err := profile.RuleEditsPath()
	require.NoError(t, err)
	ruleEdits, err := os.ReadFile(ruleEditsPath)
	require.NoError(t, err)
	assert.Contains(t, string(ruleEdits), "DOMAIN,keep.example,DIRECT")
	backups, err := filepath.Glob(mergePath + ".legacy-*.bak")
	require.NoError(t, err)
	require.Len(t, backups, 1)
	saved, err := os.ReadFile(backups[0])
	require.NoError(t, err)
	require.Equal(t, legacy, saved)
	assert.NotContains(t, string(mergeData), "DOMAIN,keep.example,DIRECT")

	path, err := config.ManagedMihomoConfigPath()
	require.NoError(t, err)
	applied, err := config.ReadMihomoYAML(path)
	require.NoError(t, err)
	assert.Equal(t, 7895, applied["mixed-port"])
	assert.Equal(t, false, applied["allow-lan"])
	assert.Equal(t, "debug", applied["log-level"])
	assert.NotContains(t, applied, "listeners", "managed inbound sanitization must still run")
	assert.Equal(t, false, applied["tun"].(map[string]any)["enable"])

	// 2. Enable allow-lan: true explicitly, then clear allow-lan & log-level.
	res = cfgSvc.ApplyMihomoOverrides(context.Background(), svc, nil, map[string]string{
		"mixed-port": "7895",
		"allow-lan":  "true",
		"log-level":  "warning",
	})
	require.NoError(t, res.Err)
	applied, err = config.ReadMihomoYAML(path)
	require.NoError(t, err)
	assert.Equal(t, true, applied["allow-lan"])
	assert.Equal(t, "warning", applied["log-level"])

	res = cfgSvc.ApplyMihomoOverrides(context.Background(), svc, nil, map[string]string{
		"mixed-port": "7895",
		"allow-lan":  "",
		"log-level":  "",
	})
	require.NoError(t, res.Err)
	require.True(t, res.WriteOK)
	mergeData, err = profile.ReadMerge()
	require.NoError(t, err)
	assert.NotContains(t, string(mergeData), "allow-lan")
	assert.NotContains(t, string(mergeData), "log-level")
	assert.NotContains(t, string(mergeData), "DOMAIN,keep.example,DIRECT")

	applied, err = config.ReadMihomoYAML(path)
	require.NoError(t, err)
	assert.Equal(t, false, applied["allow-lan"], "cleared allow-lan in managed mode must revert to safe false")
	assert.NotContains(t, applied, "log-level", "cleared log-level must fall through to raw.yaml")

	// Subscription raw file remains read-only.
	rawAfter, err := profile.ReadRawFile(uid)
	require.NoError(t, err)
	assert.Equal(t, rawBefore, rawAfter)
}

func TestManagedOverridesValidationAndReloadFailureRollback(t *testing.T) {
	t.Run("validation failure leaves merge and runtime untouched", func(t *testing.T) {
		svc, _, oldCfg := prepareManagedProfile(t)
		root, err := config.GetConfigDir()
		require.NoError(t, err)
		binary, err := coreinfra.BinaryPath(root, "v1.2.3", runtime.GOOS)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(binary, []byte("#!/bin/sh\necho bad candidate >&2\nexit 1\n"), 0700))

		res := NewConfigService().ApplyMihomoOverrides(context.Background(), svc, nil, map[string]string{
			"log-level": "debug",
		})
		require.Error(t, res.Err)
		require.False(t, res.WriteOK)
		mergeData, err := profile.ReadMerge()
		require.NoError(t, err)
		assert.Nil(t, mergeData)
		path, err := config.ManagedMihomoConfigPath()
		require.NoError(t, err)
		afterCfg, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, oldCfg, afterCfg)
	})

	t.Run("running hot-reload failure restores both merge and runtime", func(t *testing.T) {
		svc, _, _ := prepareManagedProfile(t)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "reload rejected", http.StatusInternalServerError)
		}))
		defer server.Close()
		path, err := config.ManagedMihomoConfigPath()
		require.NoError(t, err)
		require.NoError(t, config.WriteMihomoField(path, "external-controller", strings.TrimPrefix(server.URL, "http://")))
		oldCfg, err := os.ReadFile(path)
		require.NoError(t, err)
		oldMerge := []byte("mode: rule\nlog-level: info\n")
		require.NoError(t, profile.WriteMerge(oldMerge))

		svc.process = &exec.Cmd{}
		res := NewConfigService().ApplyMihomoOverrides(context.Background(), svc, nil, map[string]string{
			"log-level": "debug",
		})
		require.Error(t, res.Err)
		require.False(t, res.WriteOK)
		require.True(t, res.Restored)

		afterCfg, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, oldCfg, afterCfg)
		afterMerge, err := profile.ReadMerge()
		require.NoError(t, err)
		assert.Equal(t, oldMerge, afterMerge)
	})

	t.Run("running controller identity change uses replaceCore and restores merge on failure", func(t *testing.T) {
		svc, _, _ := prepareManagedProfile(t)
		root, err := config.GetConfigDir()
		require.NoError(t, err)
		binary, err := coreinfra.BinaryPath(root, "v1.2.3", runtime.GOOS)
		require.NoError(t, err)
		// First invocation (-t validation) succeeds; startup validation fails to trigger replacement rollback.
		require.NoError(t, os.WriteFile(binary, []byte("#!/bin/sh\nif [ -f \"$0.validated\" ]; then exit 1; fi\ntouch \"$0.validated\"\nexit 0\n"), 0700))

		path, err := config.ManagedMihomoConfigPath()
		require.NoError(t, err)
		oldCfg, err := os.ReadFile(path)
		require.NoError(t, err)
		oldMerge := []byte("mode: rule\n")
		require.NoError(t, profile.WriteMerge(oldMerge))

		svc.process = &exec.Cmd{}
		svc.done = make(chan error, 1)
		svc.stop = func() { svc.done <- nil }

		res := NewConfigService().ApplyMihomoOverrides(context.Background(), svc, nil, map[string]string{
			"external-controller": "127.0.0.1:19099",
		})
		require.Error(t, res.Err)
		require.False(t, res.WriteOK)
		require.True(t, res.Restored)

		afterCfg, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, oldCfg, afterCfg)
		afterMerge, err := profile.ReadMerge()
		require.NoError(t, err)
		assert.Equal(t, oldMerge, afterMerge)
	})
}

func TestManagedOverridesForwardedToBackgroundOwner(t *testing.T) {
	owner, uid, _ := prepareManagedProfile(t)
	require.NoError(t, config.Update(func(cfg *config.Config) error { cfg.ActiveSub = uid; return nil }))
	root, err := config.GetConfigDir()
	require.NoError(t, err)
	owner.daemon = true
	require.NoError(t, owner.startControl(root))

	caller := NewCoreService()
	t.Cleanup(caller.Close)
	res, err := caller.ApplyOverrides(context.Background(), map[string]string{
		"allow-lan": "false",
		"log-level": "warning",
	})
	require.NoError(t, err)
	require.True(t, res.WriteOK)
	require.Equal(t, "false", res.Overrides["allow-lan"])
	require.Equal(t, "warning", res.Overrides["log-level"])

	outcome := NewConfigService().ApplyRuleEdit(context.Background(), caller, nil, profile.RuleEdit{
		Rule: "DOMAIN,forwarded.example,DIRECT", Index: 1,
	})
	require.NoError(t, outcome.Err)
	require.True(t, outcome.WriteOK)
	merge, err := profile.ReadMerge()
	require.NoError(t, err)
	require.NotContains(t, string(merge), "rules:")
	require.NotContains(t, string(merge), "DOMAIN,forwarded.example,DIRECT")
	rulePath, err := profile.RuleEditsPath()
	require.NoError(t, err)
	edits, err := os.ReadFile(rulePath)
	require.NoError(t, err)
	require.Contains(t, string(edits), "DOMAIN,forwarded.example,DIRECT")
}

func TestManagedRuleEditsPersistAndDoNotReplayAgainstRuntime(t *testing.T) {
	ctx := context.Background()
	svc, uid, _ := prepareManagedProfile(t)
	cfgSvc := NewConfigService()
	result := cfgSvc.ApplyRuleEdit(ctx, svc, nil, profile.RuleEdit{Rule: "DOMAIN,persisted.example,DIRECT", Index: 1})
	require.NoError(t, result.Err)
	require.True(t, result.WriteOK)
	require.False(t, result.Applied)
	result = cfgSvc.ApplyMihomoOverrides(ctx, svc, nil, map[string]string{"log-level": "debug"})
	require.NoError(t, result.Err)
	path, err := config.ManagedMihomoConfigPath()
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(string(data), "DOMAIN,persisted.example,DIRECT"))

	raw, err := profile.ReadRawFile(uid)
	require.NoError(t, err)
	_, err = svc.ApplyProfile(ctx, uid, false)
	require.NoError(t, err)
	data, err = os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(string(data), "DOMAIN,persisted.example,DIRECT"))
	require.NotContains(t, string(data), "rule-edits")
	actualRaw, err := profile.ReadRawFile(uid)
	require.NoError(t, err)
	require.Equal(t, raw, actualRaw)
}

func TestRuleEditsKeepExplicitNativeRulesAcrossOverrideSaves(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := prepareManagedProfile(t)
	cfgSvc := NewConfigService()
	path, err := config.ManagedMihomoConfigPath()
	require.NoError(t, err)
	merge := []byte("rules: ['MATCH,DIRECT']\n")
	require.NoError(t, profile.WriteMerge(merge))
	require.NoError(t, cfgSvc.ApplyRuleEdit(ctx, svc, nil, profile.RuleEdit{Rule: "DOMAIN,only-the-edit.example,DIRECT", Index: 1}).Err)
	after, err := profile.ReadMerge()
	require.NoError(t, err)
	require.Equal(t, merge, after)

	require.NoError(t, cfgSvc.ApplyMihomoOverrides(ctx, svc, nil, map[string]string{"log-level": "warning"}).Err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(string(got), "DOMAIN,only-the-edit.example,DIRECT"))
	require.Contains(t, string(got), "MATCH,DIRECT")
	after, err = profile.ReadMerge()
	require.NoError(t, err)
	require.NotContains(t, string(after), "only-the-edit.example")
}

func TestRuleEditPreservesLegacyRuntimeRules(t *testing.T) {
	ctx := context.Background()
	svc, uid, _ := prepareManagedProfile(t)
	_, err := svc.ApplyProfile(ctx, uid, false)
	require.NoError(t, err)
	path, err := config.ManagedMihomoConfigPath()
	require.NoError(t, err)
	old := "DOMAIN,legacy.example,DIRECT"
	unchanged := "DOMAIN,keep-local.example,DIRECT"
	require.NoError(t, config.InsertRule(path, old, 1))
	require.NoError(t, config.InsertRule(path, unchanged, 1))

	result := NewConfigService().ApplyRuleEdit(ctx, svc, nil, profile.RuleEdit{
		Match: old, Rule: "DOMAIN,migrated.example,REJECT", Index: 2,
	})
	require.NoError(t, result.Err)
	require.True(t, result.WriteOK)
	runtimeData, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(runtimeData), unchanged)
	require.NotContains(t, string(runtimeData), old)

	generated, err := profile.GenerateForUID(uid, path)
	require.NoError(t, err)
	require.Contains(t, string(generated.Data), "DOMAIN,migrated.example,REJECT")
}

func TestLocalExternalOverridesApplyReloadRestartAndRollback(t *testing.T) {
	setupExternal := func(t *testing.T, withActiveSub bool) (string, *ConfigService) {
		t.Helper()
		pSvc := newIsolatedService(t)
		extPath, err := config.GetMihomoConfigPath()
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(extPath, []byte("external-controller: 127.0.0.1:9090\nsecret: old-sec\nmixed-port: 7890\nallow-lan: true\nlog-level: info\nmode: rule\n"), 0644))
		if withActiveSub {
			p, err := pSvc.AddProfile("ext-sub", profile.SubSource{Kind: profile.SourceLocal, Path: "/tmp/sub.yaml"})
			require.NoError(t, err)
			require.NoError(t, profile.WriteRaw(p.UID, []byte("allow-lan: true\nlog-level: warning\nmode: rule\nrules:\n  - MATCH,DIRECT\n")))
			require.NoError(t, pSvc.SetActiveSub(p.UID))
		}
		cfgSvc := NewConfigService()
		cfgSvc.restartMihomo = func() error { return errors.New("no sudo") }
		return extPath, cfgSvc
	}

	t.Run("hot reload applies overrides and clearing falls through to subscription", func(t *testing.T) {
		extPath, cfgSvc := setupExternal(t, true)
		var reloads atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPut && r.URL.Path == "/configs" {
				reloads.Add(1)
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()
		require.NoError(t, config.WriteMihomoField(extPath, "external-controller", strings.TrimPrefix(server.URL, "http://")))
		client := api.NewClient(config.MihomoEndpoint{ExternalController: server.URL, Secret: "old-sec"}, 1000)

		res := cfgSvc.ApplyMihomoOverrides(context.Background(), nil, client, map[string]string{
			"allow-lan":  "false",
			"log-level":  "debug",
			"mixed-port": "7892",
		})
		require.NoError(t, res.Err)
		require.True(t, res.WriteOK)
		require.True(t, res.Applied)
		require.False(t, res.NeedsRestart)
		require.Equal(t, int32(1), reloads.Load())

		data, err := config.ReadMihomoYAML(extPath)
		require.NoError(t, err)
		assert.Equal(t, false, data["allow-lan"])
		assert.Equal(t, "debug", data["log-level"])
		assert.Equal(t, 7892, data["mixed-port"])

		// Clearing allow-lan & log-level falls through to active subscription raw.yaml.
		res = cfgSvc.ApplyMihomoOverrides(context.Background(), nil, client, map[string]string{
			"mixed-port": "7892",
		})
		require.NoError(t, res.Err)
		data, err = config.ReadMihomoYAML(extPath)
		require.NoError(t, err)
		assert.Equal(t, true, data["allow-lan"])
		assert.Equal(t, "warning", data["log-level"])
	})

	t.Run("reload failure rolls back both external config and merge.yaml", func(t *testing.T) {
		extPath, cfgSvc := setupExternal(t, false)
		oldExt, err := os.ReadFile(extPath)
		require.NoError(t, err)
		oldMerge := []byte("mode: rule\n")
		require.NoError(t, profile.WriteMerge(oldMerge))

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "invalid config", http.StatusBadRequest)
		}))
		defer server.Close()
		client := api.NewClient(config.MihomoEndpoint{ExternalController: server.URL}, 1000)

		res := cfgSvc.ApplyMihomoOverrides(context.Background(), nil, client, map[string]string{
			"log-level": "debug",
		})
		require.Error(t, res.Err)
		require.False(t, res.WriteOK)
		require.True(t, res.Restored)

		afterExt, err := os.ReadFile(extPath)
		require.NoError(t, err)
		assert.Equal(t, oldExt, afterExt)
		afterMerge, err := profile.ReadMerge()
		require.NoError(t, err)
		assert.Equal(t, oldMerge, afterMerge)
	})

	t.Run("controller change tries service restart, falls back to NeedsRestart when unavailable", func(t *testing.T) {
		extPath, cfgSvc := setupExternal(t, false)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()
		require.NoError(t, config.WriteMihomoField(extPath, "external-controller", strings.TrimPrefix(server.URL, "http://")))
		client := api.NewClient(config.MihomoEndpoint{ExternalController: server.URL, Secret: "old-sec"}, 1000)

		// Without passwordless restart -> hot-reloads other fields and flags NeedsRestart.
		res := cfgSvc.ApplyMihomoOverrides(context.Background(), nil, client, map[string]string{
			"external-controller": "127.0.0.1:9095",
			"log-level":           "error",
		})
		require.NoError(t, res.Err)
		require.True(t, res.WriteOK)
		require.True(t, res.Applied)
		require.False(t, res.Restarted)
		require.True(t, res.NeedsRestart)
		require.True(t, res.EndpointChanged)

		res = cfgSvc.ApplyMihomoOverrides(context.Background(), nil, client, map[string]string{
			"external-controller": "127.0.0.1:9095",
			"log-level":           "debug",
		})
		require.NoError(t, res.Err)
		require.True(t, res.Applied)
		require.True(t, res.NeedsRestart, "saving another field does not switch the live controller")

		// With working service restart -> Restarted=true, NeedsRestart=false.
		var restarted atomic.Bool
		cfgSvc.restartMihomo = func() error { restarted.Store(true); return nil }
		res = cfgSvc.ApplyMihomoOverrides(context.Background(), nil, client, map[string]string{
			"external-controller": "127.0.0.1:9096",
		})
		require.NoError(t, res.Err)
		require.True(t, restarted.Load())
		require.True(t, res.WriteOK)
		require.True(t, res.Applied)
		require.True(t, res.Restarted)
		require.False(t, res.NeedsRestart)
	})
}

func TestRemoteOnlyExternalOverridesPayloadPatchAndRollback(t *testing.T) {
	t.Run("active subscription sends payload and reports non-persistence and identity restart need", func(t *testing.T) {
		pSvc := newIsolatedService(t)
		require.NoError(t, config.Update(func(cfg *config.Config) error {
			cfg.Core.Mode = "external"
			cfg.Core.ExternalController = "http://192.168.1.50:9090"
			cfg.Core.ExternalSecret = "remote-sec"
			cfg.Core.ExternalMixedPort = 7890
			return nil
		}))
		p, err := pSvc.AddProfile("remote-sub", profile.SubSource{Kind: profile.SourceLocal, Path: "/tmp/r.yaml"})
		require.NoError(t, err)
		require.NoError(t, profile.WriteRaw(p.UID, []byte("mode: rule\nrules:\n  - MATCH,DIRECT\n")))
		require.NoError(t, pSvc.SetActiveSub(p.UID))

		var gotPayload string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPut, r.Method)
			require.Equal(t, "/configs", r.URL.Path)
			var body map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			gotPayload = body["payload"]
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()
		client := api.NewClient(config.MihomoEndpoint{ExternalController: server.URL}, 1000)

		res := NewConfigService().ApplyMihomoOverrides(context.Background(), nil, client, map[string]string{
			"allow-lan":           "false",
			"log-level":           "debug",
			"external-controller": "192.168.1.50:9091",
		})
		require.NoError(t, res.Err)
		require.True(t, res.WriteOK)
		require.True(t, res.Applied)
		require.True(t, res.Remote)
		require.True(t, res.RemoteNotPersisted)
		require.True(t, res.NeedsRestart)
		assert.Contains(t, gotPayload, "allow-lan: false")
		assert.Contains(t, gotPayload, "log-level: debug")
		assert.Contains(t, gotPayload, "MATCH,DIRECT")

		// Remote endpoint config must NOT be silently overwritten.
		cfg, err := config.Load()
		require.NoError(t, err)
		assert.Equal(t, "http://192.168.1.50:9090", cfg.Core.ExternalController)
		assert.Equal(t, "remote-sec", cfg.Core.ExternalSecret)
	})

	t.Run("remote without active subscription patches only runtime fields and rolls back on failure", func(t *testing.T) {
		newIsolatedService(t)
		require.NoError(t, config.Update(func(cfg *config.Config) error {
			cfg.Core.Mode = "external"
			cfg.Core.ExternalController = "http://192.168.1.50:9090"
			cfg.Core.ExternalSecret = "remote-sec"
			return nil
		}))

		var gotMethod string
		var gotPatch map[string]any
		failPatch := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			if failPatch {
				http.Error(w, "patch failed", http.StatusInternalServerError)
				return
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&gotPatch))
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()
		client := api.NewClient(config.MihomoEndpoint{ExternalController: server.URL}, 1000)

		res := NewConfigService().ApplyMihomoOverrides(context.Background(), nil, client, map[string]string{
			"allow-lan":  "false",
			"log-level":  "error",
			"mixed-port": "7899",
		})
		require.NoError(t, res.Err)
		require.Equal(t, http.MethodPatch, gotMethod)
		assert.Equal(t, false, gotPatch["allow-lan"])
		assert.Equal(t, "error", gotPatch["log-level"])
		assert.Equal(t, float64(7899), gotPatch["mixed-port"])
		require.True(t, res.Remote)
		require.True(t, res.RemoteNotPersisted)
		require.False(t, res.NeedsRestart)

		// Patch failure restores previous merge.yaml.
		oldMerge, err := profile.ReadMerge()
		require.NoError(t, err)
		failPatch = true
		res = NewConfigService().ApplyMihomoOverrides(context.Background(), nil, client, map[string]string{
			"log-level": "debug",
		})
		require.Error(t, res.Err)
		require.False(t, res.WriteOK)
		require.True(t, res.Restored)
		afterMerge, err := profile.ReadMerge()
		require.NoError(t, err)
		assert.Equal(t, oldMerge, afterMerge)
	})
}

func TestCoreMutationsRejectRawFromPreviousSubscriptionSource(t *testing.T) {
	for _, name := range []string{"overrides", "rule", "chain"} {
		t.Run(name, func(t *testing.T) {
			core, uid, _ := prepareManagedProfile(t)
			profileSvc := NewProfileService(nil, core)
			require.NoError(t, profileSvc.SetActiveSub(uid))
			p, err := profileSvc.findProfile(uid)
			require.NoError(t, err)
			require.NoError(t, profile.WriteRawForSource(p, []byte(testChainConfigYAML)))
			_, err = core.ApplyProfile(context.Background(), uid, false)
			require.NoError(t, err)
			path, err := config.ManagedMihomoConfigPath()
			require.NoError(t, err)
			before, err := os.ReadFile(path)
			require.NoError(t, err)

			source := profile.SubSource{Kind: profile.SourceRemote, URL: "https://subscription.example.test/changed"}
			require.NoError(t, profileSvc.EditProfile(uid, p.Name, source))
			cfgSvc := NewConfigService()
			apply := func() MihomoOverridesSaveResult {
				ctx := context.Background()
				switch name {
				case "overrides":
					return cfgSvc.ApplyMihomoOverrides(ctx, core, nil, map[string]string{"log-level": "debug"})
				case "rule":
					return cfgSvc.ApplyRuleEdit(ctx, core, nil, profile.RuleEdit{Rule: "DOMAIN,fixture.example,DIRECT", Index: 1})
				default:
					return cfgSvc.ApplyProxyChain(ctx, core, nil, profile.ProxyChain{Group: "PROXY", Nodes: []string{"Entry-HK", "Exit-US"}, Enabled: true})
				}
			}
			result := apply()
			require.ErrorIs(t, result.Err, profile.ErrStaleRaw, "mutation must wait for a snapshot from the new source")
			require.False(t, result.WriteOK)
			after, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, before, after)

			p.Source = source
			require.NoError(t, profile.WriteRawForSource(p, []byte(testChainConfigYAML)))
			result = apply()
			require.NoError(t, result.Err)
			require.True(t, result.WriteOK)
		})
	}
}
