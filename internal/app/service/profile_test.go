package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newIsolatedService 创建隔离 HOME 与 viper 的 ProfileService。
// 返回服务与配置目录（供测试写入原始订阅文件）。
func newIsolatedService(t *testing.T) *ProfileService {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	// 预置一份最小 mihosh 配置文件，使 config.Load 可用。
	dir, err := config.GetConfigDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("test_url: http://example.com\n"), 0644))

	return NewProfileService(nil)
}

func TestProfileService_AddAndList(t *testing.T) {
	s := newIsolatedService(t)

	p, err := s.AddProfile("订阅A", profile.SubSource{Kind: profile.SourceRemote, URL: "https://x.io/a"})
	require.NoError(t, err)
	assert.NotEmpty(t, p.UID)
	assert.Equal(t, "订阅A", p.Name)

	_, err = s.AddProfile("订阅B", profile.SubSource{Kind: profile.SourceLocal, Path: "/p/b.yaml"})
	require.NoError(t, err)

	subs, active, err := s.ListProfiles()
	require.NoError(t, err)
	assert.Len(t, subs, 2)
	assert.Empty(t, active, "新建订阅不应自动激活")
}

func TestProfileService_AddRejectsEmpty(t *testing.T) {
	s := newIsolatedService(t)

	_, err := s.AddProfile("", profile.SubSource{Kind: profile.SourceRemote, URL: "https://x.io"})
	require.Error(t, err)

	_, err = s.AddProfile("名", profile.SubSource{Kind: profile.SourceRemote, URL: "  "})
	require.Error(t, err)
}

func TestProfileService_Delete(t *testing.T) {
	s := newIsolatedService(t)

	p, err := s.AddProfile("待删", profile.SubSource{Kind: profile.SourceLocal, Path: "/p/x.yaml"})
	require.NoError(t, err)
	require.NoError(t, s.DeleteProfile(p.UID))

	subs, _, err := s.ListProfiles()
	require.NoError(t, err)
	assert.Empty(t, subs)
}

func TestProfileService_DeleteMissing(t *testing.T) {
	s := newIsolatedService(t)
	err := s.DeleteProfile("nope")
	assert.ErrorIs(t, err, ErrSubNotFound)
}

func TestProfileService_DeleteActiveClearsActiveSub(t *testing.T) {
	s := newIsolatedService(t)

	p, err := s.AddProfile("激活项", profile.SubSource{Kind: profile.SourceLocal, Path: "/p/a.yaml"})
	require.NoError(t, err)
	require.NoError(t, s.SetActiveSub(p.UID))

	_, active, err := s.ListProfiles()
	require.NoError(t, err)
	assert.Equal(t, p.UID, active)

	require.NoError(t, s.DeleteProfile(p.UID))
	_, active, err = s.ListProfiles()
	require.NoError(t, err)
	assert.Empty(t, active, "删除激活项后应清空 active_sub")
}

func TestProfileService_SetActiveSubValidation(t *testing.T) {
	s := newIsolatedService(t)

	// 设置不存在的 UID 应报错
	err := s.SetActiveSub("ghost")
	assert.ErrorIs(t, err, ErrSubNotFound)

	// 取消激活（空字符串）不报错
	require.NoError(t, s.SetActiveSub(""))
}

func TestProfileService_Rename(t *testing.T) {
	s := newIsolatedService(t)

	p, err := s.AddProfile("旧名", profile.SubSource{Kind: profile.SourceLocal, Path: "/p/a.yaml"})
	require.NoError(t, err)
	require.NoError(t, s.RenameProfile(p.UID, "新名"))

	got, err := s.findProfile(p.UID)
	require.NoError(t, err)
	assert.Equal(t, "新名", got.Name)

	// 空名拒绝
	err = s.RenameProfile(p.UID, "  ")
	require.Error(t, err)
}

