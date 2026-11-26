package service

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/stretchr/testify/require"
)

func TestActivateRefreshesChangedSourceAndPreservesItsHistory(t *testing.T) {
	svc := newIsolatedService(t)
	var reloads atomic.Int32
	attachProfileController(t, svc, func(w http.ResponseWriter, _ *http.Request) {
		reloads.Add(1)
		w.WriteHeader(http.StatusNoContent)
	})
	const oldRaw = "mode: rule\nproxies:\n  - {name: old-node, type: direct}\n"
	const newRaw = "mode: rule\nproxies:\n  - {name: new-node, type: direct}\n"
	oldSource := profile.SubSource{Kind: profile.SourceRemote, URL: "https://offline.example/old"}
	p, err := svc.AddProfile("source change", oldSource)
	require.NoError(t, err)
	// Seed the old directory layout and pre-origin metadata without a network fetch.
	root, err := config.GetConfigDir()
	require.NoError(t, err)
	legacyPath := filepath.Join(root, "profiles", p.UID, "raw.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(legacyPath), 0700))
	require.NoError(t, os.WriteFile(legacyPath, []byte(oldRaw), 0600))
	require.NoError(t, config.Update(func(cfg *config.Config) error { cfg.Subs[0].UpdatedAt = 123; return nil }))
	_, err = svc.Activate(p.UID)
	require.NoError(t, err)
	require.Equal(t, int32(1), reloads.Load())

	var status, requests atomic.Int32
	status.Store(http.StatusBadGateway)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(int(status.Load()))
		_, _ = w.Write([]byte(newRaw))
	}))
	defer server.Close()
	require.NoError(t, svc.EditProfile(p.UID, p.Name, profile.SubSource{Kind: profile.SourceRemote, URL: server.URL}))
	changed, err := svc.findProfile(p.UID)
	require.NoError(t, err)
	require.Zero(t, changed.UpdatedAt)
	_, err = svc.Activate(p.UID)
	require.ErrorContains(t, err, "502")
	require.Equal(t, int32(1), reloads.Load(), "a failed source refresh must not apply the old bytes")
	got, err := profile.ReadRawFile(p.UID)
	require.NoError(t, err)
	require.Equal(t, oldRaw, string(got))

	status.Store(http.StatusOK)
	_, err = svc.Activate(p.UID)
	require.NoError(t, err)
	require.Equal(t, int32(2), reloads.Load())
	path, err := config.GetMihomoConfigPath()
	require.NoError(t, err)
	applied, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(applied), "new-node")
	require.NotContains(t, string(applied), "old-node")
	snaps, err := svc.ListRawSnapshots(p.UID)
	require.NoError(t, err)
	require.Len(t, snaps, 1)
	history, err := profile.ReadSnapshot(profile.SnapshotRaw, p.UID, snaps[0].ID)
	require.NoError(t, err)
	require.Equal(t, oldRaw, string(history))
	_, err = svc.RollbackSnapshot(profile.SnapshotRaw, p.UID, snaps[0].ID)
	require.Error(t, err)

	status.Store(http.StatusBadGateway)
	reloaded := NewProfileService(svc.client)
	_, err = reloaded.Activate(p.UID)
	require.NoError(t, err, "the current source remains usable offline after a service restart")
	require.Equal(t, int32(2), requests.Load())
}

func TestActivateRemoteRetriesUnfetchedSubscriptionAndKeepsCachedActivationOffline(t *testing.T) {
	s := newIsolatedService(t)
	attachProfileController(t, s, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	const raw = "mode: rule\nproxies: []\n"
	var status, requests atomic.Int32
	status.Store(http.StatusBadGateway)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(int(status.Load()))
		_, _ = w.Write([]byte(raw))
	}))
	defer server.Close()
	p, err := s.AddProfile("remote", profile.SubSource{Kind: profile.SourceRemote, URL: server.URL})
	require.NoError(t, err)
	require.Error(t, s.FetchProfile(p.UID))
	path, err := config.GetMihomoConfigPath()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte(raw), 0600))
	status.Store(http.StatusOK)
	_, err = s.Activate(p.UID)
	require.NoError(t, err)
	require.Equal(t, int32(2), requests.Load())
	stored, err := s.findProfile(p.UID)
	require.NoError(t, err)
	require.NotZero(t, stored.UpdatedAt)
	_, active, err := s.ListProfiles()
	require.NoError(t, err)
	require.Equal(t, p.UID, active)
	status.Store(http.StatusBadGateway)
	_, err = s.Activate(p.UID)
	require.NoError(t, err)
	require.Equal(t, int32(2), requests.Load(), "existing raw snapshots must remain usable offline")
}

func TestActivateRemoteFetchFailureReportsCauseWithoutChangingActiveConfig(t *testing.T) {
	for _, entry := range []struct {
		name   string
		status int
		body   string
	}{
		{"HTTP failure", http.StatusBadGateway, "upstream unavailable"},
		{"invalid content", http.StatusOK, "<html>login required</html>"},
	} {
		t.Run(entry.name, func(t *testing.T) {
			s := newIsolatedService(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(entry.status)
				_, _ = w.Write([]byte(entry.body))
			}))
			defer server.Close()
			old, err := s.AddProfile("old", profile.SubSource{Kind: profile.SourceLocal, Path: "/existing.yaml"})
			require.NoError(t, err)
			require.NoError(t, s.SetActiveSub(old.UID))
			p, err := s.AddProfile("remote", profile.SubSource{Kind: profile.SourceRemote, URL: server.URL})
			require.NoError(t, err)
			path, err := config.GetMihomoConfigPath()
			require.NoError(t, err)
			baseline, err := os.ReadFile(path)
			require.NoError(t, err)
			_, err = s.Activate(p.UID)
			require.Error(t, err)
			require.NotErrorIs(t, err, profile.ErrRawNotFound)
			if entry.status == http.StatusBadGateway {
				require.Contains(t, err.Error(), "502")
			}
			actual, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, baseline, actual)
			_, active, err := s.ListProfiles()
			require.NoError(t, err)
			require.Equal(t, old.UID, active)
			rawPath, err := profile.RawPath(p.UID)
			require.NoError(t, err)
			require.NoFileExists(t, rawPath)
		})
	}
}
