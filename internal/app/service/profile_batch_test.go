package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/stretchr/testify/require"
)

func TestProfileServiceBatchAndScheduledUpdates(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())

	var hits atomic.Int32
	okSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Subscription-Userinfo", "upload=10; download=20; total=1000")
		_, _ = w.Write([]byte("proxies: []\n"))
	}))
	defer okSrv.Close()

	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failSrv.Close()

	now := time.Now()
	cfg := config.DefaultConfig
	cfg.Subs = []profile.Profile{
		{
			UID:       "due1",
			Name:      "DueSub",
			UpdatedAt: now.Add(-2 * time.Hour).Unix(),
			Source:    profile.SubSource{Kind: profile.SourceRemote, URL: okSrv.URL + "/due", UpdateInterval: 60},
		},
		{
			UID:       "fresh1",
			Name:      "FreshSub",
			UpdatedAt: now.Add(-10 * time.Minute).Unix(),
			Source:    profile.SubSource{Kind: profile.SourceRemote, URL: okSrv.URL + "/fresh", UpdateInterval: 60},
		},
		{
			UID:       "manual1",
			Name:      "ManualFailingSub",
			UpdatedAt: 0,
			Source:    profile.SubSource{Kind: profile.SourceRemote, URL: failSrv.URL + "/fail", UpdateInterval: 0},
		},
		{
			UID:    "local1",
			Name:   "LocalSub",
			Source: profile.SubSource{Kind: profile.SourceLocal, Path: "/tmp/local.yaml"},
		},
	}
	if err := config.Save(&cfg); err != nil {
		t.Fatal(err)
	}

	svc := NewProfileService(nil)

	// 1. UpdateDueRemoteProfiles only updates overdue remote profiles (due1)
	dueRes, err := svc.UpdateDueRemoteProfiles(context.Background(), now)
	if err != nil {
		t.Fatalf("UpdateDueRemoteProfiles error: %v", err)
	}
	if dueRes.Total != 1 || dueRes.Updated != 1 || dueRes.Failed != 0 {
		t.Fatalf("expected 1/1 due update, got %+v", dueRes)
	}
	if hits.Load() != 1 {
		t.Fatalf("expected exactly 1 HTTP fetch for overdue sub, got %d", hits.Load())
	}

	// Re-checking immediately finds 0 due subscriptions
	dueRes2, err := svc.UpdateDueRemoteProfiles(context.Background(), now)
	if err != nil || dueRes2.Total != 0 {
		t.Fatalf("expected 0 due subscriptions on second check, got %+v err=%v", dueRes2, err)
	}

	// 2. UpdateAllRemoteProfiles updates all 3 remote profiles (ignores local1), reporting 2 ok + 1 failed
	allRes, err := svc.UpdateAllRemoteProfiles(context.Background())
	if err != nil {
		t.Fatalf("UpdateAllRemoteProfiles error: %v", err)
	}
	if allRes.Total != 3 || allRes.Updated != 2 || allRes.Failed != 1 {
		t.Fatalf("expected 2/3 ok + 1 failed, got %+v", allRes)
	}
}

func TestBatchStopsClaimingSubscriptionsAfterCancel(t *testing.T) {
	svc := newIsolatedService(t)
	started := make(chan struct{}, 4)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		select {
		case started <- struct{}{}:
		default:
		}
		<-r.Context().Done()
	}))
	defer server.Close()
	p, err := svc.AddProfile("batch test", profile.SubSource{Kind: profile.SourceRemote, URL: server.URL})
	require.NoError(t, err)
	uids := make([]string, 5000)
	for i := range uids {
		uids[i] = p.UID
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	baseline := runtime.NumGoroutine()
	finished := make(chan BatchUpdateResult, 1)
	go func() { finished <- svc.BatchUpdateRemoteProfiles(ctx, uids) }()
	for range 4 {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("expected four active downloads")
		}
	}
	require.Less(t, runtime.NumGoroutine()-baseline, 50, "a large batch must not start a goroutine for each UID")
	cancel()
	select {
	case result := <-finished:
		require.Equal(t, len(uids), result.Failed)
		require.Zero(t, result.Updated)
		require.Equal(t, int32(4), requests.Load())
	case <-time.After(2 * time.Second):
		t.Fatal("canceled batch did not finish promptly")
	}
}

func TestOversizedRemoteSubscriptionPreservesLastSnapshot(t *testing.T) {
	svc := newIsolatedService(t)
	var response atomic.Int32
	large := bytes.Repeat([]byte{'a'}, profile.MaxRemoteSubscriptionBytes+1)
	old := []byte("proxies: []\n")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch response.Load() {
		case 0:
			_, _ = w.Write(old)
		case 1:
			w.Header().Set("Content-Length", fmt.Sprint(len(large)))
			_, _ = w.Write(large)
		case 2:
			w.(http.Flusher).Flush()
			_, _ = io.Copy(w, bytes.NewReader(large))
		}
	}))
	defer server.Close()
	p, err := svc.AddProfile("size test", profile.SubSource{Kind: profile.SourceRemote, URL: server.URL})
	require.NoError(t, err)
	require.NoError(t, svc.FetchProfile(p.UID))
	first, err := svc.findProfile(p.UID)
	require.NoError(t, err)
	for _, mode := range []int32{1, 2} {
		response.Store(mode)
		err := svc.FetchProfile(p.UID)
		require.ErrorContains(t, err, i18n.Tf("profile.fetcher.err_size_limit", profile.MaxRemoteSubscriptionBytes>>20))
		got, err := profile.ReadRawFile(p.UID)
		require.NoError(t, err)
		require.Equal(t, old, got)
		current, err := svc.findProfile(p.UID)
		require.NoError(t, err)
		require.Equal(t, first.UpdatedAt, current.UpdatedAt)
	}
}
