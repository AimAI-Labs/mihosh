//go:build !windows

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

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
	require.NoError(t, os.WriteFile(binary, []byte("#!/bin/sh\nexit 1\n"), 0700))
	_, err = svc.ApplyProfile(context.Background(), uid, false)
	require.Error(t, err)
	path, _ := config.ManagedMihomoConfigPath()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, old, data)
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
