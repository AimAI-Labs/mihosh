package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/stretchr/testify/require"
)

func TestImportRawDirectoryKeepsSourcesAndUpdatesSnapshotsExplicitly(t *testing.T) {
	svc := newIsolatedService(t)
	dir, err := profile.RawDirectory()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dir, 0700))
	file := filepath.Join(dir, "work.yaml")
	initial := []byte("mode: rule\n")
	require.NoError(t, os.WriteFile(file, initial, 0600))
	require.NoError(t, svc.ImportRawDirectory())
	require.NoError(t, svc.ImportRawDirectory())
	profiles, active, err := svc.ListProfiles()
	require.NoError(t, err)
	require.Empty(t, active)
	require.Len(t, profiles, 1)
	entry := profiles[0]
	require.Equal(t, file, entry.Source.Path)
	snapshot, err := profile.RawPath(entry.UID)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dir, entry.UID, "raw.yaml"), snapshot)
	updated := []byte("mode: global\n")
	require.NoError(t, os.WriteFile(file, updated, 0600))
	require.NoError(t, svc.ImportRawDirectory())
	read, err := profile.ReadRawFile(entry.UID)
	require.NoError(t, err)
	require.Equal(t, initial, read)
	require.NoError(t, svc.FetchProfile(entry.UID))
	read, err = profile.ReadRawFile(entry.UID)
	require.NoError(t, err)
	require.Equal(t, updated, read)
	require.NoError(t, svc.DeleteProfile(entry.UID))
	read, err = os.ReadFile(file)
	require.NoError(t, err)
	require.Equal(t, updated, read)
}
