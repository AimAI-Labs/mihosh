package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
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
	Use: "doctor [--output json|table|plain]",
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
				// 使用默认配置（Timeout 为毫秒，不可硬编码为 5）以便能继续测试
				defaults := config.DefaultConfig
				cfg = &defaults
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
	doctorCmd.Flags().StringVarP(&doctorOutput, "output", "o", string(outputFormatPlain), "")
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
		return doctorCheckResult{Name: "mihosh_config", Status: doctorStatusFail, Message: err.Error(), Target: "-", Hint: i18n.T("cli.doctor.hint_config_dir")}
	}
	path := filepath.Join(dir, "config.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return doctorCheckResult{Name: "mihosh_config", Status: doctorStatusWarn, Message: "not found", Target: path, Hint: i18n.T("cli.doctor.hint_config_missing")}
	}
	return doctorCheckResult{Name: "mihosh_config", Status: doctorStatusOK, Message: "found", Target: path}
}

func checkMihomoConfig() doctorCheckResult {
	path, err := config.GetMihomoConfigPath()
	if err != nil {
		return doctorCheckResult{Name: "mihomo_config", Status: doctorStatusWarn, Message: "not found or error", Target: "-", Hint: i18n.T("cli.doctor.hint_mihomo_config")}
	}
	return doctorCheckResult{Name: "mihomo_config", Status: doctorStatusOK, Message: "discovered", Target: path}
}

// checkExternalController 校验 external-controller 地址（原值，可能无 scheme）。
func checkExternalController(raw string) doctorCheckResult {
	normalized := api.ControllerHTTPURL(raw)
	if err := validateDoctorHTTPURL(normalized); err != nil {
		return doctorCheckResult{Name: "external_controller", Status: doctorStatusFail, Message: err.Error(), Target: raw, Hint: i18n.T("cli.doctor.hint_controller")}
	}
	return doctorCheckResult{Name: "external_controller", Status: doctorStatusOK, Message: "valid", Target: raw}
}

func checkSecret(secret string) doctorCheckResult {
	if strings.TrimSpace(secret) == "" {
		return doctorCheckResult{Name: "secret", Status: doctorStatusWarn, Message: "not configured", Hint: i18n.T("cli.doctor.hint_secret")}
	}
	return doctorCheckResult{Name: "secret", Status: doctorStatusOK, Message: "configured"}
}

func checkProxyAddress(raw string) doctorCheckResult {
	target := raw
	normalized, err := normalizeDoctorProxyURL(raw)
	if err != nil {
		return doctorCheckResult{Name: "proxy", Status: doctorStatusFail, Message: err.Error(), Target: target, Hint: i18n.T("cli.doctor.hint_proxy_invalid")}
	}

	if err := dialDoctorTCP(normalized.Host); err != nil {
		return doctorCheckResult{Name: "proxy", Status: doctorStatusFail, Message: err.Error(), Target: target, Hint: i18n.T("cli.doctor.hint_proxy_unreachable")}
	}
	return doctorCheckResult{Name: "proxy", Status: doctorStatusOK, Message: "reachable", Target: target}
}

func checkMihomoReachable(cfg *config.Config, endpoint config.MihomoEndpoint) doctorCheckResult {
	timeout := config.DefaultConfig.Timeout
	if cfg != nil && cfg.Timeout > 0 {
		timeout = cfg.Timeout
	}
	client := api.NewClient(endpoint, timeout)
	if _, err := client.GetConfigs(); err != nil {
		return doctorCheckResult{Name: "mihomo_api", Status: doctorStatusFail, Message: err.Error(), Target: endpoint.ExternalController, Hint: i18n.T("cli.doctor.hint_api_unreachable")}
	}
	return doctorCheckResult{Name: "mihomo_api", Status: doctorStatusOK, Message: "reachable", Target: endpoint.ExternalController}
}

func checkWebSocketAvailable(endpoint config.MihomoEndpoint) doctorCheckResult {
	wsURL, err := buildDoctorWSURL(endpoint.ExternalController, "traffic")
	if err != nil {
		return doctorCheckResult{Name: "websocket", Status: doctorStatusFail, Message: err.Error(), Target: endpoint.ExternalController, Hint: i18n.T("cli.doctor.hint_ws_url")}
	}

	dialer := websocket.Dialer{HandshakeTimeout: doctorProbeTimeout}
	headers := make(http.Header)
	if endpoint.Secret != "" {
		headers.Set("Authorization", "Bearer "+endpoint.Secret)
	}
	conn, resp, err := dialer.Dial(wsURL, headers)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return doctorCheckResult{Name: "websocket", Status: doctorStatusFail, Message: err.Error(), Target: endpoint.ExternalController, Hint: i18n.T("cli.doctor.hint_ws_handshake")}
	}
	_ = conn.Close()
	return doctorCheckResult{Name: "websocket", Status: doctorStatusOK, Message: "reachable", Target: endpoint.ExternalController}
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
		return renderDoctorPlain(w, report)
	default:
		return fmt.Errorf(i18n.T("cli.doctor.unsupported_format"), format)
	}
}

func renderDoctorPlain(w io.Writer, report doctorReport) error {
	if _, err := fmt.Fprintf(w, "%s: %s\n", i18n.T("cli.doctor.label_health"), report.Status); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, i18n.Tf("cli.doctor.label_fail_warn", report.Failed, report.Warnings)); err != nil {
		return err
	}
	for _, check := range report.Checks {
		targetStr := ""
		if check.Target != "" && check.Target != "-" {
			targetStr = fmt.Sprintf(" (%s)", check.Target)
		}
		if _, err := fmt.Fprintf(w, "[%s] %s%s: %s\n", strings.ToUpper(string(check.Status)), check.Name, targetStr, check.Message); err != nil {
			return err
		}
		if check.Hint != "" {
			if _, err := fmt.Fprintf(w, "      %s: %s\n", i18n.T("cli.doctor.label_hint"), check.Hint); err != nil {
				return err
			}
		}
	}
	return nil
}

func renderDoctorTable(w io.Writer, report doctorReport) error {
	tw := newTabWriter(w)
	if _, err := fmt.Fprintln(tw, "CHECK\tSTATUS\tTARGET\tMESSAGE\tHINT"); err != nil {
		return err
	}
	for _, check := range report.Checks {
		target := check.Target
		if target == "" {
			target = "-"
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", check.Name, strings.ToUpper(string(check.Status)), target, check.Message, check.Hint); err != nil {
			return err
		}
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
		return errors.New(i18n.T("cli.doctor.err_controller_scheme"))
	}
	if parsed.Host == "" {
		return errors.New(i18n.T("cli.doctor.err_controller_host"))
	}
	return nil
}

func normalizeDoctorProxyURL(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New(i18n.T("cli.doctor.err_proxy_missing"))
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
		return nil, errors.New(i18n.T("cli.doctor.err_proxy_scheme"))
	}
	if parsed.Host == "" {
		return nil, errors.New(i18n.T("cli.doctor.err_proxy_host"))
	}
	if parsed.Port() == "" {
		return nil, errors.New(i18n.T("cli.doctor.err_proxy_port"))
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

func buildDoctorWSURL(baseURL, endpoint string) (string, error) {
	// external-controller 原值无 scheme，先补 http:// 再校验/转换。
	normalized := api.ControllerHTTPURL(baseURL)
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
	return parsed.String(), nil
}
