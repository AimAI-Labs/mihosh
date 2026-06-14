package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/stretchr/testify/assert"
)

// withEnvLookup 替换 osLookupEnv 以模拟环境变量状态，并在结束时恢复。
// 直接替换 package var（参照 service_test.go 替换 runSystemCommandFn 的做法）。
func withEnvLookup(t *testing.T, env map[string]string) {
	t.Helper()
	orig := osLookupEnv
	osLookupEnv = func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	}
	t.Cleanup(func() { osLookupEnv = orig })
}

// initI18nForTest 确保 i18n 字典已加载（Tf 需要 dict 才能格式化消息）。
// i18n.Init() 是幂等的，重复调用安全。
func initI18nForTest(t *testing.T) {
	t.Helper()
	i18n.Init()
}

func TestDetectProxyState(t *testing.T) {
	tests := []struct {
		name      string
		env       map[string]string
		wantState proxyState
		wantVal   string
	}{
		{
			name:      "none set",
			env:       map[string]string{},
			wantState: proxyStateNone,
			wantVal:   "",
		},
		{
			name: "all set and equal",
			env: map[string]string{
				"HTTP_PROXY":  "http://127.0.0.1:7890",
				"HTTPS_PROXY": "http://127.0.0.1:7890",
				"ALL_PROXY":   "http://127.0.0.1:7890",
			},
			wantState: proxyStateFull,
			wantVal:   "http://127.0.0.1:7890",
		},
		{
			name: "only HTTP set",
			env: map[string]string{
				"HTTP_PROXY": "http://127.0.0.1:7890",
			},
			wantState: proxyStatePartial,
			wantVal:   "http://127.0.0.1:7890",
		},
		{
			name: "set but inconsistent",
			env: map[string]string{
				"HTTP_PROXY":  "http://127.0.0.1:7890",
				"HTTPS_PROXY": "http://127.0.0.1:7891",
				"ALL_PROXY":   "http://127.0.0.1:7890",
			},
			wantState: proxyStatePartial,
			wantVal:   "http://127.0.0.1:7890",
		},
		{
			name: "HTTP empty, HTTPS set",
			env: map[string]string{
				"HTTP_PROXY":  "",
				"HTTPS_PROXY": "socks5://127.0.0.1:1080",
			},
			wantState: proxyStatePartial,
			wantVal:   "socks5://127.0.0.1:1080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withEnvLookup(t, tt.env)
			state, val := detectProxyState()
			assert.Equal(t, tt.wantState, state)
			assert.Equal(t, tt.wantVal, val)
		})
	}
}

func TestDetectProxyState_DoesNotPanicOnEmpty(t *testing.T) {
	// 防御性测试：无任何代理变量时不应 panic、返回 None。
	withEnvLookup(t, nil)
	state, val := detectProxyState()
	assert.Equal(t, proxyStateNone, state)
	assert.Empty(t, val)
}

func TestRenderProxyOnStatements(t *testing.T) {
	addr := "http://127.0.0.1:7890"
	noProxy := "localhost,127.0.0.1,::1"

	t.Run("bash/zsh export with single quotes", func(t *testing.T) {
		var out bytes.Buffer
		renderProxyOnStatements(&out, addr, noProxy)
		s := out.String()

		assert.Contains(t, s, "export HTTP_PROXY='http://127.0.0.1:7890'")
		assert.Contains(t, s, "export HTTPS_PROXY='http://127.0.0.1:7890'")
		assert.Contains(t, s, "export ALL_PROXY='http://127.0.0.1:7890'")
		assert.Contains(t, s, "export http_proxy='http://127.0.0.1:7890'")
		assert.Contains(t, s, "export https_proxy='http://127.0.0.1:7890'")
		assert.Contains(t, s, "export all_proxy='http://127.0.0.1:7890'")
		assert.Contains(t, s, "export NO_PROXY='localhost,127.0.0.1,::1'")
		assert.Contains(t, s, "export no_proxy='localhost,127.0.0.1,::1'")
		// 每行一个变量，共 8 个。
		assert.Equal(t, 8, strings.Count(s, "\n"))
	})

	t.Run("value with single quote is escaped", func(t *testing.T) {
		var out bytes.Buffer
		// 含单引号的代理地址（如带密码的场景）必须被正确转义。
		renderProxyOnStatements(&out, "http://a'b:1234@127.0.0.1:7890", noProxy)
		s := out.String()
		// POSIX 转义：'a'b → 'a'\''b'，整个值再用单引号包裹。
		assert.Contains(t, s, "'http://a'\\''b:1234@127.0.0.1:7890'")
	})
}

func TestRenderProxyOffStatements(t *testing.T) {
	t.Run("bash/zsh unset", func(t *testing.T) {
		var out bytes.Buffer
		renderProxyOffStatements(&out)
		s := out.String()

		for _, name := range proxyEnvVars {
			assert.Contains(t, s, "unset "+name)
		}
		assert.Contains(t, s, "unset NO_PROXY")
		assert.Contains(t, s, "unset no_proxy")
		// 8 个变量 = 8 行。
		assert.Equal(t, 8, strings.Count(s, "\n"))
	})
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain url", in: "http://127.0.0.1:7890", want: "'http://127.0.0.1:7890'"},
		{name: "socks5 url", in: "socks5://127.0.0.1:1080", want: "'socks5://127.0.0.1:1080'"},
		{name: "empty", in: "", want: "''"},
		{name: "with single quote", in: "a'b", want: "'a'\\''b'"},
		{name: "multiple single quotes", in: "'a'b'", want: "''\\''a'\\''b'\\'''"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, shellQuote(tt.in))
		})
	}
}

