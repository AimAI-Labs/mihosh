package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestStore_RawRoundTrip 验证 raw.yaml 的写入与读取往返。
func TestStore_RawRoundTrip(t *testing.T) {
	uid := "test-raw-rt"
	t.Cleanup(func() { _ = DeleteProfileDir(uid) })

	// 不存在时应返回 ErrRawNotFound
	_, err := ReadRaw(uid)
	assert.ErrorIs(t, err, ErrRawNotFound)

	content := []byte("mode: rule\nrules:\n  - MATCH,DIRECT\n")
	require.NoError(t, WriteRaw(uid, content))

	doc, err := ReadRaw(uid)
	require.NoError(t, err)
	require.NotNil(t, doc)
	// 解析回来内容一致
	var out map[string]interface{}
	parsed, err := yaml.Marshal(topLevelMapping(doc))
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(parsed, &out))
	assert.Equal(t, "rule", out["mode"])
}

// TestStore_MergeRoundTrip 验证 merge.yaml 的写入与读取，以及空文件降级。
func TestStore_MergeRoundTrip(t *testing.T) {
	uid := "test-merge-rt"
	t.Cleanup(func() { _ = DeleteProfileDir(uid) })

	// 不存在时返回 nil（无覆写），不报错
	data, err := ReadMerge(uid)
	require.NoError(t, err)
	assert.Nil(t, data)

	node, err := ReadMergeNode(uid)
	require.NoError(t, err)
	assert.Nil(t, node)

	// 写入合法 merge
	merge := []byte("prepend-rules:\n  - DOMAIN,a.com,DIRECT\n")
	require.NoError(t, WriteMerge(uid, merge))

	data, err = ReadMerge(uid)
	require.NoError(t, err)
	assert.Equal(t, merge, data)

	node, err = ReadMergeNode(uid)
	require.NoError(t, err)
	require.NotNil(t, node)

	// 写入空内容（应允许，覆盖为空）
	require.NoError(t, WriteMerge(uid, nil))
	data, err = ReadMerge(uid)
	require.NoError(t, err)
	assert.Nil(t, data)
}

// TestStore_WriteMergeInvalidYAML 验证非法 YAML 不写盘。
func TestStore_WriteMergeInvalidYAML(t *testing.T) {
	uid := "test-merge-invalid"
	t.Cleanup(func() { _ = DeleteProfileDir(uid) })

	// 先写一个合法 merge
	require.NoError(t, WriteMerge(uid, []byte("mode: rule\n")))

	// 尝试写非法 YAML（未闭合的 flow）
	invalid := []byte("mode: {broken: \n")
	err := WriteMerge(uid, invalid)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "语法错误")

	// 原内容应未被覆盖
	data, err := ReadMerge(uid)
	require.NoError(t, err)
	assert.Contains(t, string(data), "mode: rule")
}

// TestStore_DeleteProfileDir 验证目录删除。
func TestStore_DeleteProfileDir(t *testing.T) {
	uid := "test-delete"
	require.NoError(t, WriteRaw(uid, []byte("mode: rule\n")))

	rawP, err := RawPath(uid)
	require.NoError(t, err)
	require.True(t, fileExists(rawP))

	require.NoError(t, DeleteProfileDir(uid))
	assert.False(t, fileExists(rawP))

	// 重复删除不报错
	require.NoError(t, DeleteProfileDir(uid))
}

// TestStore_DirIsolation 验证不同 UID 目录相互隔离。
func TestStore_DirIsolation(t *testing.T) {
	uidA, uidB := "iso-a", "iso-b"
	t.Cleanup(func() {
		_ = DeleteProfileDir(uidA)
		_ = DeleteProfileDir(uidB)
	})

	require.NoError(t, WriteRaw(uidA, []byte("mode: rule\n")))
	require.NoError(t, WriteRaw(uidB, []byte("mode: global\n")))

	pa, err := RawPath(uidA)
	require.NoError(t, err)
	pb, err := RawPath(uidB)
	require.NoError(t, err)
	assert.NotEqual(t, pa, pb)

	a, err := ReadRaw(uidA)
	require.NoError(t, err)
	b, err := ReadRaw(uidB)
	require.NoError(t, err)
	assert.Equal(t, "rule", topLevelMapping(a).Content[1].Value)
	assert.Equal(t, "global", topLevelMapping(b).Content[1].Value)
}

// TestStore_ProfileDirCreation 验证 RawPath/MergePath 会自动创建目录。
func TestStore_ProfileDirCreation(t *testing.T) {
	uid := "test-dir-create"
	t.Cleanup(func() { _ = DeleteProfileDir(uid) })

	p, err := RawPath(uid)
	require.NoError(t, err)
	dir := filepath.Dir(p)
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
