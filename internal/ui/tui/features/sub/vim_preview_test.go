package sub

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/stretchr/testify/require"
)

func TestVimPreviewUsesReadOnlyTemporaryYAML(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake Vim uses a POSIX shell")
	}
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("MIHOSH_TEST_PREVIEW", dir)
	script := `#!/bin/sh
set -eu
printf '%s\n' "$@" > "$MIHOSH_TEST_PREVIEW/args"
/bin/cat "$3" > "$MIHOSH_TEST_PREVIEW/vimrc"
for path do :; done
/bin/cat "$path" > "$MIHOSH_TEST_PREVIEW/data"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "vim"), []byte(script), 0700))
	original := []byte("mode: rule\n")
	require.NoError(t, profile.WriteRaw("profile", original))
	require.NoError(t, profile.WriteMerge([]byte("mode: global\n")))

	for _, view := range []struct {
		activated bool
		name      string
		data      string
	}{
		{false, "source.yaml", "mode: rule\n"},
		{true, "activated.yaml", "mode: global\n"},
	} {
		preview := vimPreview{uid: "profile", activated: view.activated}
		require.NoError(t, preview.Run())
		log, err := os.ReadFile(filepath.Join(dir, "args"))
		require.NoError(t, err)
		args := strings.Split(strings.TrimSuffix(string(log), "\n"), "\n")
		require.Len(t, args, 11)
		require.Equal(t, []string{"--noplugin", "-i", "NONE", "-n", "-M", "-R", "--"}, args[3:10])
		path := args[len(args)-1]
		require.Equal(t, view.name, filepath.Base(path))
		require.NoDirExists(t, filepath.Dir(path))
		config, err := os.ReadFile(filepath.Join(dir, "vimrc"))
		require.NoError(t, err)
		require.Contains(t, string(config), "syntax on")
		require.Contains(t, string(config), "nomodeline")
		require.Contains(t, string(config), "g:colors_name = 'onedark-pro'")
		data, err := os.ReadFile(filepath.Join(dir, "data"))
		require.NoError(t, err)
		require.Equal(t, view.data, string(data))
	}
	actual, err := profile.ReadRawFile("profile")
	require.NoError(t, err)
	require.Equal(t, original, actual)
}
