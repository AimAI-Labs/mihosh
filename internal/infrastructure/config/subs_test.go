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

// TestConfig_SubsRoundTripFromDisk 用全新 viper 实例从磁盘读取配置，
// 真正模拟「进程重启」场景，捕获 struct tag 与磁盘序列化不一致的回归。
// （TestConfig_SubsRoundTrip 依赖 viper 全局内存状态，旧实现会因内存往返假性通过。）
func TestConfig_SubsRoundTripFromDisk(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	in := &Config{
		Language: "zh-CN",
		Subs: []profile.Profile{{
			UID:       "uid-x",
			Name:      "磁盘往返订阅",
			UpdatedAt: 1700000000,
			Source:    profile.SubSource{Kind: profile.SourceRemote, URL: "https://example.com/x.yaml"},
		}},
		ActiveSub: "uid-x",
	}
	require.NoError(t, Save(in))

	// 用全新的 viper 实例从磁盘读取，不依赖任何全局内存状态。
	dir, _ := GetConfigDir()
	configFile := filepath.Join(dir, "config.yaml")
	reader := viper.New()
	reader.SetConfigFile(configFile)
	reader.SetConfigType("yaml")
	require.NoError(t, reader.ReadInConfig())

	var out Config
	require.NoError(t, reader.Unmarshal(&out))

	require.Len(t, out.Subs, 1, "磁盘往返后应保留订阅")
	assert.Equal(t, "uid-x", out.Subs[0].UID)
	assert.Equal(t, "磁盘往返订阅", out.Subs[0].Name)
	assert.Equal(t, "https://example.com/x.yaml", out.Subs[0].Source.URL)
	// 核心断言：UpdatedAt 必须真正从磁盘 yaml 中读回（旧实现因 yaml tag 缺失会读到 0）。
	assert.Equal(t, int64(1700000000), out.Subs[0].UpdatedAt, "UpdatedAt 须持久化到磁盘并读回，否则重启后显示「未更新」")
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
