//go:build !windows

package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/stretchr/testify/require"
)

func prepareManagedProfile(t *testing.T) (*CoreService, string, []byte) {
	t.Helper()
	newIsolatedService(t)
	cfg, err := config.Load()
	require.NoError(t, err)
	cfg.Core.Version = "v1.2.3"
	require.NoError(t, config.Save(cfg))
	svc := NewCoreService()
	t.Cleanup(svc.Close)
	require.NoError(t, svc.Configure())
	root, err := config.GetConfigDir()
	require.NoError(t, err)
	binary, err := coreinfra.BinaryPath(root, "v1.2.3", runtime.GOOS)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(binary), 0700))
	require.NoError(t, os.WriteFile(binary, []byte("#!/bin/sh\nexit 0\n"), 0700))
	p, err := NewProfileService(nil).AddProfile("local", profile.SubSource{Kind: profile.SourceLocal, Path: "unused"})
	require.NoError(t, err)
	require.NoError(t, profile.WriteRaw(p.UID, []byte("proxies: []\nrules:\n  - MATCH,DIRECT\nport: 8000\nlisteners: [{name: unsafe, type: mixed, port: 8001}]\ndns: {enable: true, listen: 0.0.0.0:53}\ntun: {enable: true}\n")))
	path, err := config.ManagedMihomoConfigPath()
	require.NoError(t, err)
	old, err := os.ReadFile(path)
	require.NoError(t, err)
	return svc, p.UID, old
}
func TestManagedProfileCanBeSavedOfflineAndSanitizesInbound(t *testing.T) {
	svc, uid, _ := prepareManagedProfile(t)
	_, err := svc.ApplyProfile(context.Background(), uid, false)
	require.NoError(t, err)
	path, err := config.ManagedMihomoConfigPath()
	require.NoError(t, err)
	data, err := config.ReadMihomoYAML(path)
	require.NoError(t, err)
	require.NotContains(t, data, "listeners")
	require.NotContains(t, data, "port")
	require.NotContains(t, data["dns"].(map[string]any), "listen")
	require.Equal(t, false, data["tun"].(map[string]any)["enable"])
	cfg, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, uid, cfg.ActiveSub)
	_, err = os.Stat(filepath.Join(filepath.Dir(path), "profile-recovery.json"))
	require.True(t, os.IsNotExist(err))
}

func TestUseBasicConfigKeepsEndpointAndBacksUpSubscription(t *testing.T) {
	svc, uid, _ := prepareManagedProfile(t)
	_, err := svc.ApplyProfile(context.Background(), uid, false)
	require.NoError(t, err)
	endpoint := config.ResolveMihomoEndpoint()
	result, err := svc.UseBasicConfig(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, result.BackupName)
	require.Equal(t, endpoint, config.ResolveMihomoEndpoint())
	cfg, err := config.Load()
	require.NoError(t, err)
	require.Empty(t, cfg.ActiveSub)
	require.Len(t, cfg.Subs, 1)
	path, err := config.ManagedMihomoConfigPath()
	require.NoError(t, err)
	data, err := config.ReadMihomoYAML(path)
	require.NoError(t, err)
	require.NotContains(t, data, "dns")
	require.Equal(t, []any{"MATCH,DIRECT"}, data["rules"])
}

func TestStartCreatesBasicConfigWhenManagedFileIsMissing(t *testing.T) {
	svc, uid, _ := prepareManagedProfile(t)
	require.NoError(t, config.Update(func(cfg *config.Config) error { cfg.ActiveSub = uid; return nil }))
	path, err := config.ManagedMihomoConfigPath()
	require.NoError(t, err)
	require.NoError(t, os.Remove(path))
	root, err := config.GetConfigDir()
	require.NoError(t, err)
	binary, err := coreinfra.BinaryPath(root, "v1.2.3", runtime.GOOS)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(binary, []byte("#!/bin/sh\nexit 1\n"), 0700))
	require.ErrorContains(t, svc.Start(context.Background()), "configuration validation failed")
	data, err := config.ReadMihomoYAML(path)
	require.NoError(t, err)
	require.Equal(t, []any{"MATCH,DIRECT"}, data["rules"])
	cfg, err := config.Load()
	require.NoError(t, err)
	require.Empty(t, cfg.ActiveSub)
	log, _, _ := svc.ReadLogs(0)
	require.Contains(t, log, "Managed configuration was missing")
}

