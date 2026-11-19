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
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		t.Skip("cross-process managed-core locking is Linux/Windows only")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cfg := DefaultConfig
	cfg.Timeout = 0
	cfg.Core.Version = "v1.2.3"
	cfg.Subs = []profile.Profile{{UID: "keep", Name: "keep"}}
	require.NoError(t, Save(&cfg))
	executable, err := os.Executable()
	require.NoError(t, err)
	commands := make([]*exec.Cmd, 4)
	for i := range commands {
		command := exec.Command(executable, "-test.run=TestConfigurationUpdateChild")
		command.Env = append(os.Environ(), "MIHOSH_CONFIG_UPDATE_HELPER=1", "MIHOSH_CONFIG_UPDATE_HOME="+home)
		commands[i] = command
		require.NoError(t, command.Start())
		t.Cleanup(func() { _ = command.Process.Kill() })
	}
	for _, command := range commands {
		require.NoError(t, command.Wait())
	}
	latest, err := Load()
	require.NoError(t, err)
	require.Equal(t, 80, latest.Timeout)
	require.Equal(t, "v1.2.3", latest.Core.Version)
	require.Len(t, latest.Subs, 1)
}

func TestConfigurationUpdateChild(t *testing.T) {
	if os.Getenv("MIHOSH_CONFIG_UPDATE_HELPER") != "1" {
		return
	}
	home := os.Getenv("MIHOSH_CONFIG_UPDATE_HOME")
	relative, err := filepath.Rel(os.TempDir(), home)
	if home == "" || err != nil || strings.HasPrefix(relative, "..") {
		t.Fatal("isolated helper directory is required")
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	for i := 0; i < 20; i++ {
		if err := Update(func(cfg *Config) error { cfg.Timeout++; return nil }); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
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
