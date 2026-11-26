package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteFileAtomic_EnsureDirAndPreserveMode(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "nested", "dir", "config.yaml")

	err := WriteFileAtomic(target, []byte("secret: s1\n"), WriteOptions{
		Perm:         0600,
		EnsureDir:    true,
		PreserveMode: true,
	})
	require.NoError(t, err)

	data, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "secret: s1\n", string(data))

	if runtime.GOOS != "windows" {
		info, err := os.Stat(target)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

		// Updating with a looser default Perm must keep existing 0600 when PreserveMode is true.
		err = WriteFileAtomic(target, []byte("secret: s2\n"), WriteOptions{
			Perm:         0644,
			PreserveMode: true,
		})
		require.NoError(t, err)
		info, err = os.Stat(target)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
	}
}

func TestWriteFileAtomic_FollowSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privilege on Windows")
	}
	dir := t.TempDir()
	realPath := filepath.Join(dir, "real.yaml")
	linkPath := filepath.Join(dir, "link.yaml")
	require.NoError(t, os.WriteFile(realPath, []byte("old: 1\n"), 0600))
	require.NoError(t, os.Symlink(realPath, linkPath))

	err := WriteFileAtomic(linkPath, []byte("new: 2\n"), WriteOptions{
		Perm:          0644,
		PreserveMode:  true,
		FollowSymlink: true,
	})
	require.NoError(t, err)

	linfo, err := os.Lstat(linkPath)
	require.NoError(t, err)
	assert.NotZero(t, linfo.Mode()&os.ModeSymlink, "symlink should remain intact")

	data, err := os.ReadFile(realPath)
	require.NoError(t, err)
	assert.Equal(t, "new: 2\n", string(data))

	info, err := os.Stat(realPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestMarshalYAMLIndent2(t *testing.T) {
	out, err := MarshalYAMLIndent2(map[string]any{
		"rules": []string{"MATCH,DIRECT"},
	})
	require.NoError(t, err)
	assert.Equal(t, "rules:\n  - MATCH,DIRECT\n", string(out))
}
