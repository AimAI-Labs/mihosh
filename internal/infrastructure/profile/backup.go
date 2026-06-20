package profile

// backup.go — 覆盖用户 config.yaml 前的备份与 rotate。
//
// 备份目录：~/.mihosh/config-backups/config.yaml.bak.<unix-ts>
// 定长保留最近 MaxBackups 个（避免无界增长），超限删最旧。

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// MaxBackups 保留的备份份数上限（与 spec 一致：10）。
const MaxBackups = 10

// BackupConfig 在覆盖前把 targetPath 备份到 config-backups/。
//
//   - targetPath 不存在：不报错（视为首次生成），返回空备份名；
//   - 备份成功后按时间戳排序，删除超出 MaxBackups 的最旧备份。
//
// 返回备份文件名（不含目录）与错误。
func BackupConfig(targetPath string) (string, error) {
	if targetPath == "" {
		return "", fmt.Errorf("目标配置路径为空")
	}
	src, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // 目标不存在，无需备份
		}
		return "", fmt.Errorf("读取待备份配置失败: %w", err)
	}

	backupDir, err := configBackupsDir()
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("config.yaml.bak.%d", time.Now().Unix())
	dst := filepath.Join(backupDir, name)
	if err := atomicWrite(dst, src); err != nil {
		return "", err
	}

	if err := rotateBackups(backupDir); err != nil {
		// rotate 失败不阻断主流程（备份已成功），仅返回错误供上层记录。
		return name, err
	}
	return name, nil
}

// configBackupsDir 返回 ~/.mihosh/config-backups 并确保目录存在。
func configBackupsDir() (string, error) {
	mihoshDir, err := mihoshConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(mihoshDir, "config-backups")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("创建备份目录失败: %w", err)
	}
	return dir, nil
}

// rotateBackups 保留最近 MaxBackups 个 config.yaml.bak.* 文件，删除多余的。
func rotateBackups(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("读取备份目录失败: %w", err)
	}

	var backups []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), "config.yaml.bak.") {
			backups = append(backups, e.Name())
		}
	}
	if len(backups) <= MaxBackups {
		return nil
	}

	// 按文件名排序：时间戳越大越新。删除最旧的若干个。
	sort.Strings(backups)
	toDelete := backups[:len(backups)-MaxBackups]
	for _, name := range toDelete {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			// 单个删除失败不中断，继续删其余。
			continue
		}
	}
	return nil
}
