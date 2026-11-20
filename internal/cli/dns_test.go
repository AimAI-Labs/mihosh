package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
)

func TestRenderDNSQueryResultFormats(t *testing.T) {
	i18n.Init()
	resp := &model.DNSQueryResponse{
		Status: 0,
		Answer: []model.DNSAnswer{
			{Name: "example.com.", Type: 1, TTL: 60, Data: "198.18.0.7"},
			{Name: "example.com.", Type: 1, TTL: 120, Data: "93.184.216.34"},
		},
	}

	// JSON
	var jsonBuf bytes.Buffer
	if err := renderDNSQueryResult(&jsonBuf, "example.com", "A", resp, outputFormatJSON); err != nil {
		t.Fatalf("json render failed: %v", err)
	}
	var parsed dnsQueryOutput
	if err := json.Unmarshal(jsonBuf.Bytes(), &parsed); err != nil {
		t.Fatalf("json unmarshal failed: %v", err)
	}
	if !parsed.HasFakeIP || len(parsed.Answers) != 2 || !parsed.Answers[0].FakeIP || parsed.Answers[1].FakeIP {
		t.Fatalf("unexpected json output: %+v", parsed)
	}

	// Table
	var tableBuf bytes.Buffer
	if err := renderDNSQueryResult(&tableBuf, "example.com", "A", resp, outputFormatTable); err != nil {
		t.Fatalf("table render failed: %v", err)
	}
	if !strings.Contains(tableBuf.String(), "198.18.0.7") || !strings.Contains(tableBuf.String(), "fake-ip") {
		t.Fatalf("unexpected table output:\n%s", tableBuf.String())
	}

	// Plain
	var plainBuf bytes.Buffer
	if err := renderDNSQueryResult(&plainBuf, "example.com", "A", resp, outputFormatPlain); err != nil {
		t.Fatalf("plain render failed: %v", err)
	}
	if !strings.Contains(plainBuf.String(), "[Fake-IP]") || !strings.Contains(plainBuf.String(), "93.184.216.34") {
		t.Fatalf("unexpected plain output:\n%s", plainBuf.String())
	}
}
