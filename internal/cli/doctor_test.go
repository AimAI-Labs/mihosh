package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoctorSummaryFailsWhenAnyCheckFails(t *testing.T) {
	results := []doctorCheckResult{
		{Name: "external_controller", Status: doctorStatusOK, Message: "valid"},
		{Name: "mihomo", Status: doctorStatusFail, Message: "connection refused"},
		{Name: "secret", Status: doctorStatusWarn, Message: "not configured"},
	}

	summary := buildDoctorSummary(results)

	assert.Equal(t, doctorStatusFail, summary.Status)
	assert.False(t, summary.Healthy)
	assert.Equal(t, 1, summary.Failed)
	assert.Equal(t, 1, summary.Warnings)
}

func TestDoctorSummaryWarnsWhenOnlyWarningsExist(t *testing.T) {
	results := []doctorCheckResult{
		{Name: "external_controller", Status: doctorStatusOK, Message: "valid"},
		{Name: "secret", Status: doctorStatusWarn, Message: "not configured"},
	}

	summary := buildDoctorSummary(results)

	assert.Equal(t, doctorStatusWarn, summary.Status)
	assert.True(t, summary.Healthy)
	assert.Equal(t, 0, summary.Failed)
	assert.Equal(t, 1, summary.Warnings)
}

func TestRenderDoctorJSON(t *testing.T) {
	report := doctorReport{
		Status:   doctorStatusFail,
		Healthy:  false,
		Failed:   1,
		Warnings: 1,
		Checks: []doctorCheckResult{
			{Name: "external_controller", Status: doctorStatusOK, Message: "valid", Target: "http://127.0.0.1:9090"},
			{Name: "secret", Status: doctorStatusWarn, Message: "not configured"},
			{Name: "mihomo", Status: doctorStatusFail, Message: "API request failed"},
		},
	}

	var out bytes.Buffer
	err := renderDoctorReport(&out, report, outputFormatJSON)
	require.NoError(t, err)

	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	assert.Equal(t, "fail", got["status"])
	assert.Equal(t, false, got["healthy"])
	assert.Equal(t, float64(1), got["failed"])
	assert.Equal(t, float64(1), got["warnings"])
	assert.Contains(t, out.String(), `"name": "mihomo"`)
	assert.Contains(t, out.String(), `"status": "fail"`)
}

func TestRenderDoctorPlainAndTable(t *testing.T) {
	report := doctorReport{
		Status:   doctorStatusWarn,
		Healthy:  true,
		Failed:   0,
		Warnings: 1,
		Checks: []doctorCheckResult{
			{Name: "external_controller", Status: doctorStatusOK, Message: "valid", Target: "http://127.0.0.1:9090"},
			{Name: "secret", Status: doctorStatusWarn, Message: "not configured"},
		},
	}

	i18n.Init()

	for _, tt := range []struct {
		name     string
		format   outputFormat
		contains []string
	}{
		{
			name:   "plain",
			format: outputFormatPlain,
			contains: []string{
				i18n.T("cli.doctor.label_health") + ": warn",
				"[OK] external_controller",
				"[WARN] secret",
				"not configured",
			},
		},
		{
			name:   "table",
			format: outputFormatTable,
			contains: []string{
				"CHECK",
				"STATUS",
				"external_controller",
				"WARN",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := renderDoctorReport(&out, report, tt.format)
			require.NoError(t, err)
			output := out.String()
			for _, want := range tt.contains {
				assert.True(t, strings.Contains(output, want), "expected %q in %q", want, output)
			}
		})
	}
}

func TestValidateDoctorHTTPURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "valid http", raw: "http://127.0.0.1:9090"},
		{name: "valid https", raw: "https://example.com"},
		{name: "missing scheme", raw: "127.0.0.1:9090", wantErr: true},
		{name: "unsupported scheme", raw: "socks5://127.0.0.1:7890", wantErr: true},
		{name: "missing host", raw: "http://", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDoctorHTTPURL(tt.raw)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDoctorErrorForReportUsesNetworkKindForFailedChecks(t *testing.T) {
	report := doctorReport{
		Status:  doctorStatusFail,
		Failed:  1,
		Healthy: false,
		Checks: []doctorCheckResult{
			{Name: "mihomo", Status: doctorStatusFail, Message: "connection refused"},
		},
	}

	err := doctorErrorForReport(report)

	require.Error(t, err)
	var cmdErr *commandError
	require.True(t, errors.As(err, &cmdErr))
	assert.Equal(t, commandErrorNetwork, cmdErr.kind)
}

type doctorFailingWriter struct {
	err error
}

func (w doctorFailingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestRenderDoctorReportReturnsWriterError(t *testing.T) {
	want := errors.New("output unavailable")
	for _, format := range []outputFormat{outputFormatPlain, outputFormatTable, outputFormatJSON} {
		t.Run(string(format), func(t *testing.T) {
			err := renderDoctorReport(doctorFailingWriter{err: want}, doctorReport{}, format)
			require.ErrorIs(t, err, want)
		})
	}
}

func TestDoctorWebSocketNormalizesWildcardAndUsesAuthorization(t *testing.T) {
	const secret = "fixture"
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil || host != "127.0.0.1" {
			t.Errorf("websocket controller host = %q, want 127.0.0.1", r.Host)
		}
		if r.URL.Path != "/traffic" || r.URL.RawQuery != "" {
			t.Error("doctor must probe the traffic stream without query parameters")
		}
		if r.Header.Get("Authorization") != "Bearer "+secret {
			t.Error("missing bearer authorization")
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		_ = conn.Close()
	}))
	defer server.Close()

	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	require.NoError(t, err)
	for _, host := range []string{"0.0.0.0", "::", ""} {
		addr := net.JoinHostPort(host, port)
		for _, raw := range []string{addr, "http://" + addr} {
			result := checkWebSocketAvailable(config.MihomoEndpoint{ExternalController: raw, Secret: secret})
			require.Equal(t, doctorStatusOK, result.Status, result.Message)
		}
	}
}

func TestEnglishCLIOutputHasNoUntranslatedChinese(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")

	report := buildDoctorSummary([]doctorCheckResult{
		checkSecret(""),
		checkExternalController("ftp://127.0.0.1:9090"),
		checkProxyAddress(""),
	})
	_, formatErr := parseOutputFormat("bad-format")
	require.Error(t, formatErr)
	var doctor bytes.Buffer
	require.NoError(t, renderDoctorReport(&doctor, report, outputFormatPlain))
	localizeCommands()
	t.Cleanup(func() {
		i18n.SetLanguageOverride("auto")
		localizeCommands()
	})
	for _, item := range []struct{ name, text string }{
		{"doctor", doctor.String()},
		{"config selector", initialSelectorModel().View()},
		{"unsupported output format", formatErr.Error()},
		{"test usage", testCmd.Use},
	} {
		assert.NotContains(t, item.text, "cli.", item.name)
		assert.False(t, strings.ContainsFunc(item.text, func(r rune) bool { return unicode.Is(unicode.Han, r) }), "%s contains untranslated text: %q", item.name, item.text)
	}
}
