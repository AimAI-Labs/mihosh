package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// noProxyBypass 是设置代理时排除的本地地址，避免本地请求走代理。
const noProxyBypass = "localhost,127.0.0.1,::1"

// proxyEnvVars 是开/关代理需要操作的变量名集合（大写 + 小写）。
// 大写为多数 CLI 工具约定，小写为 curl 等工具识别的写法，两者都设保证兼容。
var proxyEnvVars = []string{
	"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY",
	"http_proxy", "https_proxy", "all_proxy",
}

// noProxyEnvVars 是开/关时需要操作的 no_proxy 变量名。
var noProxyEnvVars = []string{"NO_PROXY", "no_proxy"}

// osLookupEnv 暴露为 package var，便于测试中替换 env 读取行为。
var osLookupEnv = os.LookupEnv

// proxyState 描述当前终端的代理状态。
type proxyState int

const (
	proxyStateNone    proxyState = iota // 无任何代理变量
	proxyStateFull                      // HTTP/HTTPS/ALL 全部设置且一致
	proxyStatePartial                   // 部分设置或值不一致
)

// detectProxyState 检查当前进程的代理环境变量，推断终端的代理状态。
// only 大写变量用于状态判定（小写与大写在 Windows 下等价，避免重复计数）。
func detectProxyState() (proxyState, string) {
	httpProxy, hasHTTP := osLookupEnv("HTTP_PROXY")
	httpsProxy, hasHTTPS := osLookupEnv("HTTPS_PROXY")
	allProxy, hasAll := osLookupEnv("ALL_PROXY")

	if !hasHTTP && !hasHTTPS && !hasAll {
		return proxyStateNone, ""
	}

	// 全部设置且值一致 → Full；否则 Partial。
	if hasHTTP && hasHTTPS && hasAll && httpProxy == httpsProxy && httpsProxy == allProxy {
		return proxyStateFull, httpProxy
	}

	// 取首个非空值作为展示用。
	first := httpProxy
	if first == "" {
		if httpsProxy != "" {
			first = httpsProxy
		} else {
			first = allProxy
		}
	}
	return proxyStatePartial, first
}

// proxySuccessStyle / proxyHintStyle 复用 errors.go 的 lipgloss 配色思路：
// 成功用绿色，提示用灰色，与项目既有错误渲染风格一致。
func proxySuccessStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
}

func proxyHintStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
}

var proxyOnCmd = &cobra.Command{
	Use:   "on",
	Short: "在当前终端开启代理",
	Long: `输出在当前终端开启代理所需的 bash/zsh 语句（写到 stdout）。

子进程无法直接修改父 Shell 的环境变量，因此请用 eval 执行输出：

  eval "$(mihosh on)"

默认代理地址由 mihomo 配置文件的 mixed-port 派生（http://127.0.0.1:<mixed-port>），
如需修改请在 mihomo 配置中调整 mixed-port。
设置的环境变量：HTTP_PROXY / HTTPS_PROXY / ALL_PROXY（大小写各一组）+ no_proxy。
若当前终端已开启代理，会给出友好提示。

仅支持 bash / zsh（Linux 与 macOS）。`,
	Example: `  eval "$(mihosh on)"`,
	RunE:    runProxyOn,
}

var proxyOffCmd = &cobra.Command{
	Use:   "off",
	Short: "在当前终端关闭代理",
	Long: `输出在当前终端关闭代理所需的 bash/zsh 语句（写到 stdout）。

子进程无法直接修改父 Shell 的环境变量，因此请用 eval 执行输出：

  eval "$(mihosh off)"

会清除 HTTP_PROXY / HTTPS_PROXY / ALL_PROXY / no_proxy（大小写各一组）。
若当前终端未开启代理，会给出友好提示。

仅支持 bash / zsh（Linux 与 macOS）。`,
	Example: `  eval "$(mihosh off)"`,
	RunE:    runProxyOff,
}

