package profile

// store.go — 订阅的磁盘读写。
//
// 布局（相对 mihosh 配置目录 ~/.mihosh）：
//   raw/<uid>/raw.yaml         订阅原始配置快照（旧路径 profiles/<uid>/raw.yaml 仍可读写）
//   merge.yaml                 全局覆写 YAML（TUI 编辑器写入）
//
// 元数据（Profile 列表）不在此处持久化，由 mihosh config 包管理（存 config.yaml
// 的 subs/active_sub 字段）。本包只读写 raw/merge 内容文件。
//
// 所有写入均原子（临时文件 + Rename），与 config.atomicWriteFile 风格一致。

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ErrRawNotFound 表示某订阅尚未拉取（raw.yaml 不存在）。
var ErrRawNotFound = errors.New("订阅尚未拉取，请先更新")

func RawDirectory() (string, error) {
	root, err := mihoshConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "raw"), nil
}

func rawStoragePath(uid string) (string, error) {
	if uid == "" || uid == "." || uid == ".." || filepath.Base(uid) != uid {
		return "", errors.New("invalid profile ID")
	}
	root, err := mihoshConfigDir()
	if err != nil {
		return "", err
	}
	legacy := filepath.Join(root, "profiles", uid, "raw.yaml")
	if _, err := os.Lstat(legacy); err == nil {
		return legacy, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return filepath.Join(root, "raw", uid, "raw.yaml"), nil
}

// RawPath 返回某订阅 raw.yaml 的完整路径（不保证文件存在）。
func RawPath(uid string) (string, error) {
	path, err := rawStoragePath(uid)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", fmt.Errorf("创建订阅目录失败: %w", err)
	}
	return path, nil
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
	data, err := ReadRawFile(uid)
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("解析订阅原始配置失败: %w", err)
	}
	return &doc, nil
}

func ReadRawFile(uid string) ([]byte, error) {
	path, err := rawStoragePath(uid)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, ErrRawNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("读取订阅原始配置失败: %w", err)
	}
	return data, nil
}

// WriteRaw 原子写入 raw.yaml（覆盖前自动保留历史快照）。
func WriteRaw(uid string, data []byte) error {
	p, err := RawPath(uid)
	if err != nil {
		return err
	}
	if prev, rErr := os.ReadFile(p); rErr == nil && len(trimSpaceBytes(prev)) > 0 && !bytes.Equal(prev, data) {
		_, _ = SaveRawSnapshot(uid, prev, DefaultSnapshotRetention)
	}
	return atomicWrite(p, data, 0600)
}

// WriteMergeField modifies a specific field in the global merge.yaml file while preserving structure.
func WriteMergeField(key string, value interface{}) error {
	data, err := ReadMerge()
	if err != nil {
		return err
	}

	var root yaml.Node
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &root); err != nil {
			return err
		}
	}

	if len(root.Content) == 0 {
		root = yaml.Node{
			Kind: yaml.DocumentNode,
			Content: []*yaml.Node{
				{Kind: yaml.MappingNode},
			},
		}
	}

	mapping := root.Content[0]
	if mapping.Kind != yaml.MappingNode {
		mapping = &yaml.Node{Kind: yaml.MappingNode}
		root.Content[0] = mapping
	}

	found := false
	for i := 0; i < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			// Update existing
			node := yaml.Node{}
			if err := node.Encode(value); err != nil {
				return err
			}
			mapping.Content[i+1] = &node
			found = true
			break
		}
	}

	if !found {
		// Append new key-value pair
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
		valNode := &yaml.Node{}
		if err := valNode.Encode(value); err != nil {
			return err
		}
		mapping.Content = append(mapping.Content, keyNode, valNode)
	}

	out, err := marshalYAML2Spaces(&root)
	if err != nil {
		return err
	}

	return WriteMerge(out)
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
	return parseReadableMerge(data)
}

func parseReadableMerge(data []byte) (*yaml.Node, error) {
	doc, err := parseOverrideDocument(data)
	if err != nil {
		return nil, err
	}
	if err := validateMergeDocument(doc, true); err != nil {
		return nil, err
	}
	return doc, nil
}

// WriteMerge validates and atomically saves native Mihomo overrides, retaining
// the original bytes when replacing a legacy override file.
func WriteMerge(data []byte) error {
	doc, err := parseOverrideDocument(data)
	if err != nil {
		return fmt.Errorf("覆写 YAML 语法错误: %w", err)
	}
	if err := validateMergeDocument(doc, false); err != nil {
		return err
	}
	p, err := MergePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	previous, err := ReadMerge()
	if err != nil {
		return err
	}
	if old, err := parseOverrideDocument(previous); err == nil {
		root := topLevelMapping(old)
		for i := 0; i < len(root.Content); i += 2 {
			if legacyMergeTarget(root.Content[i].Value) != "" {
				backup := fmt.Sprintf("%s.legacy-%x.bak", p, sha256.Sum256(previous))
				if err := atomicWrite(backup, previous, 0600); err != nil {
					return fmt.Errorf("backup legacy merge.yaml: %w", err)
				}
				break
			}
		}
	}
	if err := PreserveLegacyRuleEdits(previous); err != nil {
		return err
	}
	if len(trimSpaceBytes(previous)) > 0 && !bytes.Equal(previous, data) {
		_, _ = SaveMergeSnapshot(previous, DefaultSnapshotRetention)
	}
	return atomicWrite(p, data, 0600)
}

// DeleteProfileDir 删除某订阅的整个目录（raw + 历史快照）。
// 目录不存在不报错。
func DeleteProfileDir(uid string) error {
	path, err := rawStoragePath(uid)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Dir(path)); err != nil {
		return fmt.Errorf("删除订阅目录失败: %w", err)
	}
	_ = DeleteRawSnapshots(uid)
	return nil
}

// atomicWrite 先写临时文件再 Rename 覆盖目标（避免半写损坏）。
// 与 config 包的 atomicWriteFile 行为一致。
func atomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".mihosh-profile-*")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // Rename 成功后已不存在，忽略错误

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("同步临时文件失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时文件失败: %w", err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return fmt.Errorf("设置临时文件权限失败: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("替换配置文件失败: %w", err)
	}
	return nil
}

// mihoshConfigDir 返回 ~/.mihosh 配置目录。
// （从 config 包取，但为避免循环依赖此处独立实现，逻辑与 config.GetConfigDir 一致。）
var mihoshConfigDir = func() (string, error) {
	if root := os.Getenv("MIHOSH_CONFIG_DIR"); root != "" {
		if !filepath.IsAbs(root) {
			return "", errors.New("MIHOSH_CONFIG_DIR must be an absolute path")
		}
		return filepath.Clean(root), nil
	}
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

// marshalYAML2Spaces 以 2 空格缩进序列化。
func marshalYAML2Spaces(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	err := enc.Encode(v)
	if err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