func TestManagedProfileReloadFailureRestoresFileAndMetadata(t *testing.T) {
	svc, uid, _ := prepareManagedProfile(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "reload failed", 500) }))
	defer server.Close()
	path, err := config.ManagedMihomoConfigPath()
	require.NoError(t, err)
	require.NoError(t, config.WriteMihomoField(path, "external-controller", strings.TrimPrefix(server.URL, "http://")))
	old, err := os.ReadFile(path)
	require.NoError(t, err)
	svc.process = &exec.Cmd{} // marks an owned runtime; requests go only to the isolated fake controller
	_, err = svc.ApplyProfile(context.Background(), uid, false)
	require.Error(t, err)
	after, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	require.Equal(t, old, after)
	cfg, err := config.Load()
	require.NoError(t, err)
	require.Empty(t, cfg.ActiveSub)
}
func TestManagedProfileValidationFailureDoesNotCommit(t *testing.T) {
	svc, uid, old := prepareManagedProfile(t)
	root, err := config.GetConfigDir()
	require.NoError(t, err)
	binary, err := coreinfra.BinaryPath(root, "v1.2.3", runtime.GOOS)
	require.NoError(t, err)
	path, err := config.ManagedMihomoConfigPath()
	require.NoError(t, err)
	baseline, err := config.ReadMihomoYAML(path)
	require.NoError(t, err)
	controllerSecret := baseline["secret"].(string)
	require.NoError(t, profile.WriteRaw(uid, []byte("proxies: [{name: test, type: ss, password: subscription-password, uuid: subscription-uuid}]\nproxy-providers:\n  private:\n    type: http\n    url: https://provider.example/private-path-token?auth=query-token\n")))
	script := fmt.Sprintf(`#!/bin/sh
i=0
while [ "$i" -lt 1000 ]; do
  printf '%%080d\n' "$i"
  i=$((i+1))
done
printf '%%s\n' 'yaml: line 7: cannot unmarshal value' 'proxy 0: unsupported proxy type' "can't download GeoIP.dat" >&2
printf '\033[31m%%s\033[0m\n' 'subscription-password subscription-uuid %s' >&2
printf '%%s\n' 'https://provider.example/private-path-token?auth=query-token query-token' >&2
exit 1
`, controllerSecret)
	require.NoError(t, os.WriteFile(binary, []byte(script), 0700))
	_, err = svc.ApplyProfile(context.Background(), uid, false)
	require.Error(t, err)
	for _, reason := range []string{"yaml: line 7", "proxy 0: unsupported proxy type", "can't download GeoIP.dat", "exit status 1"} {
		require.Contains(t, err.Error(), reason)
	}
	for _, secret := range []string{"subscription-password", "subscription-uuid", controllerSecret, "provider.example", "private-path-token", "query-token", "\x1b"} {
		require.NotContains(t, err.Error(), secret)
	}
	require.Less(t, len(err.Error()), 4200, "child output must remain bounded")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, old, data)
	cfg, err := config.Load()
	require.NoError(t, err)
	require.Empty(t, cfg.ActiveSub)
	candidates, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".candidate-*.yaml"))
	require.NoError(t, err)
	require.Empty(t, candidates)
}

func TestManagedProfileValidationReportsProcessFailures(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "validator")
	err := validateManagedProfile(context.Background(), binary, filepath.Join(dir, "config.yaml"), "unused", []byte("proxies: []\n"), nil)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.Contains(t, err.Error(), "cannot start core validation")

	require.NoError(t, os.WriteFile(binary, []byte("#!/bin/sh\nexec sleep 30\n"), 0700))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = validateManagedProfile(ctx, binary, filepath.Join(dir, "config.yaml"), "unused", []byte("proxies: []\n"), nil)
	require.ErrorIs(t, err, context.Canceled)

	ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err = validateManagedProfile(ctx, binary, filepath.Join(dir, "config.yaml"), "unused", []byte("proxies: []\n"), nil)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Contains(t, err.Error(), "subscription validation interrupted")
}

func TestProfileValidationOutputRedactsNestedAndEscapedCredentials(t *testing.T) {
	data := []byte("auth: {credentials: [nested-credential]}\npassword: 'line\"\\password'\nurl: https://alice:secret%2Fvalue@host.example/private?token=query%2Fcredential\n")
	output := "yaml: line 4: nested-credential line\\\"\\\\password query/credential alice secret/value secret%2Fvalue https://other.example/private-token\x00\x07"
	result := profileValidationOutput(output, data)
	require.Contains(t, result, "yaml: line 4")
	for _, secret := range []string{"nested-credential", "password", "query/credential", "alice", "secret/value", "secret%2Fvalue", "other.example", "private-token", "\x00", "\x07"} {
		require.NotContains(t, result, secret)
	}
}

func TestCoreImportsLocalProfileWithoutExternalKernel(t *testing.T) {
	svc, _, _ := prepareManagedProfile(t)
	source := filepath.Join(t.TempDir(), "subscription.yaml")
	require.NoError(t, os.WriteFile(source, []byte("proxies: []\nrules:\n  - MATCH,DIRECT\n"), 0600))
	require.NoError(t, svc.ImportProfile(context.Background(), "core import", source, false))
	cfg, err := config.Load()
	require.NoError(t, err)
	require.Len(t, cfg.Subs, 2)
	require.Equal(t, cfg.Subs[1].UID, cfg.ActiveSub)
}

