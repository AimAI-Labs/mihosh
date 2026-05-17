package components

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
)

func renderTargetIPGeoSection(ipInfo *model.IPInfo, s detailStyles) []string {
	lines := []string{
		s.SectionTitle.Render("─── 目标 IP 地理信息 ───"),
		"",
	}

	if ipInfo == nil {
		lines = append(lines, s.Dim.Render("正在加载 IP 信息..."))
		return lines
	}

	ip := firstNonEmpty(ipInfo.IP, ipInfo.Query, "-")
	location := strings.Join(nonEmptyStrings(ipInfo.Country, ipInfo.RegionName, ipInfo.City), ", ")
	if location == "" {
		location = "未知"
	}

	asn := firstNonEmpty(
		formatASNInt(ipInfo.ASN),
		ipInfo.AS,
	)
	if asn == "" {
		asn = "-"
	}

	network := firstNonEmpty(ipInfo.ISP, ipInfo.Org, ipInfo.Organization, ipInfo.ASNOrganization, "-")

	lines = append(lines, renderKVLine("IP", ip, s))
	lines = append(lines, renderKVLine("位置", location, s))
	lines = append(lines, renderKVLine("ASN", asn, s))
	lines = append(lines, renderKVLine("网络", network, s))

	if timezone := firstNonEmpty(ipInfo.Timezone); timezone != "" {
		lines = append(lines, renderKVLine("时区", timezone, s))
	}

	lat, lon, hasCoord := coordinates(ipInfo)
	if hasCoord {
		lines = append(lines, renderKVLine("坐标", fmt.Sprintf("%.3f, %.3f", lat, lon), s))
	}

	return lines
}

func formatASNInt(asn int) string {
	if asn <= 0 {
		return ""
	}
	return fmt.Sprintf("AS%d", asn)
}

func nonEmptyStrings(values ...string) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			items = append(items, value)
		}
	}
	return items
}

func coordinates(ipInfo *model.IPInfo) (float64, float64, bool) {
	if ipInfo.Latitude != 0 || ipInfo.Longitude != 0 {
		return ipInfo.Latitude, ipInfo.Longitude, true
	}
	if ipInfo.Lat != 0 || ipInfo.Lon != 0 {
		return ipInfo.Lat, ipInfo.Lon, true
	}
	return 0, 0, false
}
