package components

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
)

func renderTargetIPGeoSection(ipInfo *model.IPInfo, s detailStyles) []string {
	lines := []string{
		s.SectionTitle.Render(fmt.Sprintf("─── %s ───", i18n.T("conns.detail.title_geo"))),
		"",
	}

	if ipInfo == nil {
		lines = append(lines, s.Dim.Render(i18n.T("conns.detail.loading_geo")))
		return lines
	}

	ip := firstNonEmpty(ipInfo.IP, ipInfo.Query, "-")
	location := strings.Join(nonEmptyStrings(ipInfo.Country, ipInfo.RegionName, ipInfo.City), ", ")
	if location == "" {
		location = i18n.T("conns.detail.unknown")
	}

	asn := firstNonEmpty(
		formatASNInt(ipInfo.ASN),
		ipInfo.AS,
	)
	if asn == "" {
		asn = "-"
	}

	network := firstNonEmpty(ipInfo.ISP, ipInfo.Org, ipInfo.Organization, ipInfo.ASNOrganization, "-")

	lines = append(lines, renderKVLine(i18n.T("conns.detail.label.ip"), ip, s))
	lines = append(lines, renderKVLine(i18n.T("conns.detail.label.location"), location, s))
	lines = append(lines, renderKVLine(i18n.T("conns.detail.label.asn"), asn, s))
	lines = append(lines, renderKVLine(i18n.T("conns.detail.label.network"), network, s))

	if timezone := firstNonEmpty(ipInfo.Timezone); timezone != "" {
		lines = append(lines, renderKVLine(i18n.T("conns.detail.label.timezone"), timezone, s))
	}

	lat, lon, hasCoord := coordinates(ipInfo)
	if hasCoord {
		lines = append(lines, renderKVLine(i18n.T("conns.detail.label.coordinates"), fmt.Sprintf("%.3f, %.3f", lat, lon), s))
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
