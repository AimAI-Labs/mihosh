package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
)

const doctorProbeTimeout = 2 * time.Second

var doctorOutput string

var doctorCmd = &cobra.Command{
	Use:   "doctor [--output json|table|plain]",
	Short: "检查 Mihosh 配置和 Mihomo 连接健康状态",
	Long: `检查 Mihosh 配置和 Mihomo 连接健康状态。

连接信息（external-controller/secret/mixed-port）自动从 mihomo 配置文件发现。
检查项包括：external-controller、secret、代理端口（mixed-port 派生）、Mihomo REST API 可达性、WebSocket 可用性。

可通过 --output 选择输出格式：
  plain  人类可读文本（默认）
  table  表格输出
  json   结构化 JSON 输出`,
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
			return wrapConfigError(fmt.Errorf("加载配置失败: %w", err))
		}

		// 连接信息由 mihomo 配置文件自动发现
		endpoint := config.ResolveMihomoEndpoint()
		report := runDoctorChecks(cfg, endpoint)
		if err := renderDoctorReport(os.Stdout, report, format); err != nil {
			return fmt.Errorf("渲染输出失败: %w", err)
		}
		return doctorErrorForReport(report)
	},
}

func init() {
	doctorCmd.Flags().StringVar(&doctorOutput, "output", string(outputFormatPlain), "输出格式: json|table|plain")
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
		checkExternalController(endpoint.ExternalController),
		checkSecret(endpoint.Secret),
		checkProxyAddress(config.MixedPortToProxyURL(endpoint.MixedPort)),
		checkMihomoReachable(cfg, endpoint),
		checkWebSocketAvailable(endpoint),
	}
	return buildDoctorSummary(checks)
}

// checkExternalController 校验 external-controller 地址（原值，可能无 scheme）。
func checkExternalController(raw string) doctorCheckResult {
	normalized := ensureDoctorHTTPScheme(raw)
	if err := validateDoctorHTTPURL(normalized); err != nil {
		return doctorCheckResult{Name: "external_controller", Status: doctorStatusFail, Message: err.Error(), Target: raw}
	}
	return doctorCheckResult{Name: "external_controller", Status: doctorStatusOK, Message: "valid", Target: raw}
}

func checkSecret(secret string) doctorCheckResult {
	if strings.TrimSpace(secret) == "" {
		return doctorCheckResult{Name: "secret", Status: doctorStatusWarn, Message: "not configured"}
	}
	return doctorCheckResult{Name: "secret", Status: doctorStatusOK, Message: "configured"}
}

func checkProxyAddress(raw string) doctorCheckResult {
	target := raw
	normalized, err := normalizeDoctorProxyURL(raw)
	if err != nil {
		return doctorCheckResult{Name: "proxy", Status: doctorStatusFail, Message: err.Error(), Target: target}
	}

	if err := dialDoctorTCP(normalized.Host); err != nil {
		return doctorCheckResult{Name: "proxy", Status: doctorStatusFail, Message: err.Error(), Target: target}
	}
	return doctorCheckResult{Name: "proxy", Status: doctorStatusOK, Message: "reachable", Target: target}
}

func checkMihomoReachable(cfg *config.Config, endpoint config.MihomoEndpoint) doctorCheckResult {
	client := api.NewClient(endpoint, cfg.Timeout)
	if _, err := client.GetConfigs(); err != nil {
		return doctorCheckResult{Name: "mihomo", Status: doctorStatusFail, Message: err.Error(), Target: endpoint.ExternalController}
	}
	return doctorCheckResult{Name: "mihomo", Status: doctorStatusOK, Message: "reachable", Target: endpoint.ExternalController}
}

func checkWebSocketAvailable(endpoint config.MihomoEndpoint) doctorCheckResult {
	wsURL, err := buildDoctorWSURL(endpoint.ExternalController, endpoint.Secret, "traffic")
	if err != nil {
		return doctorCheckResult{Name: "websocket", Status: doctorStatusFail, Message: err.Error(), Target: endpoint.ExternalController}
	}

	dialer := websocket.Dialer{HandshakeTimeout: doctorProbeTimeout}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return doctorCheckResult{Name: "websocket", Status: doctorStatusFail, Message: err.Error(), Target: endpoint.ExternalController}
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
		return fmt.Errorf("不支持的输出格式: %s", format)
	}
}

func renderDoctorPlain(w io.Writer, report doctorReport) {
	fmt.Fprintf(w, "配置健康检查: %s\n", report.Status)
	fmt.Fprintf(w, "失败: %d, 警告: %d\n", report.Failed, report.Warnings)
	for _, check := range report.Checks {
		if check.Target == "" {
			fmt.Fprintf(w, "[%s] %s: %s\n", check.Status, check.Name, check.Message)
			continue
		}
		fmt.Fprintf(w, "[%s] %s (%s): %s\n", check.Status, check.Name, check.Target, check.Message)
	}
}

func renderDoctorTable(w io.Writer, report doctorReport) error {
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "CHECK\tSTATUS\tTARGET\tMESSAGE")
	for _, check := range report.Checks {
		target := check.Target
		if target == "" {
			target = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", check.Name, check.Status, target, check.Message)
	}
	return tw.Flush()
}

func doctorErrorForReport(report doctorReport) error {
	if report.Failed == 0 {
		return nil
	}
	return wrapNetworkError(fmt.Errorf("健康检查失败: %d 项失败", report.Failed))
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
