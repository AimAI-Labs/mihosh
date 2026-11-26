package service

import (
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/stretchr/testify/require"
)

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