func TestUpgradeCannotSilentlySelectAnOlderRelease(t *testing.T) {
	svc, _, _ := prepareManagedProfile(t)
	require.NoError(t, config.Update(func(cfg *config.Config) error { cfg.Core.Version = "v2.0.0"; return nil }))
	require.Error(t, svc.UpgradeVersion(context.Background(), "v1.2.3"))
	cfg, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, "v2.0.0", cfg.Core.Version)
}

func TestProfileActivationPreservesGenerationErrors(t *testing.T) {
	for _, mode := range []string{"external", "managed"} {
		for _, failure := range []string{"missing-raw", "invalid-raw", "invalid-current"} {
			t.Run(mode+"/"+failure, func(t *testing.T) {
				svc, uid, _ := prepareManagedProfile(t)
				require.NoError(t, config.Update(func(cfg *config.Config) error { cfg.Core.Mode = mode; return nil }))
				path, err := config.GetMihomoConfigPath()
				require.NoError(t, err)
				switch failure {
				case "missing-raw":
					raw, err := profile.RawPath(uid)
					require.NoError(t, err)
					require.NoError(t, os.Remove(raw))
				case "invalid-raw":
					require.NoError(t, profile.WriteRaw(uid, []byte("proxies: [\n")))
				case "invalid-current":
					require.NoError(t, os.WriteFile(path, []byte("external-controller: [\n"), 0600))
				}
				before, err := os.ReadFile(path)
				require.NoError(t, err)
				if mode == "managed" {
					_, err = svc.ApplyProfile(context.Background(), uid, false)
				} else {
					_, err = NewProfileService(nil).Activate(uid)
				}
				require.Error(t, err)
				switch failure {
				case "missing-raw":
					require.ErrorIs(t, err, profile.ErrRawNotFound)
				case "invalid-raw":
					require.NotErrorIs(t, err, profile.ErrRawNotFound)
					require.Contains(t, err.Error(), "解析订阅原始配置失败")
				case "invalid-current":
					require.NotErrorIs(t, err, profile.ErrRawNotFound)
					require.Contains(t, err.Error(), "解析现有 mihomo 配置失败")
				}
				after, err := os.ReadFile(path)
				require.NoError(t, err)
				require.Equal(t, before, after)
				cfg, err := config.Load()
				require.NoError(t, err)
				require.Empty(t, cfg.ActiveSub)
			})
		}
	}
}

func TestProfileActivationKeepsMergeWarning(t *testing.T) {
	for _, mode := range []string{"external", "managed"} {
		t.Run(mode, func(t *testing.T) {
			svc, uid, _ := prepareManagedProfile(t)
			require.NoError(t, config.Update(func(cfg *config.Config) error { cfg.Core.Mode = mode; return nil }))
			path, err := config.GetMihomoConfigPath()
			require.NoError(t, err)
			if mode == "external" {
				// The unchanged target still requires a confirmed reload on first activation.
				raw, err := profile.ReadRaw(uid)
				require.NoError(t, err)
				data, err := profile.GenerateYAMLWithPreserve(raw, nil, path)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(path, data, 0600))
			}
			merge, err := profile.MergePath()
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(merge, []byte("rules: [\n"), 0600))
			var result ActivateResult
			if mode == "managed" {
				result, err = svc.ApplyProfile(context.Background(), uid, false)
			} else {
				external := NewProfileService(nil)
				attachProfileController(t, external, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
				result, err = external.Activate(uid)
			}
			require.NoError(t, err)
			require.Error(t, result.MergeErr)
			cfg, err := config.Load()
			require.NoError(t, err)
			require.Equal(t, uid, cfg.ActiveSub)
		})
	}
}

func TestManagedYAMLHelpersHandleEmptyAndNullInput(t *testing.T) {
	for _, input := range []string{"", "   \n# comment\n", "null\n"} {
		sanitized, err := sanitizeManagedInbound([]byte(input))
		require.NoError(t, err)
		require.Contains(t, string(sanitized), "tun:")

		networked, err := applyManagedNetwork([]byte(input), false, t.TempDir())
		require.NoError(t, err)
		require.Contains(t, string(networked), "tun:")
	}
}

func TestConnectExternalRejectsInvalidPort(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	svc := NewCoreService()
	defer svc.Close()
	for _, addr := range []string{"127.0.0.1:", "127.0.0.1:0", "127.0.0.1:99999", "http://127.0.0.1:abc"} {
		require.Error(t, svc.ConnectExternal(addr, "", 7890), "expected error for %q", addr)
	}
}
