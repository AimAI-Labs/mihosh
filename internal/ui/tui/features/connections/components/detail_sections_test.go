package components

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/charmbracelet/lipgloss"
)

func TestRenderConnectionInfoSection(t *testing.T) {
	s := plainDetailStyles()
	conn := &model.Connection{
		ID:   "c1",
		Rule: "MATCH",
		Metadata: model.Metadata{
			Host:            "example.com",
			SourceIP:        "10.0.0.1",
			SourcePort:      "52345",
			DestinationIP:   "8.8.8.8",
			DestinationPort: "443",
			Network:         "tcp",
			Type:            "https",
			Process:         "mihosh.exe",
		},
		Chains:        []string{"Proxy-A", "Proxy-B"},
		Start:         "invalid-time",
		Download:      2048,
		Upload:        1024,
		DownloadSpeed: 1536,
		UploadSpeed:   512,
	}

	content := strings.Join(renderConnectionInfoSection(conn, s), "\n")
	assertContains(t, content, "─── 连接详情 ───")
	assertContains(t, content, "主机： example.com")
	assertContains(t, content, "源地址： 10.0.0.1:52345")
	assertContains(t, content, "目标地址： 8.8.8.8:443")
	assertContains(t, content, "规则链路： MATCH → Proxy-A → Proxy-B")
	assertContains(t, content, "连接时长： -")
	assertContains(t, content, "实时速率： ↓1.5 KB/s  ↑512 B/s")
}

func TestRenderJSONDetailSection(t *testing.T) {
	s := plainDetailStyles()
	conn := &model.Connection{
		ID:   "conn-1",
		Rule: "MATCH",
		Metadata: model.Metadata{
			DestinationIP: "1.1.1.1",
		},
	}

	lines, err := renderJSONDetailSection(conn, s)
	if err != nil {
		t.Fatalf("renderJSONDetailSection returned error: %v", err)
	}

	content := strings.Join(lines, "\n")
	assertContains(t, content, "─── JSON 详情 ───")
	assertContains(t, content, "\"id\": \"conn-1\"")
	assertContains(t, content, "\"destinationIP\": \"1.1.1.1\"")
}

func TestRenderTargetIPGeoSection(t *testing.T) {
	s := plainDetailStyles()

	nilContent := strings.Join(renderTargetIPGeoSection(nil, s), "\n")
	assertContains(t, nilContent, "─── 目标 IP 地理信息 ───")
	assertContains(t, nilContent, "正在加载 IP 信息...")

	info := &model.IPInfo{
		IP:         "8.8.8.8",
		Country:    "United States",
		RegionName: "California",
		City:       "Mountain View",
		ASN:        15169,
		ISP:        "Google LLC",
		Timezone:   "America/Los_Angeles",
		Latitude:   37.386,
		Longitude:  -122.0838,
	}

	content := strings.Join(renderTargetIPGeoSection(info, s), "\n")
	assertContains(t, content, "IP： 8.8.8.8")
	assertContains(t, content, "位置： United States, California, Mountain View")
	assertContains(t, content, "ASN： AS15169")
	assertContains(t, content, "网络： Google LLC")
	assertContains(t, content, "时区： America/Los_Angeles")
	assertContains(t, content, "坐标： 37.386, -122.084")
}

func plainDetailStyles() detailStyles {
	style := lipgloss.NewStyle()
	return detailStyles{
		Header:       style,
		SectionTitle: style,
		Label:        style,
		Value:        style,
		JSON:         style,
		Dim:          style,
	}
}

func assertContains(t *testing.T, content, sub string) {
	t.Helper()
	if !strings.Contains(content, sub) {
		t.Fatalf("expected content to contain %q, got: %s", sub, content)
	}
}
