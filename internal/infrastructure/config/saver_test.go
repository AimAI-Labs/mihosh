package config

import (
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSavePersistsProxyAddress(t *testing.T) {
	t.Cleanup(func() {
		viper.Reset()
	})
	viper.Reset()

	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	cfg := &Config{
		APIAddress:   "http://127.0.0.1:9090",
		Secret:       "test-secret",
		TestURL:      "http://www.gstatic.com/generate_204",
		Timeout:      5000,
		ProxyAddress: "http://127.0.0.1:9999",
	}

	err := Save(cfg)
	require.NoError(t, err, "Save() returned error")

	configFile := filepath.Join(tempHome, ".mihosh", "config.yaml")
	reader := viper.New()
	reader.SetConfigFile(configFile)
	reader.SetConfigType("yaml")

	err = reader.ReadInConfig()
	require.NoError(t, err, "ReadInConfig() returned error")

	got := reader.GetString("proxy_address")
	assert.Equal(t, cfg.ProxyAddress, got, "proxy_address not persisted")
}

func TestSavePersistsLanguage(t *testing.T) {
	t.Cleanup(func() {
		viper.Reset()
	})
	viper.Reset()

	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	cfg := &Config{
		APIAddress:   "http://127.0.0.1:9090",
		Secret:       "test-secret",
		TestURL:      "http://www.gstatic.com/generate_204",
		Timeout:      5000,
		ProxyAddress: "http://127.0.0.1:7890",
		Language:     "en-US",
	}

	err := Save(cfg)
	require.NoError(t, err, "Save() returned error")

	configFile := filepath.Join(tempHome, ".mihosh", "config.yaml")
	reader := viper.New()
	reader.SetConfigFile(configFile)
	reader.SetConfigType("yaml")

	err = reader.ReadInConfig()
	require.NoError(t, err, "ReadInConfig() returned error")

	got := reader.GetString("language")
	assert.Equal(t, cfg.Language, got, "language not persisted")
}

func TestSavePersistsAutoRefreshInterval(t *testing.T) {
	t.Cleanup(func() {
		viper.Reset()
	})
	viper.Reset()

	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	cfg := &Config{
		APIAddress:          "http://127.0.0.1:9090",
		Secret:              "test-secret",
		TestURL:             "http://www.gstatic.com/generate_204",
		Timeout:             5000,
		ProxyAddress:        "http://127.0.0.1:7890",
		Language:            "en-US",
		AutoRefreshInterval: 9,
	}

	err := Save(cfg)
	require.NoError(t, err, "Save() returned error")

	configFile := filepath.Join(tempHome, ".mihosh", "config.yaml")
	reader := viper.New()
	reader.SetConfigFile(configFile)
	reader.SetConfigType("yaml")

	err = reader.ReadInConfig()
	require.NoError(t, err, "ReadInConfig() returned error")

	got := reader.GetInt("auto_refresh_interval")
	assert.Equal(t, cfg.AutoRefreshInterval, got, "auto_refresh_interval not persisted")
}
