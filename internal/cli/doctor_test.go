package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoctorSummaryFailsWhenAnyCheckFails(t *testing.T) {
	results := []doctorCheckResult{
		{Name: "api_address", Status: doctorStatusOK, Message: "valid"},
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
		{Name: "api_address", Status: doctorStatusOK, Message: "valid"},
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
			{Name: "api_address", Status: doctorStatusOK, Message: "valid", Target: "http://127.0.0.1:9090"},
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
			{Name: "api_address", Status: doctorStatusOK, Message: "valid", Target: "http://127.0.0.1:9090"},
			{Name: "secret", Status: doctorStatusWarn, Message: "not configured"},
		},
	}

	for _, tt := range []struct {
		name     string
		format   outputFormat
		contains []string
	}{
		{
			name:   "plain",
			format: outputFormatPlain,
			contains: []string{
				"配置健康检查: warn",
				"[ok] api_address",
				"[warn] secret",
				"not configured",
			},
		},
		{
			name:   "table",
			format: outputFormatTable,
			contains: []string{
				"CHECK",
				"STATUS",
				"api_address",
				"warn",
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
