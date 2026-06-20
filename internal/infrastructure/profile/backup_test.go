package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBackup_TargetMissing 不存在的目标不报错（首次生成场景）。
func TestBackup_TargetMissing(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "absent.yaml")
	name, err := BackupConfig(target)
	require.NoError(t, err)
	assert.Empty(t, name, "目标不存在时不应产生备份")
}

// TestBackup_CreatesBackup 验证备份被创建到 config-backups 目录。
func TestBackup_CreatesBackup(t *testing.T) {
	dir := t.TempDir()
	target := writeTargetConfig(t, dir, "mode: rule\n")

	name, err := BackupConfig(target)
	require.NoError(t, err)
	require.NotEmpty(t, name)

	backupDir, err := configBackupsDir()
	require.NoError(t, err)
	bp := filepath.Join(backupDir, name)
	data, err := os.ReadFile(bp)
	require.NoError(t, err)
	assert.Equal(t, "mode: rule\n", string(data))
}

// TestBackup_RotateEnforcesMaxBackups 验证超过上限后最旧备份被删除。
//
// 注意：备份名带秒级时间戳，连续创建可能重名。本测试通过 sleep 1s 保证时间戳唯一。
func TestBackup_RotateEnforcesMaxBackups(t *testing.T) {
	dir := t.TempDir()
	target := writeTargetConfig(t, dir, "mode: rule\n")

	// 创建 MaxBackups + 3 个备份。
	total := MaxBackups + 3
	for i := 0; i < total; i++ {
		_, err := BackupConfig(target)
		require.NoError(t, err)
		// 等待 1.1s 以保证时间戳唯一（避免秒级重名导致 rotate 统计偏差）。
		time.Sleep(1100 * time.Millisecond)
	}

	backupDir, err := configBackupsDir()
	require.NoError(t, err)
	entries, err := os.ReadDir(backupDir)
	require.NoError(t, err)

	var backups []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "config.yaml.bak.") {
			backups = append(backups, e.Name())
		}
	}
	assert.Len(t, backups, MaxBackups, "应只保留最近 %d 个备份", MaxBackups)
}

// TestBackup_RotateKeepsNewest 验证保留的是最新的若干备份。
func TestBackup_RotateKeepsNewest(t *testing.T) {
	dir := t.TempDir()
	target := writeTargetConfig(t, dir, "mode: rule\n")

	// 清空备份目录，写入 MaxBackups 个，再追加 2 个，检查最新 MaxBackups 个都在。
	var created []string
	for i := 0; i < MaxBackups+2; i++ {
		name, err := BackupConfig(target)
		require.NoError(t, err)
		created = append(created, name)
		time.Sleep(1100 * time.Millisecond)
	}

	backupDir, err := configBackupsDir()
	require.NoError(t, err)
	entries, err := os.ReadDir(backupDir)
	require.NoError(t, err)

	present := map[string]bool{}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "config.yaml.bak.") {
			present[e.Name()] = true
		}
	}

	// 最新的 MaxBackups 个应存在；最早的 2 个应被删
	keep := created[len(created)-MaxBackups:]
	drop := created[:len(created)-MaxBackups]
	for _, n := range keep {
		assert.True(t, present[n], "应保留最新备份 %s", n)
	}
	for _, n := range drop {
		assert.False(t, present[n], "应删除最旧备份 %s", n)
	}
}

func writeTargetConfig(t *testing.T, dir, content string) string {
	t.Helper()
	target := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(target, []byte(content), 0644))
	return target
}