// ===== writeProxyOn / writeProxyOff 端到端：状态检测 + stdout/stderr 分离 =====

func TestWriteProxyOn_AlreadyOn_FriendlyHint(t *testing.T) {
	initI18nForTest(t)
	withEnvLookup(t, map[string]string{
		"HTTP_PROXY":  "http://127.0.0.1:7890",
		"HTTPS_PROXY": "http://127.0.0.1:7890",
		"ALL_PROXY":   "http://127.0.0.1:7890",
	})

	var stdout, stderr bytes.Buffer
	writeProxyOn(&stdout, &stderr, "http://127.0.0.1:7890")

	// stderr 应包含"已开启"友好提示（zh-CN 或 en-US 任一命中即可，取决于测试机语言）。
	errOut := stderr.String()
	assert.True(t,
		strings.Contains(errOut, "已开启代理") || strings.Contains(errOut, "already enabled"),
		"stderr should contain already-on hint, got: %s", errOut)

	// stdout 仍应包含 eval 语句（幂等输出）。
	assert.Contains(t, stdout.String(), "export HTTP_PROXY='http://127.0.0.1:7890'")
}

func TestWriteProxyOn_PartialOn_OverrideHint(t *testing.T) {
	initI18nForTest(t)
	withEnvLookup(t, map[string]string{
		"HTTP_PROXY": "http://127.0.0.1:7890",
	})

	var stdout, stderr bytes.Buffer
	writeProxyOn(&stdout, &stderr, "http://127.0.0.1:7890")

	errOut := stderr.String()
	assert.True(t,
		strings.Contains(errOut, "部分开启") || strings.Contains(errOut, "partially enabled"),
		"stderr should contain partial-override hint, got: %s", errOut)
	assert.Contains(t, stdout.String(), "export HTTP_PROXY='http://127.0.0.1:7890'")
}

func TestWriteProxyOn_NoneSet_NoStderrHint(t *testing.T) {
	initI18nForTest(t)
	withEnvLookup(t, map[string]string{})

	var stdout, stderr bytes.Buffer
	writeProxyOn(&stdout, &stderr, "http://127.0.0.1:7890")

	// 未开启时 stderr 应为空（无状态提示可报）。
	assert.Empty(t, stderr.String())
	// stdout 仍有 eval 语句。
	assert.Contains(t, stdout.String(), "export HTTP_PROXY='http://127.0.0.1:7890'")
}

func TestWriteProxyOff_NoneSet_FriendlyHint(t *testing.T) {
	initI18nForTest(t)
	withEnvLookup(t, map[string]string{})

	var stdout, stderr bytes.Buffer
	writeProxyOff(&stdout, &stderr)

	errOut := stderr.String()
	assert.True(t,
		strings.Contains(errOut, "未开启代理") || strings.Contains(errOut, "nothing to disable"),
		"stderr should contain no-proxy hint, got: %s", errOut)
	// off 仍然输出 unset 语句（幂等）。
	assert.Contains(t, stdout.String(), "unset HTTP_PROXY")
}

func TestWriteProxyOff_Active_WillOffHint(t *testing.T) {
	initI18nForTest(t)
	withEnvLookup(t, map[string]string{
		"HTTP_PROXY":  "http://127.0.0.1:7890",
		"HTTPS_PROXY": "http://127.0.0.1:7890",
		"ALL_PROXY":   "http://127.0.0.1:7890",
	})

	var stdout, stderr bytes.Buffer
	writeProxyOff(&stdout, &stderr)

	errOut := stderr.String()
	assert.True(t,
		strings.Contains(errOut, "即将关闭") || strings.Contains(errOut, "will disable"),
		"stderr should contain will-off hint, got: %s", errOut)
	assert.Contains(t, stdout.String(), "unset HTTP_PROXY")
}

// stdout 必须"纯净"：不含任何 ANSI 转义或人类提示，确保 eval 安全。
func TestWriteProxyOn_StdoutIsEvalSafe(t *testing.T) {
	initI18nForTest(t)
	// 设置一个会触发 stderr 提示的状态，验证 stdout 不被污染。
	withEnvLookup(t, map[string]string{
		"HTTP_PROXY":  "http://127.0.0.1:7890",
		"HTTPS_PROXY": "http://127.0.0.1:7890",
		"ALL_PROXY":   "http://127.0.0.1:7890",
	})

	var stdout, stderr bytes.Buffer
	writeProxyOn(&stdout, &stderr, "http://127.0.0.1:7890")

	// stdout 不应包含 ANSI 转义序列（提示文字用的彩色样式不应泄漏到 stdout）。
	assert.NotContains(t, stdout.String(), "\x1b[")
	// stderr 提示可以包含 ANSI（lipgloss 着色），这是预期的。
	assert.NotEmpty(t, stderr.String())
}
