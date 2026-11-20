package config

import (
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/stretchr/testify/require"
)

func TestUnlockCacheMergesChecksAndKeepsProxyScopesSeparate(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	first := UnlockResultsScope("http://127.0.0.1:7890", MihomoEndpoint{})
	second := UnlockResultsScope("http://127.0.0.1:7891", MihomoEndpoint{})
	now := time.Now()
	firstResult := model.UnlockResult{Status: model.UnlockAvailable, Region: "JP", CheckedAt: now}
	secondResult := model.UnlockResult{Status: model.UnlockRestricted, CheckedAt: now.Add(time.Second)}
	require.NoError(t, SaveUnlockResults(first, map[string]model.UnlockResult{"Netflix": firstResult, "Disney+": {}}))
	require.NoError(t, SaveUnlockResults(first, map[string]model.UnlockResult{"Netflix": {}, "Disney+": secondResult}))

	got := LoadUnlockResults(first)
	require.Len(t, got, 2)
	require.Equal(t, firstResult.Status, got["Netflix"].Status)
	require.Equal(t, firstResult.Region, got["Netflix"].Region)
	require.Equal(t, secondResult.Status, got["Disney+"].Status)
	require.Nil(t, LoadUnlockResults(second))

	require.NoError(t, SaveUnlockResults(second, map[string]model.UnlockResult{"Netflix": firstResult}))
	require.Nil(t, LoadUnlockResults(second), "an older snapshot must not replace the current cache")
	secondResult.CheckedAt = now.Add(2 * time.Second)
	require.NoError(t, SaveUnlockResults(second, map[string]model.UnlockResult{"Disney+": secondResult}))
	require.Equal(t, model.UnlockRestricted, LoadUnlockResults(second)["Disney+"].Status)
	require.Nil(t, LoadUnlockResults(first))
}