func TestProfileService_EditProfile(t *testing.T) {
	s := newIsolatedService(t)

	p, err := s.AddProfile("原订阅", profile.SubSource{Kind: profile.SourceLocal, Path: "/old/path.yaml"})
	require.NoError(t, err)

	// 编辑名称 + 来源（local → remote）
	require.NoError(t, s.EditProfile(p.UID, "新订阅", profile.SubSource{Kind: profile.SourceRemote, URL: "https://new.io/sub"}))

	got, err := s.findProfile(p.UID)
	require.NoError(t, err)
	assert.Equal(t, "新订阅", got.Name)
	assert.Equal(t, profile.SourceRemote, got.Source.Kind)
	assert.Equal(t, "https://new.io/sub", got.Source.URL)

	// 来源 Kind 规范化：传入 remote URL（Kind 已正确）应原样保留
	require.NoError(t, s.EditProfile(p.UID, "X", profile.SubSource{Kind: profile.SourceRemote, URL: "https://auto.io/sub"}))
	got, err = s.findProfile(p.UID)
	require.NoError(t, err)
	assert.Equal(t, profile.SourceRemote, got.Source.Kind)
	assert.Equal(t, "https://auto.io/sub", got.Source.URL)

	// 空名拒绝
	err = s.EditProfile(p.UID, "  ", profile.SubSource{Kind: profile.SourceRemote, URL: "https://x.io/s"})
	require.Error(t, err)

	// 空来源拒绝
	err = s.EditProfile(p.UID, "ok", profile.SubSource{Kind: profile.SourceLocal, Path: "  "})
	require.Error(t, err)

	// 不存在的 UID 报错
	err = s.EditProfile("ghost", "ok", profile.SubSource{Kind: profile.SourceLocal, Path: "/p/x.yaml"})
	assert.ErrorIs(t, err, ErrSubNotFound)
}

func TestProfileService_FetchLocal(t *testing.T) {
	s := newIsolatedService(t)

	// 准备本地源文件
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "sub.yaml")
	require.NoError(t, os.WriteFile(srcPath, []byte("mode: rule\nproxies: []\n"), 0644))

	p, err := s.AddProfile("本地", profile.SubSource{Kind: profile.SourceLocal, Path: srcPath})
	require.NoError(t, err)
	require.NoError(t, s.FetchProfile(p.UID))

	// UpdatedAt 应被刷新
	got, err := s.findProfile(p.UID)
	require.NoError(t, err)
	assert.NotZero(t, got.UpdatedAt)
}

func TestProfileService_MergeRoundTrip(t *testing.T) {
	s := newIsolatedService(t)

	_, err := s.AddProfile("合并测试", profile.SubSource{Kind: profile.SourceLocal, Path: "/p/a.yaml"})
	require.NoError(t, err)

	// 初始无 merge
	data, err := s.LoadMerge()
	require.NoError(t, err)
	assert.Nil(t, data)

	// 写入合法 merge
	merge := []byte("prepend-rules:\n  - DOMAIN,a.com,DIRECT\n")
	require.NoError(t, s.SaveMerge(merge))

	data, err = s.LoadMerge()
	require.NoError(t, err)
	assert.Equal(t, merge, data)

	// 非法 YAML 拒绝写盘
	err = s.SaveMerge([]byte("mode: {broken:\n"))
	require.Error(t, err)

	// 原内容未变
	data, err = s.LoadMerge()
	require.NoError(t, err)
	assert.Equal(t, merge, data)
}

func TestProfileService_ActivateMissingRaw(t *testing.T) {
	s := newIsolatedService(t)
	p, err := s.AddProfile("无raw", profile.SubSource{Kind: profile.SourceLocal, Path: "/p/a.yaml"})
	require.NoError(t, err)

	// 未 fetch 过 raw.yaml → 应返回 ErrRawNotFound
	_, err = s.Activate(p.UID)
	require.Error(t, err)
	assert.ErrorIs(t, err, profile.ErrRawNotFound)
}

