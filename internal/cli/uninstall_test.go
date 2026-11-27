package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/stretchr/testify/require"
)

func setupCLIUninstallEnv(t *testing.T) (root, home, exe string) {
	t.Helper()
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	t.Cleanup(func() { i18n.SetLanguageOverride("auto") })

	home = t.TempDir()
	root = filepath.Join(t.TempDir(), "mihosh-cfg")
	t.Setenv("MIHOSH_CONFIG_DIR", root)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("ZDOTDIR", "")

	cfg := config.DefaultConfig
	cfg.ShellProxy = true
	require.NoError(t, config.Save(&cfg))
	require.NoError(t, os.WriteFile(filepath.Join(root, "proxy.sh"), []byte("echo proxy\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".bashrc"), []byte("# >>> mihosh proxy >>>\neval mihosh\n# <<< mihosh proxy <<<\n"), 0600))

	exe = filepath.Join(t.TempDir(), "mihosh")
	require.NoError(t, os.WriteFile(exe, []byte("bin"), 0755))
	return root, home, exe
}

func newTestUninstallCmd(exe string) (*bytes.Buffer, *bytes.Buffer, func(args ...string) error) {
	var stdout, stderr bytes.Buffer
	run := func(args ...string) error {
		stdout.Reset()
		stderr.Reset()
		cmd := newUninstallCommandWithService(func() *service.UninstallService {
			return service.NewUninstallService(nil).SetTargets(exe, os.Getenv("HOME"))
		})
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs(args)
		return cmd.Execute()
	}
	return &stdout, &stderr, run
}

func TestCLIUninstallArgumentAndOutputValidation(t *testing.T) {
	_, _, exe := setupCLIUninstallEnv(t)
	_, _, run := newTestUninstallCmd(exe)

	err := run("extra-arg")
	require.Error(t, err)
	require.Equal(t, exitCodeParameter, exitCodeForError(err))

	err = run("--output", "xml", "--yes")
	require.Error(t, err)
	require.Equal(t, exitCodeParameter, exitCodeForError(err))

	err = run("--output", "json")
	require.Error(t, err)
	require.Equal(t, exitCodeParameter, exitCodeForError(err))
}

func TestCLIUninstallDryRunAndNonInteractivePipeRejected(t *testing.T) {
	root, _, exe := setupCLIUninstallEnv(t)
	stdout, _, run := newTestUninstallCmd(exe)

	require.NoError(t, run("--dry-run", "-o", "json"))
	var res service.UninstallResult
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &res))
	require.True(t, res.DryRun)
	require.True(t, res.ConfigRemoved)
	require.True(t, res.BinaryRemoved)
	require.FileExists(t, exe)
	require.DirExists(t, root)

	// Non-interactive *os.File stdin without --yes must fail cleanly with exitCodeParameter.
	devNull, err := os.Open(os.DevNull)
	require.NoError(t, err)
	defer func() { _ = devNull.Close() }()
	cmd := newUninstallCommandWithService(func() *service.UninstallService {
		return service.NewUninstallService(nil).SetTargets(exe, os.Getenv("HOME"))
	})
	cmd.SetIn(devNull)
	err = cmd.Execute()
	require.Error(t, err)
	require.Equal(t, exitCodeParameter, exitCodeForError(err))
}

func TestCLIUninstallYesKeepAndRemove(t *testing.T) {
	t.Run("yes keep-config table", func(t *testing.T) {
		root, _, exe := setupCLIUninstallEnv(t)
		stdout, _, run := newTestUninstallCmd(exe)

		require.NoError(t, run("--yes", "--keep-config", "-o", "table"))
		require.NoFileExists(t, exe)
		require.FileExists(t, filepath.Join(root, "config.yaml"))
		require.Contains(t, stdout.String(), root)
	})

	t.Run("yes remove all json", func(t *testing.T) {
		root, _, exe := setupCLIUninstallEnv(t)
		stdout, _, run := newTestUninstallCmd(exe)

		require.NoError(t, run("-y", "-o", "json"))
		var res service.UninstallResult
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &res))
		require.True(t, res.ConfigRemoved)
		require.True(t, res.BinaryRemoved)
		require.NoFileExists(t, exe)
		require.NoDirExists(t, root)
	})
}

func TestCLIUninstallInteractivePrompts(t *testing.T) {
	t.Run("cancel on n", func(t *testing.T) {
		root, _, exe := setupCLIUninstallEnv(t)
		var out, errBuf bytes.Buffer
		cmd := newUninstallCommandWithService(func() *service.UninstallService {
			return service.NewUninstallService(nil).SetTargets(exe, os.Getenv("HOME"))
		})
		cmd.SetIn(strings.NewReader("n\n"))
		cmd.SetOut(&out)
		cmd.SetErr(&errBuf)
		require.NoError(t, cmd.Execute())
		require.FileExists(t, exe)
		require.DirExists(t, root)
	})

	t.Run("confirm and default keep config", func(t *testing.T) {
		root, _, exe := setupCLIUninstallEnv(t)
		var out, errBuf bytes.Buffer
		cmd := newUninstallCommandWithService(func() *service.UninstallService {
			return service.NewUninstallService(nil).SetTargets(exe, os.Getenv("HOME"))
		})
		cmd.SetIn(strings.NewReader("y\n\n"))
		cmd.SetOut(&out)
		cmd.SetErr(&errBuf)
		require.NoError(t, cmd.Execute())
		require.NoFileExists(t, exe)
		require.FileExists(t, filepath.Join(root, "config.yaml"))
	})

	t.Run("confirm and decline keeping config", func(t *testing.T) {
		root, _, exe := setupCLIUninstallEnv(t)
		var out, errBuf bytes.Buffer
		cmd := newUninstallCommandWithService(func() *service.UninstallService {
			return service.NewUninstallService(nil).SetTargets(exe, os.Getenv("HOME"))
		})
		cmd.SetIn(strings.NewReader("y\nn\n"))
		cmd.SetOut(&out)
		cmd.SetErr(&errBuf)
		require.NoError(t, cmd.Execute())
		require.NoFileExists(t, exe)
		require.NoDirExists(t, root)
	})

	t.Run("explicit --keep-config skips second prompt", func(t *testing.T) {
		root, _, exe := setupCLIUninstallEnv(t)
		var out, errBuf bytes.Buffer
		cmd := newUninstallCommandWithService(func() *service.UninstallService {
			return service.NewUninstallService(nil).SetTargets(exe, os.Getenv("HOME"))
		})
		cmd.SetIn(strings.NewReader("y\n"))
		cmd.SetOut(&out)
		cmd.SetErr(&errBuf)
		cmd.SetArgs([]string{"--keep-config"})
		require.NoError(t, cmd.Execute())
		require.NoFileExists(t, exe)
		require.FileExists(t, filepath.Join(root, "config.yaml"))
	})
}
