package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionUsageSurvivesLegacySaveAndReload(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MIHOSH_CONFIG_DIR", dir)
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("subs:\n"+
		"- uid: example\n"+
		"  name: Example\n"+
		"  source: {kind: 0, url: 'https://example.com/sub?token=fixture'}\n"+
		"  usage: {used_bytes: 4096, total_bytes: 8192, expires_at: 2000000000, upload_bytes: 0}\n"), 0600))

	require.NoError(t, PreserveSubscriptionUsage())
	preserved, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(preserved), subscriptionUsageKey+":")
	require.Contains(t, string(preserved), "source_hash:")
	require.NoError(t, PreserveSubscriptionUsage())
	again, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, preserved, again)

	// The previous writer unmarshalled into a Profile without Usage, then replaced subs.
	type legacySubscription struct {
		UID       string            `mapstructure:"uid" yaml:"uid"`
		Name      string            `mapstructure:"name" yaml:"name"`
		Source    profile.SubSource `mapstructure:"source" yaml:"source"`
		UpdatedAt int64             `mapstructure:"updated_at" yaml:"updated_at"`
	}
	var legacy []legacySubscription
	writer := viper.New()
	writer.SetConfigFile(path)
	require.NoError(t, writer.ReadInConfig())
	require.NoError(t, writer.UnmarshalKey("subs", &legacy))
	require.Len(t, legacy, 1)
	writer.Set("subs", legacy)
	require.NoError(t, writer.WriteConfig())

	fresh, err := Load()
	require.NoError(t, err)
	require.Len(t, fresh.Subs, 1)
	zero := int64(0)
	require.Equal(t, &profile.SubscriptionUsage{
		UsedBytes: 4096, TotalBytes: 8192, ExpiresAt: 2000000000, UploadBytes: &zero,
	}, fresh.Subs[0].Usage)

	legacy[0].Source.URL = "https://example.com/another-source"
	writer.Set("subs", legacy)
	require.NoError(t, writer.WriteConfig())
	fresh, err = Load()
	require.NoError(t, err)
	require.Nil(t, fresh.Subs[0].Usage)
}