// TestProfileService_ActivateSkipsUnchangedConfig 验证：当生成的最终配置与 mihomo
// 当前配置文件字节相同时，Activate 跳过写盘+备份+重载（client==nil 也不会报错）。
//
// 这是"跳过相同配置重载"优化的核心保障——避免重复触发 mihomo ApplyConfig
// 重建 proxies/rules/providers 导致的内存峰值。
func TestProfileService_ActivateSkipsUnchangedConfig(t *testing.T) {
	// 单一临时 HOME，同时容纳 mihosh 配置（.mihosh）与 mihomo 配置（.config/mihomo）。
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("APPDATA", "") // 防 Windows 下搜到真实系统的 mihomo 目录
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
	viper.Reset()
	t.Cleanup(viper.Reset)

	// 预置 mihosh 配置文件，使 config.Load 可用。
	mihoshDir := filepath.Join(home, ".mihosh")
	require.NoError(t, os.MkdirAll(mihoshDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(mihoshDir, "config.yaml"),
		[]byte("test_url: http://example.com\n"), 0644))

	s := NewProfileService(nil)

	// 添加订阅并写入一份本地 raw.yaml。
	p, err := s.AddProfile("跳过测试", profile.SubSource{Kind: profile.SourceLocal, Path: "/p/a.yaml"})
	require.NoError(t, err)
	require.NoError(t, profile.WriteRaw(p.UID, []byte("mode: rule\nproxies: []\n")))

	// 预先算出 Activate 将生成的字节，写入 mihomo 配置目录（模拟"上次已成功激活"）。
	mihomoDir := filepath.Join(home, ".config", "mihomo")
	require.NoError(t, os.MkdirAll(mihomoDir, 0755))
	mihomoPath := filepath.Join(mihomoDir, "config.yaml")
	expected, err := profile.GenerateAndWriteForUIDIgnoreMergeError(p.UID)
	require.NoError(t, err)
	require.NotEmpty(t, expected)
	require.NoError(t, os.WriteFile(mihomoPath, expected, 0644))

	// client==nil：若未跳过，会在重载阶段报"未配置 mihomo 客户端"。
	// 跳过时应直接返回成功且 BackupName 为空。
	res, err := s.Activate(p.UID)
	require.NoError(t, err)
	assert.Empty(t, res.BackupName, "配置未变更时应跳过备份，BackupName 为空")
}

// TestProfileService_ActivateRewritesWhenConfigChanged 验证：当生成的配置与
// mihomo 当前配置不同时，Activate 走完整流程（此处因 client==nil 在重载阶段报错，
// 但能证明未跳过——跳过分支不会报 client 错误）。
func TestProfileService_ActivateRewritesWhenConfigChanged(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("APPDATA", "")
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
	viper.Reset()
	t.Cleanup(viper.Reset)

	mihoshDir := filepath.Join(home, ".mihosh")
	require.NoError(t, os.MkdirAll(mihoshDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(mihoshDir, "config.yaml"),
		[]byte("test_url: http://example.com\n"), 0644))

	s := NewProfileService(nil)

	p, err := s.AddProfile("变更测试", profile.SubSource{Kind: profile.SourceLocal, Path: "/p/a.yaml"})
	require.NoError(t, err)
	require.NoError(t, profile.WriteRaw(p.UID, []byte("mode: rule\nproxies: []\n")))

	// mihomoPath 放一份与生成内容不同的旧配置。
	mihomoDir := filepath.Join(home, ".config", "mihomo")
	require.NoError(t, os.MkdirAll(mihomoDir, 0755))
	mihomoPath := filepath.Join(mihomoDir, "config.yaml")
	require.NoError(t, os.WriteFile(mihomoPath, []byte("mode: direct\n"), 0644))

	// 配置不同 → 不跳过 → 走到重载阶段 → client==nil 报错。
	_, err = s.Activate(p.UID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "未配置 mihomo 客户端")
}

