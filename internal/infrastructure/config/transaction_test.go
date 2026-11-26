package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/stretchr/testify/require"
)

func TestLoadReflectsAnotherProcessConfigurationWrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cfg := DefaultConfig
	cfg.Language = "en-US"
	require.NoError(t, Save(&cfg))
	root, err := GetConfigDir()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "config.yaml"), []byte("language: zh-CN\ntimeout: 9876\n"), 0600))
	latest, err := Load()
	require.NoError(t, err)
	require.Equal(t, "zh-CN", latest.Language)
	require.Equal(t, 9876, latest.Timeout)
}

func TestExplicitServiceRootIsSharedByConfigProfilesAndBackups(t *testing.T) {
	root := t.TempDir()
	t.Setenv("MIHOSH_CONFIG_DIR", root)
	require.NoError(t, Save(&DefaultConfig))
	path, err := profile.RawPath("root-test")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "raw", "root-test", "raw.yaml"), path)
	actual, err := GetConfigDir()
	require.NoError(t, err)
	require.Equal(t, root, actual)
	backup, err := profile.BackupConfig(filepath.Join(root, "config.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, backup)
	_, err = os.Stat(filepath.Join(root, "config-backups", backup))
	require.NoError(t, err)
	t.Setenv("MIHOSH_CONFIG_DIR", "relative-root")
	_, err = GetConfigDir()
	require.Error(t, err)
	_, err = profile.RawPath("root-test")
	require.Error(t, err)
}

func TestConfigurationUpdatesAreAtomicAcrossProcesses(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		t.Skip("cross-process configuration locking is available on Linux, macOS and Windows")
	}
	root := t.TempDir()
	t.Setenv("MIHOSH_CONFIG_DIR", root)
	cfg := DefaultConfig
	cfg.Timeout = 0
	cfg.AutoRefreshInterval = 0
	cfg.Core.Version = "v1.2.3"
	cfg.Subs = []profile.Profile{{UID: "keep", Name: "keep"}}
	require.NoError(t, Save(&cfg))
	executable, err := os.Executable()
	require.NoError(t, err)
	var commands []*exec.Cmd
	for _, field := range []string{"timeout", "auto_refresh_interval"} {
		command := exec.Command(executable, "-test.run=TestConfigurationUpdateChild")
		command.Env = append(os.Environ(), "MIHOSH_CONFIG_UPDATE_FIELD="+field, "MIHOSH_TEST_CONFIG_ROOT="+root)
		commands = append(commands, command)
		require.NoError(t, command.Start())
		t.Cleanup(func() { _ = command.Process.Kill() })
	}
	for _, command := range commands {
		require.NoError(t, command.Wait())
	}
	latest, err := Load()
	require.NoError(t, err)
	require.Equal(t, 20, latest.Timeout)
	require.Equal(t, 20, latest.AutoRefreshInterval)
	require.Equal(t, "v1.2.3", latest.Core.Version)
	require.Len(t, latest.Subs, 1)
}

func TestConfigurationUpdateChild(t *testing.T) {
	field := os.Getenv("MIHOSH_CONFIG_UPDATE_FIELD")
	if field == "" {
		return
	}
	root := os.Getenv("MIHOSH_TEST_CONFIG_ROOT")
	relative, err := filepath.Rel(os.TempDir(), root)
	if root == "" || err != nil || relative == "." || strings.HasPrefix(relative, "..") {
		t.Fatal("isolated helper directory is required")
	}
	t.Setenv("MIHOSH_CONFIG_DIR", root)
	change := func(cfg *Config) error {
		switch field {
		case "timeout":
			cfg.Timeout++
		case "auto_refresh_interval":
			cfg.AutoRefreshInterval++
		default:
			return fmt.Errorf("unsupported test field %q", field)
		}
		return nil
	}
	for i := 0; i < 20; i++ {
		require.NoError(t, Update(change))
	}
}

func TestConfigurationLockReturnsRetryableConflict(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		t.Skip("cross-process configuration locking is available on Linux, macOS and Windows")
	}
	root := t.TempDir()
	t.Setenv("MIHOSH_CONFIG_DIR", root)
	release, err := lockAppConfigFile(filepath.Join(root, ".config.lock"))
	require.NoError(t, err)
	err = Update(func(*Config) error {
		t.Error("mutation ran with the lock held")
		return nil
	})
	release()
	require.ErrorIs(t, err, ErrConfigBusy)
	require.NoError(t, Update(func(*Config) error { return nil }))
}

func TestConcurrentMihomoFieldChangesPreserveEveryField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("mixed-port: 7890\n"), 0600))
	done := make(chan error, 20)
	for i := 0; i < 20; i++ {
		go func(index int) { done <- WriteMihomoField(path, fmt.Sprintf("field-%d", index), index) }(i)
	}
	for i := 0; i < 20; i++ {
		require.NoError(t, <-done)
	}
	data, err := ReadMihomoYAML(path)
	require.NoError(t, err)
	for i := 0; i < 20; i++ {
		require.Equal(t, i, data[fmt.Sprintf("field-%d", i)])
	}
}

func TestManagedFieldEditsRespectProfileTransactionLock(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		t.Skip("managed-core locking is Linux/Windows only")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	path, err := ManagedMihomoConfigPath()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, []byte("mixed-port: 7890\nrules:\n  - MATCH,DIRECT\n"), 0600))
	release, err := coreinfra.LockRuntime(filepath.Join(filepath.Dir(path), "config.lock"))
	require.NoError(t, err)
	require.Error(t, WriteMihomoField(path, "mixed-port", 7891))
	require.Error(t, InsertRule(path, "DOMAIN,example.com,DIRECT", 0))
	release()
	require.NoError(t, WriteMihomoField(path, "mixed-port", 7891))
}
