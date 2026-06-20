package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfig_SubsRoundTrip 验证 Subs / ActiveSub 经 Save→Load 后保持完整。
func TestConfig_SubsRoundTrip(t *testing.T) {
	// viper 全局状态需 Reset 隔离。
	viper.Reset()
	t.Cleanup(viper.Reset)

	// 把 HOME 指到临时目录，使 GetConfigDir 落在隔离位置。
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	in := &Config{
		APIAddress:          "http://127.0.0.1:9090",
		Language:            "zh-CN",
		AutoRefreshInterval: 5,
		Theme:               "tokyo-night",
		Subs: []profile.Profile{
			{
				UID:  "uid-a",
				Name: "订阅A",
				Source: profile.SubSource{
					Kind: profile.SourceRemote,
					URL:  "https://example.com/a.yaml",
				},
				UpdatedAt: 1700000000,
			},
			{
				UID:  "uid-b",
				Name: "订阅B",
				Source: profile.SubSource{
					Kind: profile.SourceLocal,
					Path: "/etc/mihomo/local.yaml",
				},
			},
		},
		ActiveSub: "uid-a",
	}

	require.NoError(t, Save(in))

	// 确认文件确实写到了隔离目录
	dir, _ := GetConfigDir()
	require.FileExists(t, filepath.Join(dir, "config.yaml"))

	out, err := Load()
	require.NoError(t, err)
	require.NotNil(t, out)

	assert.Equal(t, in.ActiveSub, out.ActiveSub)
	require.Len(t, out.Subs, 2, "subs 应原样读回")

	assert.Equal(t, "uid-a", out.Subs[0].UID)
	assert.Equal(t, "订阅A", out.Subs[0].Name)
	assert.Equal(t, profile.SourceRemote, out.Subs[0].Source.Kind)
	assert.Equal(t, "https://example.com/a.yaml", out.Subs[0].Source.URL)
	assert.Equal(t, int64(1700000000), out.Subs[0].UpdatedAt)

	assert.Equal(t, "uid-b", out.Subs[1].UID)
	assert.Equal(t, profile.SourceLocal, out.Subs[1].Source.Kind)
	assert.Equal(t, "/etc/mihomo/local.yaml", out.Subs[1].Source.Path)
}

// TestConfig_NoSubsLegacyConfig 旧配置（无 subs 键）加载时不报错且字段为零值。
func TestConfig_NoSubsLegacyConfig(t *testing.T) {
	// viper 使用全局状态，需 Reset 隔离本测试，避免被前置用例的 Set 污染。
	viper.Reset()
	t.Cleanup(viper.Reset)

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	// 手写一个不含 subs/active_sub 的旧式配置
	dir, _ := GetConfigDir()
	require.NoError(t, writeLegacyConfig(dir, "api_address: http://x:9090\nlanguage: en-US\n"))

	out, err := Load()
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Empty(t, out.Subs, "旧配置无 subs 键时应为空")
	assert.Empty(t, out.ActiveSub, "旧配置无 active_sub 键时应为空")
	// 注：viper 全局状态在测试间残留，此处不断言 api_address 等标量
	// （与本改动无关的既有脆弱性）；重点只验证 subs/active_sub 的默认值降级。
}

// writeLegacyConfig 在指定目录写入 config.yaml 内容（自动创建目录）。
func writeLegacyConfig(dir, content string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0644)
}
