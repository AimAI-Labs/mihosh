package service

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/stretchr/testify/require"
)

func TestCoreConfigureWithoutSubscriptionCreatesPrivateDirectConfig(t *testing.T) {
	newIsolatedService(t)
	cfg, err := config.Load()
	require.NoError(t, err)
	cfg.Core.Version = coreinfra.PinnedRelease().Tag
	require.NoError(t, config.Save(cfg))
	svc := NewCoreService()
	defer svc.Close()
	require.NoError(t, svc.Configure())
	path, err := config.ManagedMihomoConfigPath()
	require.NoError(t, err)
	actual, err := config.GetMihomoConfigPath()
	require.NoError(t, err)
	require.Equal(t, path, actual)
	data, err := config.ReadMihomoYAML(path)
	require.NoError(t, err)
	require.Equal(t, false, data["allow-lan"])
	require.GreaterOrEqual(t, len(data["secret"].(string)), 32)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(raw), "MATCH,DIRECT")
	secret := data["secret"]
	require.NoError(t, svc.Configure())
	data, err = config.ReadMihomoYAML(path)
	require.NoError(t, err)
	require.Equal(t, secret, data["secret"])
	cfg, err = config.Load()
	require.NoError(t, err)
	require.Equal(t, "managed", cfg.Core.Mode)
	require.Empty(t, cfg.Subs)
	profile, err := NewProfileService(nil).AutoImportLocalSub()
	require.NoError(t, err)
	require.Nil(t, profile)
}
func TestCoreNetworkSettingsValidateBeforeSave(t *testing.T) {
	newIsolatedService(t)
	svc := NewCoreService()
	defer svc.Close()
	require.Error(t, svc.SaveNetwork("socks5://localhost:1234", nil))
	require.Error(t, svc.SaveNetwork("", []coreinfra.Accelerator{{URL: "http://unsafe.example", Metadata: true}}))
	require.NoError(t, svc.SaveNetwork("http://localhost:8888", []coreinfra.Accelerator{}, 1200))
	cfg, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, "http://localhost:8888", cfg.Core.Proxy)
	require.Equal(t, 1200, cfg.Core.DownloadTimeoutSeconds)
	require.NotNil(t, cfg.Core.Accelerators)
	require.Empty(t, coreFetcher(cfg, nil).Accelerators)
	require.False(t, strings.Contains(cfg.Core.Mode, "managed"), "network setup must not take ownership of an external core")
}

func TestCoreNetworkPreferencesCanBeSavedDuringDownload(t *testing.T) {
	newIsolatedService(t)
	svc := NewCoreService()
	defer svc.Close()
	svc.mu.Lock()
	err := svc.SaveNetwork("", []coreinfra.Accelerator{{URL: "https://mirror.example", Metadata: true, Asset: true}})
	svc.mu.Unlock()
	require.NoError(t, err)
	cfg, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, "https://mirror.example", cfg.Core.Accelerators[0].URL)
}

func TestCoreExternalConnectionRoundTripDoesNotResolveLocalFiles(t *testing.T) {
	newIsolatedService(t)
	svc := NewCoreService()
	defer svc.Close()
	require.NoError(t, svc.ConnectExternal("https://example.com:9443", "remote-secret", 8888))
	cfg, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, "https://example.com:9443", cfg.Core.ExternalController)
	endpoint := config.ResolveMihomoEndpoint()
	require.Equal(t, "https://example.com:9443", endpoint.ExternalController)
	require.Equal(t, "remote-secret", endpoint.Secret)
	require.Equal(t, 8888, endpoint.MixedPort)
	_, err = config.GetMihomoConfigPath()
	require.ErrorIs(t, err, config.ErrExternalConfig)
	_, err = config.GetMihomoConfigPathForWrite()
	require.ErrorIs(t, err, config.ErrExternalConfig)
	cfg.Core.Version = "v1.2.3"
	cfg.Core.PreviousVersion = "v1.2.2"
	require.NoError(t, config.Save(cfg))
	cfg, err = config.Load()
	require.NoError(t, err)
	require.Equal(t, "v1.2.2", cfg.Core.PreviousVersion)
}

func TestCoreStatusIncludesManagedPathsAndEndpoint(t *testing.T) {
	newIsolatedService(t)
	cfg, err := config.Load()
	require.NoError(t, err)
	cfg.Core.Version = "v1.2.3"
	require.NoError(t, config.Save(cfg))
	svc := NewCoreService()
	defer svc.Close()
	require.NoError(t, svc.Configure())
	status, err := svc.Status()
	require.NoError(t, err)
	root, err := config.GetConfigDir()
	require.NoError(t, err)
	binary, err := coreinfra.BinaryPath(root, cfg.Core.Version, runtime.GOOS)
	require.NoError(t, err)
	path, err := config.ManagedMihomoConfigPath()
	require.NoError(t, err)
	require.Equal(t, binary, status.BinaryPath)
	require.Equal(t, path, status.ConfigPath)
	require.Equal(t, config.ResolveMihomoEndpoint().ExternalController, status.Controller)
	require.Equal(t, config.ResolveMihomoEndpoint().MixedPort, status.MixedPort)
}

func TestCoreActivatesPrereleaseByExactBuildVersion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture executable uses a POSIX shell")
	}
	newIsolatedService(t)
	root, err := config.GetConfigDir()
	require.NoError(t, err)
	version := "alpha-478af2a"
	binary, err := coreinfra.BinaryPath(root, version, runtime.GOOS)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(binary), 0700))
	require.NoError(t, os.WriteFile(binary, []byte("#!/bin/sh\nprintf 'Mihomo alpha-478af2a linux amd64\\n'\n"), 0700))
	svc := NewCoreService()
	defer svc.Close()
	require.NoError(t, svc.ActivateVersion(context.Background(), version))
	cfg, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, version, cfg.Core.Version)
	other := "alpha-1234567"
	wrongBinary, err := coreinfra.BinaryPath(root, other, runtime.GOOS)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(wrongBinary), 0700))
	require.NoError(t, os.WriteFile(wrongBinary, []byte("#!/bin/sh\nprintf 'Mihomo alpha-478af2a linux amd64\\n'\n"), 0700))
	require.Error(t, svc.ActivateVersion(context.Background(), other))
	cfg, err = config.Load()
	require.NoError(t, err)
	require.Equal(t, version, cfg.Core.Version)
}
