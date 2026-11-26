package config

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProfileRestoreRollsBackFinalWriteFailure(t *testing.T) {
	root := seedProfileRestore(t)
	want := errors.New("final rename failed")
	err := restoreProfileFiles(prepareProfileRestoreTest, func(from, to string) error {
		if filepath.Base(to) == "config.yaml" {
			merge, err := os.ReadFile(filepath.Join(root, "merge.yaml"))
			require.NoError(t, err)
			require.Equal(t, "log-level: debug\n", string(merge))
			return want
		}
		return os.Rename(from, to)
	})
	require.ErrorIs(t, err, want)
	require.NoDirExists(t, filepath.Join(root, profileRestoreDir))
	assertOriginalProfileFiles(t, root)
}

func TestProfileRestoreRecoversInterruptedProcess(t *testing.T) {
	root := seedProfileRestore(t)
	exe, err := os.Executable()
	require.NoError(t, err)
	child := exec.Command(exe, "-test.run=^TestProfileRestoreExitHelper$")
	child.Env = append(os.Environ(), "MIHOSH_TEST_RESTORE_ROOT="+root)
	output, err := child.CombinedOutput()
	var exit *exec.ExitError
	require.ErrorAs(t, err, &exit, "%s", output)
	require.Equal(t, 47, exit.ExitCode(), "%s", output)
	require.FileExists(t, filepath.Join(root, profileRestoreDir, restoreJournal))
	merge, err := os.ReadFile(filepath.Join(root, "merge.yaml"))
	require.NoError(t, err)
	require.Equal(t, "log-level: debug\n", string(merge))
	metadata, err := os.ReadFile(filepath.Join(root, "config.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(metadata), "language: en-US")
	// Load is a normal startup path and must not expose partially restored metadata.
	assertOriginalProfileFiles(t, root)
	require.NoDirExists(t, filepath.Join(root, profileRestoreDir))
}

func TestProfileRestoreExitHelper(t *testing.T) {
	root := os.Getenv("MIHOSH_TEST_RESTORE_ROOT")
	if root == "" {
		return
	}
	relative, err := filepath.Rel(os.TempDir(), root)
	require.NoError(t, err)
	require.False(t, strings.HasPrefix(relative, ".."))
	require.NotEqual(t, ".", relative)
	t.Setenv("MIHOSH_CONFIG_DIR", root)
	require.NoError(t, restoreProfileFiles(prepareProfileRestoreTest, func(from, to string) error {
		if err := os.Rename(from, to); err != nil {
			return err
		}
		if filepath.Base(to) == "config.yaml" {
			os.Exit(47)
		}
		return nil
	}))
}

func seedProfileRestore(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("MIHOSH_CONFIG_DIR", root)
	for name, data := range map[string]string{
		"config.yaml":     "language: zh-CN\nlocal_extension: retained\n",
		"merge.yaml":      "log-level: warning\n",
		"rule-edits.yaml": "edits: []\n",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(data), 0600))
	}
	return root
}

func prepareProfileRestoreTest(cfg *Config) (ProfileFiles, error) {
	cfg.Language = "en-US"
	return ProfileFiles{Merge: []byte("log-level: debug\n"), Rules: []byte("legacy: {}\n"), Chains: []byte("chains: []\n")}, nil
}

func assertOriginalProfileFiles(t *testing.T, root string) {
	t.Helper()
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "zh-CN", cfg.Language)
	for name, want := range map[string]string{
		"config.yaml":     "language: zh-CN\nlocal_extension: retained\n",
		"merge.yaml":      "log-level: warning\n",
		"rule-edits.yaml": "edits: []\n",
	} {
		got, err := os.ReadFile(filepath.Join(root, name))
		require.NoError(t, err)
		require.Equal(t, want, string(got), name)
	}
	require.NoFileExists(t, filepath.Join(root, "chain-edits.yaml"))
}
