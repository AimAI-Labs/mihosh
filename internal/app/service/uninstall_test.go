package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/host"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/stretchr/testify/require"
)

type stubUninstallServices struct {
	state       host.ServiceState
	statusErr   error
	uninstallFn func(context.Context) error
	actions     []string
}

func (s *stubUninstallServices) Install(context.Context, string) error { return nil }
func (s *stubUninstallServices) Action(ctx context.Context, action string) error {
	s.actions = append(s.actions, action)
	if action == "uninstall" && s.uninstallFn != nil {
		return s.uninstallFn(ctx)
	}
	return nil
}
func (s *stubUninstallServices) Status(context.Context) (host.ServiceState, error) {
	return s.state, s.statusErr
}

func setupUninstallFixture(t *testing.T, svcStub *stubUninstallServices, homebrew bool) (*UninstallService, string, string, string) {
	t.Helper()
	home := t.TempDir()
	root := filepath.Join(t.TempDir(), "mihosh-cfg")
	t.Setenv("MIHOSH_CONFIG_DIR", root)
	t.Setenv("HOME", home)
	t.Setenv("ZDOTDIR", "")

	require.NoError(t, os.MkdirAll(filepath.Join(root, "bin"), 0700))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "raw", "sub1"), 0700))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "runtime"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(root, "merge.yaml"), []byte("allow-lan: true\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "raw", "sub1", "raw.yaml"), []byte("mixed-port: 7890\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "proxy.sh"), []byte("echo proxy\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "bin", "mihosh"), []byte("svcbin"), 0700))

	cfg := config.DefaultConfig
	cfg.Core.Mode = "managed"
	cfg.Core.Runtime = "service"
	cfg.ShellProxy = true
	require.NoError(t, config.Save(&cfg))

	bashrc := filepath.Join(home, ".bashrc")
	require.NoError(t, os.WriteFile(bashrc, []byte("export KEEP=1\n"+shellProxyBlockBegin+"\neval mihosh\n"+shellProxyBlockEnd+"\n"), 0600))

	binDir := t.TempDir()
	if homebrew {
		binDir = filepath.Join(t.TempDir(), "Cellar", "mihosh", "2.0.0", "bin")
		require.NoError(t, os.MkdirAll(binDir, 0755))
	}
	exe := filepath.Join(binDir, "mihosh")
	require.NoError(t, os.WriteFile(exe, []byte("binary"), 0755))

	svc := NewUninstallService(nil)
	svc.executableLookup = func() (string, error) { return exe, nil }
	svc.homeDirLookup = func() (string, error) { return home, nil }
	svc.goos = "linux"
	svc.servicesFactory = func(string) host.Services { return svcStub }
	return svc, root, home, exe
}

func TestUninstallDryRunLeavesEverythingIntact(t *testing.T) {
	stub := &stubUninstallServices{state: host.ServiceState{Installed: true, Running: true}}
	svc, root, home, exe := setupUninstallFixture(t, stub, false)

	res, err := svc.Uninstall(context.Background(), UninstallOptions{KeepConfig: false, DryRun: true})
	require.NoError(t, err)
	require.True(t, res.DryRun)
	require.True(t, res.ServiceRemoved)
	require.True(t, res.CoreStopped)
	require.True(t, res.ConfigRemoved)
	require.True(t, res.BinaryRemoved)
	require.Len(t, res.ShellFilesCleaned, 1)
	require.Empty(t, stub.actions)
	require.FileExists(t, exe)
	require.FileExists(t, filepath.Join(root, "config.yaml"))
	require.FileExists(t, filepath.Join(root, "proxy.sh"))
	require.NotEmpty(t, FindShellProxyStartupFiles(home))
}

func TestUninstallKeepConfigPreservesUserFilesAndResetsRuntimeFlags(t *testing.T) {
	stub := &stubUninstallServices{state: host.ServiceState{Installed: true, Running: true}}
	svc, root, home, exe := setupUninstallFixture(t, stub, false)

	res, err := svc.Uninstall(context.Background(), UninstallOptions{KeepConfig: true})
	require.NoError(t, err)
	require.True(t, res.KeepConfig)
	require.True(t, res.ServiceRemoved)
	require.True(t, res.CoreStopped)
	require.True(t, res.ProxyScriptRemoved)
	require.True(t, res.ServiceBinaryRemoved)
	require.True(t, res.ConfigPreserved)
	require.False(t, res.ConfigRemoved)
	require.True(t, res.BinaryRemoved)
	require.Equal(t, []string{"uninstall"}, stub.actions)

	require.NoFileExists(t, exe)
	require.NoFileExists(t, filepath.Join(root, "proxy.sh"))
	require.NoFileExists(t, filepath.Join(root, "bin", "mihosh"))
	require.FileExists(t, filepath.Join(root, "merge.yaml"))
	require.FileExists(t, filepath.Join(root, "raw", "sub1", "raw.yaml"))
	require.Empty(t, FindShellProxyStartupFiles(home))

	bashrcData, err := os.ReadFile(filepath.Join(home, ".bashrc"))
	require.NoError(t, err)
	require.Equal(t, "export KEEP=1\n", string(bashrcData))

	preserved, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, "session", preserved.Core.Runtime)
	require.False(t, preserved.ShellProxy)
}

