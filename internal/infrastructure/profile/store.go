package profile

// store.go — 订阅的磁盘读写。
//
// 布局（相对 mihosh 配置目录 ~/.mihosh）：
//   profiles/<uid>/raw.yaml    订阅原始配置快照（fetcher 写入）
//   profiles/<uid>/merge.yaml  用户覆写 YAML（TUI 编辑器写入）
//
// 元数据（Profile 列表）不在此处持久化，由 mihosh config 包管理（存 config.yaml
// 的 subs/active_sub 字段）。本包只读写 raw/merge 内容文件。
//
// 所有写入均原子（临时文件 + Rename），与 config.atomicWriteFile 风格一致。

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ErrRawNotFound 表示某订阅尚未拉取（raw.yaml 不存在）。
var ErrRawNotFound = errors.New("订阅尚未拉取，请先更新")

// profilesRoot 返回 ~/.mihosh/profiles 根目录。
func profilesRoot() (string, error) {
	mihoshDir, err := mihoshConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(mihoshDir, "profiles"), nil
}

// profileDir 返回某订阅的专属目录，并确保已创建。
func profileDir(uid string) (string, error) {
	root, err := profilesRoot()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, uid)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("创建订阅目录失败: %w", err)
	}
	return dir, nil
}

// RawPath 返回某订阅 raw.yaml 的完整路径（不保证文件存在）。
func RawPath(uid string) (string, error) {
	dir, err := profileDir(uid)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "raw.yaml"), nil
}

// MergePath 返回全局 merge.yaml 的完整路径。
func MergePath() (string, error) {
	root, err := mihoshConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "merge.yaml"), nil
}

// ReadRaw 读取并解析某订阅的 raw.yaml 为 yaml.Node。
// 文件不存在返回 ErrRawNotFound。
func ReadRaw(uid string) (*yaml.Node, error) {
	p, err := RawPath(uid)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrRawNotFound
		}
		return nil, fmt.Errorf("读取订阅原始配置失败: %w", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("解析订阅原始配置失败: %w", err)
	}
	return &doc, nil
}

// WriteRaw 原子写入 raw.yaml（覆盖）。
func WriteRaw(uid string, data []byte) error {
	p, err := RawPath(uid)
	if err != nil {
		return err
	}
	return atomicWrite(p, data)
}

// ReadMerge 读取全局 merge.yaml 内容（原始字节）。
func ReadMerge() ([]byte, error) {
	p, err := MergePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("读取覆写配置失败: %w", err)
	}
	if len(trimSpaceBytes(data)) == 0 {
		return nil, nil
	}
	return data, nil
}

// ReadMergeNode 读取并解析全局 merge.yaml 为 yaml.Node。
func ReadMergeNode() (*yaml.Node, error) {
	data, err := ReadMerge()
	if err != nil {
		return nil, err
	}
	if len(trimSpaceBytes(data)) == 0 {
		return nil, nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("解析覆写配置失败: %w", err)
	}
	return &doc, nil
}

// WriteMerge 原子写入全局 merge.yaml。
func WriteMerge(data []byte) error {
	if len(trimSpaceBytes(data)) > 0 {
		var probe yaml.Node
		if err := yaml.Unmarshal(data, &probe); err != nil {
			return fmt.Errorf("覆写 YAML 语法错误: %w", err)
		}
	}
	p, err := MergePath()
	if err != nil {
		return err
	}
	return atomicWrite(p, data)
}

// DeleteProfileDir 删除某订阅的整个目录（raw + merge）。
// 目录不存在不报错。
func DeleteProfileDir(uid string) error {
	root, err := profilesRoot()
	if err != nil {
		return err
	}
	dir := filepath.Join(root, uid)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("删除订阅目录失败: %w", err)
	}
	return nil
}

// atomicWrite 先写临时文件再 Rename 覆盖目标（避免半写损坏）。
// 与 config 包的 atomicWriteFile 行为一致。
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".mihosh-profile-*")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // Rename 成功后已不存在，忽略错误

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("同步临时文件失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时文件失败: %w", err)
	}
	if err := os.Chmod(tmpName, 0644); err != nil {
		return fmt.Errorf("设置临时文件权限失败: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("替换配置文件失败: %w", err)
	}
	return nil
}

// mihoshConfigDir 返回 ~/.mihosh 配置目录。
// （从 config 包取，但为避免循环依赖此处独立实现，逻辑与 config.GetConfigDir 一致。）
func mihoshConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".mihosh"), nil
}

// trimSpaceBytes 去除首尾空白字节（与 config.bytesTrimSpace 等价）。
func trimSpaceBytes(b []byte) []byte {
	for len(b) > 0 && isSpaceByte(b[0]) {
		b = b[1:]
	}
	for len(b) > 0 && isSpaceByte(b[len(b)-1]) {
		b = b[:len(b)-1]
	}
	return b
}

func isSpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}
