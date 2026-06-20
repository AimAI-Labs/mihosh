package profile

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFetch_Local 读取本地文件并写入 raw.yaml。
func TestFetch_Local(t *testing.T) {
	uid := "fetch-local"
	t.Cleanup(func() { _ = DeleteProfileDir(uid) })

	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "sub.yaml")
	require.NoError(t, os.WriteFile(srcPath, []byte("mode: rule\nrules:\n  - MATCH,DIRECT\n"), 0644))

	p := Profile{UID: uid, Source: SubSource{Kind: SourceLocal, Path: srcPath}}
	require.NoError(t, Fetch(p))

	raw, err := ReadRaw(uid)
	require.NoError(t, err)
	require.NotNil(t, raw)
}

// TestFetch_LocalMissingFile 本地路径不存在时报错。
func TestFetch_LocalMissingFile(t *testing.T) {
	uid := "fetch-local-missing"
	t.Cleanup(func() { _ = DeleteProfileDir(uid) })

	p := Profile{UID: uid, Source: SubSource{Kind: SourceLocal, Path: "/nonexistent/path/x.yaml"}}
	err := Fetch(p)
	require.Error(t, err)
}

// TestFetch_Remote 远程拉取正常响应。
func TestFetch_Remote(t *testing.T) {
	uid := "fetch-remote"
	t.Cleanup(func() { _ = DeleteProfileDir(uid) })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "mihosh/1.0", r.Header.Get("User-Agent"))
		w.Header().Set("Content-Type", "text/yaml")
		_, _ = w.Write([]byte("mode: rule\nproxies: []\n"))
	}))
	defer srv.Close()

	p := Profile{UID: uid, Source: SubSource{Kind: SourceRemote, URL: srv.URL}}
	require.NoError(t, Fetch(p))

	raw, err := ReadRaw(uid)
	require.NoError(t, err)
	require.NotNil(t, raw)
}

// TestFetch_RemoteNon2xx 非 2xx 报错且不写 raw。
func TestFetch_RemoteNon2xx(t *testing.T) {
	uid := "fetch-remote-err"
	t.Cleanup(func() { _ = DeleteProfileDir(uid) })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := Profile{UID: uid, Source: SubSource{Kind: SourceRemote, URL: srv.URL}}
	err := Fetch(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")

	// raw 不应被写入
	_, err = ReadRaw(uid)
	assert.ErrorIs(t, err, ErrRawNotFound)
}

// TestFetch_RemoteInvalidYAML 非 YAML 内容（如 HTML 错误页）被拒绝。
func TestFetch_RemoteInvalidYAML(t *testing.T) {
	uid := "fetch-remote-invalid"
	t.Cleanup(func() { _ = DeleteProfileDir(uid) })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html><body>not yaml</body></html>"))
	}))
	defer srv.Close()

	p := Profile{UID: uid, Source: SubSource{Kind: SourceRemote, URL: srv.URL}}
	err := Fetch(p)
	require.Error(t, err)
	// html 实际可能被部分解析，但至少非合法 yaml 应被 yaml.Unmarshal 拦截或保留。
	// 关键：失败时 raw 不被写入。
	_, readErr := ReadRaw(uid)
	assert.ErrorIs(t, readErr, ErrRawNotFound)
}

// TestFetch_RemoteInvalidScheme 非 http(s) 协议被拒绝（防 file:// 绕过）。
func TestFetch_RemoteInvalidScheme(t *testing.T) {
	uid := "fetch-remote-scheme"
	t.Cleanup(func() { _ = DeleteProfileDir(uid) })

	p := Profile{UID: uid, Source: SubSource{Kind: SourceRemote, URL: "file:///etc/passwd"}}
	err := Fetch(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http/https")
}

// TestFetch_RemoteEmptyURL 空 URL 报错。
func TestFetch_RemoteEmptyURL(t *testing.T) {
	uid := "fetch-remote-empty"
	t.Cleanup(func() { _ = DeleteProfileDir(uid) })

	p := Profile{UID: uid, Source: SubSource{Kind: SourceRemote, URL: "  "}}
	err := Fetch(p)
	require.Error(t, err)
}

// TestParseSourceKind 验证来源自动判断。
func TestParseSourceKind(t *testing.T) {
	assert.Equal(t, SourceRemote, ParseSourceKind("https://example.com/sub"))
	assert.Equal(t, SourceRemote, ParseSourceKind("http://localhost:8080/sub"))
	assert.Equal(t, SourceLocal, ParseSourceKind("/path/to/config.yaml"))
	assert.Equal(t, SourceLocal, ParseSourceKind("./relative/config.yaml"))
	assert.Equal(t, SourceLocal, ParseSourceKind("config.yaml"))
}

// TestSourceKindString 验证可读名称。
func TestSourceKindString(t *testing.T) {
	assert.Equal(t, "remote", SourceRemote.String())
	assert.Equal(t, "local", SourceLocal.String())
}

// TestSubSourceDisplay 验证 Display 按类型返回对应字段。
func TestSubSourceDisplay(t *testing.T) {
	assert.Equal(t, "https://x.io/s", SubSource{Kind: SourceRemote, URL: "https://x.io/s"}.Display())
	assert.Equal(t, "/p/c.yaml", SubSource{Kind: SourceLocal, Path: "/p/c.yaml"}.Display())
}

// TestFetch_PreservesOldRawOnError 本地读取失败时不覆盖已有 raw。
func TestFetch_PreservesOldRawOnError(t *testing.T) {
	uid := "fetch-preserve"
	t.Cleanup(func() { _ = DeleteProfileDir(uid) })

	// 先写入合法 raw
	require.NoError(t, WriteRaw(uid, []byte("mode: rule\n")))

	// 指向不存在本地路径，应失败
	p := Profile{UID: uid, Source: SubSource{Kind: SourceLocal, Path: "/no/such/file.yaml"}}
	err := Fetch(p)
	require.Error(t, err)

	// 旧 raw 应仍存在
	raw, err := ReadRaw(uid)
	require.NoError(t, err)
	require.NotNil(t, raw)
}

// ensure fetchTimeout is used (compile-time reference to avoid unused)
var _ = fetchTimeout
var _ = time.Second