func TestUninstallRemoveConfigDeletesConfigDirAndBinary(t *testing.T) {
	stub := &stubUninstallServices{state: host.ServiceState{Installed: true}}
	svc, root, home, exe := setupUninstallFixture(t, stub, false)

	res, err := svc.Uninstall(context.Background(), UninstallOptions{KeepConfig: false})
	require.NoError(t, err)
	require.False(t, res.KeepConfig)
	require.True(t, res.ConfigRemoved)
	require.False(t, res.ConfigPreserved)
	require.True(t, res.BinaryRemoved)
	require.NoFileExists(t, exe)
	require.NoDirExists(t, root)
	require.Empty(t, FindShellProxyStartupFiles(home))
}

func TestUninstallHomebrewAndPermissionFallback(t *testing.T) {
	t.Run("homebrew leaves binary and outputs brew command", func(t *testing.T) {
		stub := &stubUninstallServices{}
		svc, root, _, exe := setupUninstallFixture(t, stub, true)

		res, err := svc.Uninstall(context.Background(), UninstallOptions{KeepConfig: false})
		require.NoError(t, err)
		require.True(t, res.BinaryHomebrew)
		require.False(t, res.BinaryRemoved)
		require.Equal(t, "brew uninstall mihosh", res.ManualCommand)
		require.FileExists(t, exe)
		require.NoDirExists(t, root)
	})

	t.Run("unwritable binary emits manual command", func(t *testing.T) {
		stub := &stubUninstallServices{}
		svc, root, _, exe := setupUninstallFixture(t, stub, false)
		svc.removeBinaryFn = func(_ context.Context, path string, _ bool) binaryRemoveOutcome {
			return binaryRemoveOutcome{NeedsManual: true, ManualCommand: "sudo rm -f " + quoteShellProxy(path)}
		}

		res, err := svc.Uninstall(context.Background(), UninstallOptions{KeepConfig: false})
		require.NoError(t, err)
		require.True(t, res.BinaryNeedsManualRemove)
		require.False(t, res.BinaryRemoved)
		require.Contains(t, res.ManualCommand, "sudo rm -f")
		require.Contains(t, res.ManualCommand, exe)
		require.FileExists(t, exe)
		require.NoDirExists(t, root)
	})
}

func TestUninstallAbortsIfRunningServiceFailsToStop(t *testing.T) {
	stub := &stubUninstallServices{
		state: host.ServiceState{Installed: true, Running: true},
		uninstallFn: func(context.Context) error {
			return errors.New("systemctl denied")
		},
	}
	svc, root, _, exe := setupUninstallFixture(t, stub, false)

	_, err := svc.Uninstall(context.Background(), UninstallOptions{KeepConfig: false})
	require.ErrorContains(t, err, "systemctl denied")
	require.DirExists(t, root)
	require.FileExists(t, exe)
}

func TestIsSafeConfigDirForRemoval(t *testing.T) {
	home := filepath.Join("/home", "alice")
	if runtime.GOOS == "windows" {
		home = `C:\Users\alice`
		require.False(t, isSafeConfigDirForRemoval(`C:\`, home))
		require.False(t, isSafeConfigDirForRemoval(`C:\Users`, home))
		require.False(t, isSafeConfigDirForRemoval(home, home))
		require.True(t, isSafeConfigDirForRemoval(`C:\Users\alice\.mihosh`, home))
		return
	}
	require.False(t, isSafeConfigDirForRemoval("", home))
	require.False(t, isSafeConfigDirForRemoval("relative/.mihosh", home))
	require.False(t, isSafeConfigDirForRemoval("/", home))
	require.False(t, isSafeConfigDirForRemoval("/tmp", home))
	require.False(t, isSafeConfigDirForRemoval("/home", home))
	require.False(t, isSafeConfigDirForRemoval(home, home))
	require.True(t, isSafeConfigDirForRemoval("/home/alice/.mihosh", home))
	require.True(t, isSafeConfigDirForRemoval(t.TempDir(), home))
}

func TestFormatUninstallSummaryLocalized(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	t.Cleanup(func() { i18n.SetLanguageOverride("auto") })
	out := FormatUninstallSummary(UninstallResult{
		ConfigDir:      "/home/test/.mihosh",
		ConfigRemoved:  true,
		ExecutablePath: "/usr/local/bin/mihosh",
		BinaryRemoved:  true,
	})
	require.Contains(t, out, "/home/test/.mihosh")
	require.Contains(t, out, "/usr/local/bin/mihosh")
}
