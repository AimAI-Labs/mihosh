package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/stretchr/testify/require"
)

func TestControlRequiresLocalPrivateOwner(t *testing.T) {
	newIsolatedService(t)
	svc := NewCoreService()
	defer svc.Close()
	require.ErrorIs(t, svc.Control(context.Background(), "stop", ""), ErrNoCoreOwner)
	root, err := config.GetConfigDir()
	require.NoError(t, err)
	dir := filepath.Join(root, "runtime")
	require.NoError(t, os.MkdirAll(dir, 0700))
	for _, address := range []string{"https://evil.example", "http://127.0.0.1:1234/?token=bad", "http://user:password@127.0.0.1:1234"} {
		data, _ := json.Marshal(coreOwner{address, "0000000000000000000000000000000000000000000000000000000000000000"})
		require.NoError(t, os.WriteFile(filepath.Join(dir, "owner.json"), data, 0600))
		err = svc.Control(context.Background(), "stop", "")
		require.Error(t, err)
		require.False(t, errors.Is(err, ErrNoCoreOwner), "invalid record should be rejected before any request")
	}
}

func TestLocalControlRejectsWrongToken(t *testing.T) {
	newIsolatedService(t)
	svc := NewCoreService()
	defer svc.Close()
	root, err := config.GetConfigDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "runtime"), 0700))
	svc.mu.Lock()
	err = svc.startControl(root)
	svc.mu.Unlock()
	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(root, "runtime", "owner.json"))
	require.NoError(t, err)
	var owner coreOwner
	require.NoError(t, json.Unmarshal(data, &owner))
	req, err := http.NewRequest(http.MethodPost, owner.URL+"/control", strings.NewReader(`{"action":"stop"}`))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer incorrect-token")
	response, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = response.Body.Close() }()
	require.Equal(t, http.StatusUnauthorized, response.StatusCode)
}