// TestProfileService_ActivatePreservesManagedFields 验证：当订阅 S 与 merge M 都未提供
// mihomo 设置页托管的 5 个字段时，Activate 仍会从当前 O 中保留它们，避免被订阅覆盖丢失。
func TestProfileService_ActivatePreservesManagedFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("APPDATA", "")
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
	viper.Reset()
	t.Cleanup(viper.Reset)

	mihoshDir := filepath.Join(home, ".mihosh")
	require.NoError(t, os.MkdirAll(mihoshDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(mihoshDir, "config.yaml"),
		[]byte("test_url: http://example.com\n"), 0644))

	s := NewProfileService(nil)
	p, err := s.AddProfile("保留字段测试", profile.SubSource{Kind: profile.SourceLocal, Path: "/p/a.yaml"})
	require.NoError(t, err)

	// 订阅 S 不包含 5 个托管字段。
	require.NoError(t, profile.WriteRaw(p.UID, []byte("" +
		"mode: rule\n" +
		"proxies: []\n" +
		"proxy-groups: []\n")))

	// merge M 也不包含 5 个托管字段。
	require.NoError(t, profile.WriteMerge([]byte("prepend-rules:\n  - MATCH,DIRECT\n")))

	// 当前 O 已包含设置页托管字段，Activate 后应继续保留。
	mihomoDir := filepath.Join(home, ".config", "mihomo")
	require.NoError(t, os.MkdirAll(mihomoDir, 0755))
	mihomoPath := filepath.Join(mihomoDir, "config.yaml")
	require.NoError(t, os.WriteFile(mihomoPath, []byte("" +
		"external-controller: 127.0.0.1:9090\n" +
		"secret: abc123\n" +
		"mixed-port: 7890\n" +
		"allow-lan: true\n" +
		"log-level: debug\n" +
		"mode: global\n"), 0644))

	// client==nil：会在写入后进入重载阶段报错，但足以验证最终 O 已被正确写回。
	_, err = s.Activate(p.UID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "未配置 mihomo 客户端")

	written, err := os.ReadFile(mihomoPath)
	require.NoError(t, err)
	assert.Contains(t, string(written), "external-controller: 127.0.0.1:9090")
	assert.Contains(t, string(written), "secret: abc123")
	assert.Contains(t, string(written), "mixed-port: 7890")
	assert.Contains(t, string(written), "allow-lan: true")
	assert.Contains(t, string(written), "log-level: debug")
}

// TestProfileService_ActivateManagedFieldsOverrideSubscription 验证：即使订阅 S 自带
// 这 5 个字段，Activate 后也必须以当前 O 中的托管值为准，避免设置页值被订阅覆盖。
func TestProfileService_ActivateManagedFieldsOverrideSubscription(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("APPDATA", "")
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
	viper.Reset()
	t.Cleanup(viper.Reset)

	mihoshDir := filepath.Join(home, ".mihosh")
	require.NoError(t, os.MkdirAll(mihoshDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(mihoshDir, "config.yaml"),
		[]byte("test_url: http://example.com\n"), 0644))

	s := NewProfileService(nil)
	p, err := s.AddProfile("托管字段优先级测试", profile.SubSource{Kind: profile.SourceLocal, Path: "/p/a.yaml"})
	require.NoError(t, err)

	// 订阅 S 明确带了与当前 O 冲突的 5 个字段。
	require.NoError(t, profile.WriteRaw(p.UID, []byte("" +
		"mixed-port: 17890\n" +
		"allow-lan: false\n" +
		"log-level: debug\n" +
		"external-controller: 127.0.0.1:19090\n" +
		"secret: from-subscription\n" +
		"mode: rule\n" +
		"proxies: []\n")))

	mihomoDir := filepath.Join(home, ".config", "mihomo")
	require.NoError(t, os.MkdirAll(mihomoDir, 0755))
	mihomoPath := filepath.Join(mihomoDir, "config.yaml")
	require.NoError(t, os.WriteFile(mihomoPath, []byte("" +
		"mixed-port: 7890\n" +
		"allow-lan: true\n" +
		"log-level: info\n" +
		"external-controller: 0.0.0.0:9090\n" +
		"secret: xxxxx\n" +
		"mode: global\n"), 0644))

	_, err = s.Activate(p.UID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "未配置 mihomo 客户端")

	written, err := os.ReadFile(mihomoPath)
	require.NoError(t, err)
	got := string(written)
	assert.Contains(t, got, "mixed-port: 7890")
	assert.Contains(t, got, "allow-lan: true")
	assert.Contains(t, got, "log-level: info")
	assert.Contains(t, got, "external-controller: 0.0.0.0:9090")
	assert.Contains(t, got, "secret: xxxxx")
	assert.NotContains(t, got, "mixed-port: 17890")
	assert.NotContains(t, got, "allow-lan: false")
	assert.NotContains(t, got, "external-controller: 127.0.0.1:19090")
	assert.NotContains(t, got, "secret: from-subscription")
}

// TestNewUID verifies UID uniqueness and format.
func TestNewUID(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		uid, err := newUID()
		require.NoError(t, err)
		assert.Len(t, uid, 8, "UID 应为 8 字符 hex")
		assert.False(t, seen[uid], "UID 不应重复: %s", uid)
		seen[uid] = true
	}
}
