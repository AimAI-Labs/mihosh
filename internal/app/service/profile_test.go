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
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("api_address: http://127.0.0.1:9090\n"), 0644))

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

	p, err := s.AddProfile("合并测试", profile.SubSource{Kind: profile.SourceLocal, Path: "/p/a.yaml"})
	require.NoError(t, err)

	// 初始无 merge
	data, err := s.LoadMerge(p.UID)
	require.NoError(t, err)
	assert.Nil(t, data)

	// 写入合法 merge
	merge := []byte("prepend-rules:\n  - DOMAIN,a.com,DIRECT\n")
	require.NoError(t, s.SaveMerge(p.UID, merge))

	data, err = s.LoadMerge(p.UID)
	require.NoError(t, err)
	assert.Equal(t, merge, data)

	// 非法 YAML 拒绝写盘
	err = s.SaveMerge(p.UID, []byte("mode: {broken:\n"))
	require.Error(t, err)

	// 原内容未变
	data, err = s.LoadMerge(p.UID)
	require.NoError(t, err)
	assert.Equal(t, merge, data)
}

func TestProfileService_MergeOnMissingProfile(t *testing.T) {
	s := newIsolatedService(t)
	_, err := s.LoadMerge("ghost")
	assert.ErrorIs(t, err, ErrSubNotFound)

	err = s.SaveMerge("ghost", []byte("mode: rule\n"))
	assert.ErrorIs(t, err, ErrSubNotFound)
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