func runProxyOn(cmd *cobra.Command, args []string) error {
	// 代理地址由 mihomo 配置文件的 mixed-port 派生；自动发现失败回退默认端口。
	// 先确保配置目录存在（首次运行）：on 是"开关"型命令，不应因缺配置卡住。
	if _, err := config.Load(); err != nil {
		if !errors.Is(err, config.ErrConfigNotFound) {
			return wrapConfigError(fmt.Errorf(i18n.T("cli.root.err_load_config")+": %w", err))
		}
	}

	addr := config.MixedPortToProxyURL(config.ResolveMihomoEndpoint().MixedPort)

	// 复用 doctor.go 的代理地址规范化：自动补 scheme、校验端口与协议。
	parsed, err := normalizeDoctorProxyURL(addr)
	if err != nil {
		if strings.TrimSpace(addr) == "" {
			return wrapParameterError(fmt.Errorf("%s", i18n.T("cli.proxy.empty_address")))
		}
		return wrapParameterError(fmt.Errorf(i18n.T("cli.proxy.invalid_address"), addr))
	}
	addr = parsed.String()

	writeProxyOn(os.Stdout, os.Stderr, addr)
	return nil
}

func runProxyOff(cmd *cobra.Command, args []string) error {
	writeProxyOff(os.Stdout, os.Stderr)
	return nil
}

// writeProxyOn 写出 on 命令的全部输出：eval 语句到 stdout（供 eval 使用），
// 状态检测的友好提示到 stderr（供用户阅读，不污染 stdout）。
func writeProxyOn(stdout, stderr io.Writer, addr string) {
	// 状态检测 → 友好提示写到 stderr。
	state, current := detectProxyState()
	switch state {
	case proxyStateFull:
		fmt.Fprintln(stderr, proxySuccessStyle().Render(i18n.Tf("cli.proxy.already_on", current)))
	case proxyStatePartial:
		fmt.Fprintln(stderr, proxyHintStyle().Render(i18n.Tf("cli.proxy.partial_on", addr)))
	}

	// eval 语句 → stdout。
	renderProxyOnStatements(stdout, addr, noProxyBypass)
}

// writeProxyOff 写出 off 命令的全部输出。
func writeProxyOff(stdout, stderr io.Writer) {
	// 状态检测 → 友好提示。
	state, current := detectProxyState()
	switch state {
	case proxyStateNone:
		fmt.Fprintln(stderr, proxySuccessStyle().Render(i18n.T("cli.proxy.not_on")))
	default: // Full 或 Partial 都提示"即将关闭"。
		fmt.Fprintln(stderr, proxyHintStyle().Render(i18n.Tf("cli.proxy.will_off", current)))
	}

	// 总是输出 unset 语句（幂等：未设置时 unset 也无副作用）。
	renderProxyOffStatements(stdout)
}

// renderProxyOnStatements 写出 bash/zsh 下设置代理环境变量的语句。
// bash 与 zsh 的 export/unset 语法完全兼容，输出统一。
func renderProxyOnStatements(w io.Writer, addr, noProxy string) {
	for _, name := range proxyEnvVars {
		fmt.Fprintf(w, "export %s=%s\n", name, shellQuote(addr))
	}
	for _, name := range noProxyEnvVars {
		fmt.Fprintf(w, "export %s=%s\n", name, shellQuote(noProxy))
	}
}

// renderProxyOffStatements 写出 bash/zsh 下清除代理环境变量的语句。
func renderProxyOffStatements(w io.Writer) {
	allVars := append(append([]string{}, proxyEnvVars...), noProxyEnvVars...)
	for _, name := range allVars {
		fmt.Fprintf(w, "unset %s\n", name)
	}
}

// shellQuote 用单引号包裹值并转义内部单引号。
// 单引号比双引号更安全：避免 $、` 等被 shell 解释。
// POSIX sh 单引号转义：' → '\''（关闭单引号、转义单引号、重开单引号）。
func shellQuote(s string) string {
	escaped := strings.ReplaceAll(s, "'", "'\\''")
	return "'" + escaped + "'"
}
