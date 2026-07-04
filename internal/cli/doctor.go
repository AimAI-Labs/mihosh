package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
)

const doctorProbeTimeout = 2 * time.Second

var doctorOutput string

var doctorCmd = &cobra.Command{
	Use:   "doctor [--output json|table|plain]",
	Example: `  mihosh doctor
  mihosh doctor --output table
  mihosh doctor --output json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		format, err := parseOutputFormat(doctorOutput)
		if err != nil {
			return wrapParameterError(err)
		}

		cfg, err := config.Load()
		if err != nil {
			if errors.Is(err, config.ErrConfigNotFound) {
				// 使用默认配置以便能继续测试
				cfg = &config.Config{Timeout: 5}
			} else {
				return wrapConfigError(fmt.Errorf(i18n.T("cli.doctor.err_load_config")+": %w", err))
			}
		}

		// 连接信息由 mihomo 配置文件自动发现
		endpoint := config.ResolveMihomoEndpoint()
		report := runDoctorChecks(cfg, endpoint)
		if err := renderDoctorReport(os.Stdout, report, format); err != nil {
			return fmt.Errorf(i18n.T("cli.doctor.err_render")+": %w", err)
		}
		return doctorErrorForReport(report)
	},
}

func init() {
	doctorCmd.Flags().StringVar(&doctorOutput, "output", string(outputFormatPlain), "")
}

type doctorStatus string

const (
	doctorStatusOK   doctorStatus = "ok"
	doctorStatusWarn doctorStatus = "warn"
	doctorStatusFail doctorStatus = "fail"
)

type doctorCheckResult struct {
	Name    string       `json:"name"`
	Status  doctorStatus `json:"status"`
	Message string       `json:"message"`
	Target  string       `json:"target,omitempty"`
	Hint    string       `json:"hint,omitempty"`
}

type doctorReport struct {
	Status   doctorStatus        `json:"status"`
	Healthy  bool                `json:"healthy"`
	Failed   int                 `json:"failed"`
	Warnings int                 `json:"warnings"`
	Checks   []doctorCheckResult `json:"checks"`
}

func runDoctorChecks(cfg *config.Config, endpoint config.MihomoEndpoint) doctorReport {
	checks := []doctorCheckResult{
		checkMihoshConfig(),
		checkMihomoConfig(),
		checkExternalController(endpoint.ExternalController),
		checkSecret(endpoint.Secret),
		checkProxyAddress(config.MixedPortToProxyURL(endpoint.MixedPort)),
		checkMihomoReachable(cfg, endpoint),
		checkWebSocketAvailable(endpoint),
	}
	return buildDoctorSummary(checks)
}

func checkMihoshConfig() doctorCheckResult {
	dir, err := config.GetConfigDir()
	if err != nil {
		return doctorCheckResult{Name: "mihosh_config", Status: doctorStatusFail, Message: err.Error(), Target: "-", Hint: "无法获取 Mihosh 配置目录，请检查系统权限或环境变量。"}
	}
	path := filepath.Join(dir, "config.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return doctorCheckResult{Name: "mihosh_config", Status: doctorStatusWarn, Message: "not found", Target: path, Hint: "Mihosh 配置文件不存在。这通常是因为这是首次运行，请使用 `mihosh config edit` 初始化配置。"}
	}
	return doctorCheckResult{Name: "mihosh_config", Status: doctorStatusOK, Message: "found", Target: path}
}

func checkMihomoConfig() doctorCheckResult {
	path, err := config.GetMihomoConfigPath()
	if err != nil {
		return doctorCheckResult{Name: "mihomo_config", Status: doctorStatusWarn, Message: "not found or error", Target: "-", Hint: "未能自动发现 Mihomo 的配置文件。请确保 Mihomo 已运行，或使用 `mihosh config edit` 手动指定配置目录。"}
	}
	return doctorCheckResult{Name: "mihomo_config", Status: doctorStatusOK, Message: "discovered", Target: path}
}

// checkExternalController 校验 external-controller 地址（原值，可能无 scheme）。
func checkExternalController(raw string) doctorCheckResult {
	normalized := ensureDoctorHTTPScheme(raw)
	if err := validateDoctorHTTPURL(normalized); err != nil {
		return doctorCheckResult{Name: "external_controller", Status: doctorStatusFail, Message: err.Error(), Target: raw, Hint: "Mihomo API 外部控制器地址无效。请检查是否为有效的 HTTP/HTTPS URL，例如 127.0.0.1:9090。"}
	}
	return doctorCheckResult{Name: "external_controller", Status: doctorStatusOK, Message: "valid", Target: raw}
}

func checkSecret(secret string) doctorCheckResult {
	if strings.TrimSpace(secret) == "" {
		return doctorCheckResult{Name: "secret", Status: doctorStatusWarn, Message: "not configured", Hint: "未配置 secret。如果您在公网或局域网暴露了 Mihomo API，强烈建议在 Mihomo 配置中设置 secret 以防止未授权访问。"}
	}
	return doctorCheckResult{Name: "secret", Status: doctorStatusOK, Message: "configured"}
}

func checkProxyAddress(raw string) doctorCheckResult {
	target := raw
	normalized, err := normalizeDoctorProxyURL(raw)
	if err != nil {
		return doctorCheckResult{Name: "proxy", Status: doctorStatusFail, Message: err.Error(), Target: target, Hint: "混合代理地址格式不正确。请检查 Mihomo 的 mixed-port 配置。"}
	}

	if err := dialDoctorTCP(normalized.Host); err != nil {
		return doctorCheckResult{Name: "proxy", Status: doctorStatusFail, Message: err.Error(), Target: target, Hint: "无法连接到混合代理端口。请确认 Mihomo 正在运行，且 mixed-port 未被防火墙拦截。"}
	}
	return doctorCheckResult{Name: "proxy", Status: doctorStatusOK, Message: "reachable", Target: target}
}

func checkMihomoReachable(cfg *config.Config, endpoint config.MihomoEndpoint) doctorCheckResult {
	client := api.NewClient(endpoint, cfg.Timeout)
	if _, err := client.GetConfigs(); err != nil {
		return doctorCheckResult{Name: "mihomo_api", Status: doctorStatusFail, Message: err.Error(), Target: endpoint.ExternalController, Hint: "无法访问 Mihomo API。请检查 Mihomo 是否已启动，且 external-controller 配置是否正确。若配置了 secret，请确保一致。"}
	}
	return doctorCheckResult{Name: "mihomo_api", Status: doctorStatusOK, Message: "reachable", Target: endpoint.ExternalController}
}

func checkWebSocketAvailable(endpoint config.MihomoEndpoint) doctorCheckResult {
	wsURL, err := buildDoctorWSURL(endpoint.ExternalController, endpoint.Secret, "traffic")
	if err != nil {
		return doctorCheckResult{Name: "websocket", Status: doctorStatusFail, Message: err.Error(), Target: endpoint.ExternalController, Hint: "WebSocket URL 构建失败，请检查 external-controller 地址是否合法。"}
	}

	dialer := websocket.Dialer{HandshakeTimeout: doctorProbeTimeout}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return doctorCheckResult{Name: "websocket", Status: doctorStatusFail, Message: err.Error(), Target: endpoint.ExternalController, Hint: "WebSocket 握手失败。可能的原因：API 地址错误、网络不通或 Mihomo 版本过低不支持此端点。"}
	}
	conn.Close()
	return doctorCheckResult{Name: "websocket", Status: doctorStatusOK, Message: "reachable", Target: endpoint.ExternalController}
}

// ensureDoctorHTTPScheme 为 external-controller 原值（无 scheme）补 http://，
// 供 doctor 校验/拼接 URL 时使用。
func ensureDoctorHTTPScheme(raw string) string {
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	return "http://" + raw
}

func buildDoctorSummary(checks []doctorCheckResult) doctorReport {
	report := doctorReport{
		Status:  doctorStatusOK,
		Healthy: true,
		Checks:  checks,
	}
	for _, check := range checks {
		switch check.Status {
		case doctorStatusFail:
			report.Failed++
		case doctorStatusWarn:
			report.Warnings++
		}
	}
	if report.Failed > 0 {
		report.Status = doctorStatusFail
		report.Healthy = false
		return report
	}
	if report.Warnings > 0 {
		report.Status = doctorStatusWarn
	}
	return report
}

func renderDoctorReport(w io.Writer, report doctorReport, format outputFormat) error {
	switch format {
	case outputFormatJSON:
		return writeJSON(w, report)
	case outputFormatTable:
		return renderDoctorTable(w, report)
	case outputFormatPlain:
		renderDoctorPlain(w, report)
		return nil
	default:
		return fmt.Errorf(i18n.T("cli.doctor.unsupported_format"), format)
	}
}

func renderDoctorPlain(w io.Writer, report doctorReport) {
	fmt.Fprintf(w, "%s: %s\n", i18n.T("cli.doctor.label_health"), report.Status)
	fmt.Fprintln(w, i18n.Tf("cli.doctor.label_fail_warn", report.Failed, report.Warnings))
	for _, check := range report.Checks {
		targetStr := ""
		if check.Target != "" && check.Target != "-" {
			targetStr = fmt.Sprintf(" (%s)", check.Target)
		}
		fmt.Fprintf(w, "[%s] %s%s: %s\n", strings.ToUpper(string(check.Status)), check.Name, targetStr, check.Message)
		if check.Hint != "" {
			fmt.Fprintf(w, "      %s: %s\n", i18n.T("cli.doctor.label_hint"), check.Hint)
		}
	}
}

func renderDoctorTable(w io.Writer, report doctorReport) error {
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "CHECK\tSTATUS\tTARGET\tMESSAGE\tHINT")
	for _, check := range report.Checks {
		target := check.Target
		if target == "" {
			target = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", check.Name, strings.ToUpper(string(check.Status)), target, check.Message, check.Hint)
	}
	return tw.Flush()
}

func doctorErrorForReport(report doctorReport) error {
	if report.Failed == 0 {
		return nil
	}
	return wrapNetworkError(fmt.Errorf("%s", i18n.Tf("cli.doctor.err_health_fail", report.Failed)))
}

func validateDoctorHTTPURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("必须使用 http 或 https 协议")
	}
	if parsed.Host == "" {
		return errors.New("缺少主机地址")
	}
	return nil
}

func normalizeDoctorProxyURL(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("代理地址为空")
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "http://" + trimmed
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, err
	}
	switch parsed.Scheme {
	case "http", "https", "socks5":
	default:
		return nil, errors.New("代理地址必须使用 http、https 或 socks5 协议")
	}
	if parsed.Host == "" {
		return nil, errors.New("缺少代理主机地址")
	}
	if parsed.Port() == "" {
		return nil, errors.New("缺少代理端口")
	}
	return parsed, nil
}

func dialDoctorTCP(address string) error {
	ctx, cancel := context.WithTimeout(context.Background(), doctorProbeTimeout)
	defer cancel()

	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	return conn.Close()
}

func buildDoctorWSURL(baseURL, secret, endpoint string) (string, error) {
	// external-controller 原值无 scheme，先补 http:// 再校验/转换。
	normalized := ensureDoctorHTTPScheme(baseURL)
	if err := validateDoctorHTTPURL(normalized); err != nil {
		return "", err
	}

	parsed, err := url.Parse(normalized)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "https" {
		parsed.Scheme = "wss"
	} else {
		parsed.Scheme = "ws"
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/" + endpoint
	if secret != "" {
		q := parsed.Query()
		q.Set("token", secret)
		parsed.RawQuery = q.Encode()
	}
	return parsed.String(), nil
}
