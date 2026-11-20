package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
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
