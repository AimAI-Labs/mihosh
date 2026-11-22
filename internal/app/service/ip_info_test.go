package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchCurrentIPInfo_ProviderParsersAndNormalization(t *testing.T) {
	tests := []struct {
		name        string
		provider    func(endpoint string) ipInfoProvider
		payload     string
		wantIP      string
		wantCountry string
		wantCode    string
		wantRegion  string
		wantCity    string
		wantASN     int
		wantAS      string
		wantOrg     string
		wantISP     string
		wantTZ      string
		wantLat     float64
		wantLon     float64
	}{
		{
			name: "ip.sb",
			provider: func(endpoint string) ipInfoProvider {
				return ipInfoProvider{name: "ip.sb", endpoint: endpoint, parse: parseIPSBInfo}
			},
			payload:     `{"ip":"203.0.113.10","country":"Japan","country_code":"jp","region":"Tokyo","city":"Tokyo","asn":2514,"asn_organization":"NTT PC Communications, Inc.","isp":"Unknown","latitude":35.68,"longitude":139.68,"timezone":"Asia/Tokyo"}`,
			wantIP:      "203.0.113.10",
			wantCountry: "Japan",
			wantCode:    "JP",
			wantRegion:  "Tokyo",
			wantCity:    "Tokyo",
			wantASN:     2514,
			wantAS:      "AS2514",
			wantOrg:     "NTT PC Communications, Inc.",
			wantISP:     "Unknown",
			wantTZ:      "Asia/Tokyo",
			wantLat:     35.68,
			wantLon:     139.68,
		},
		{
			name: "ipwho.is",
			provider: func(endpoint string) ipInfoProvider {
				return ipInfoProvider{name: "ipwho.is", endpoint: endpoint, parse: parseIPWhoisInfo}
			},
			payload:     `{"success":true,"ip":"198.51.100.2","country":"United States","country_code":"US","region":"California","city":"San Jose","latitude":37.34,"longitude":-121.89,"connection":{"asn":13335,"org":"Cloudflare, Inc.","isp":"Cloudflare"},"timezone":{"id":"America/Los_Angeles"}}`,
			wantIP:      "198.51.100.2",
			wantCountry: "United States",
			wantCode:    "US",
			wantRegion:  "California",
			wantCity:    "San Jose",
			wantASN:     13335,
			wantAS:      "AS13335",
			wantOrg:     "Cloudflare, Inc.",
			wantISP:     "Cloudflare",
			wantTZ:      "America/Los_Angeles",
			wantLat:     37.34,
			wantLon:     -121.89,
		},
		{
			name: "ip-api.com",
			provider: func(endpoint string) ipInfoProvider {
				return ipInfoProvider{name: "ip-api.com", endpoint: endpoint, parse: parseIPAPIInfo}
			},
			payload:     `{"status":"success","query":"192.0.2.5","country":"Singapore","countryCode":"SG","regionName":"Central Singapore","city":"Singapore","as":"AS45102 Alibaba (US) Technology Co., Ltd.","org":"Alibaba","isp":"Alibaba Cloud","lat":1.29,"lon":103.85,"timezone":"Asia/Singapore"}`,
			wantIP:      "192.0.2.5",
			wantCountry: "Singapore",
			wantCode:    "SG",
			wantRegion:  "Central Singapore",
			wantCity:    "Singapore",
			wantASN:     45102,
			wantAS:      "AS45102",
			wantOrg:     "Alibaba",
			wantISP:     "Alibaba Cloud",
			wantTZ:      "Asia/Singapore",
			wantLat:     1.29,
			wantLon:     103.85,
		},
		{
			name: "ipapi.co",
			provider: func(endpoint string) ipInfoProvider {
				return ipInfoProvider{name: "ipapi.co", endpoint: endpoint, parse: parseIPAPICoInfo}
			},
			payload:     `{"ip":"2001:db8::1","country_name":"Germany","country_code":"DE","region":"Hesse","city":"Frankfurt","asn":"AS24940","org":"Hetzner Online GmbH","latitude":50.11,"longitude":8.68,"timezone":"Europe/Berlin"}`,
			wantIP:      "2001:db8::1",
			wantCountry: "Germany",
			wantCode:    "DE",
			wantRegion:  "Hesse",
			wantCity:    "Frankfurt",
			wantASN:     24940,
			wantAS:      "AS24940",
			wantOrg:     "Hetzner Online GmbH",
			wantTZ:      "Europe/Berlin",
			wantLat:     50.11,
			wantLon:     8.68,
		},
		{
			name: "geojs.io",
			provider: func(endpoint string) ipInfoProvider {
				return ipInfoProvider{name: "geojs.io", endpoint: endpoint, parse: parseGeoJSInfo}
			},
			payload:     `{"ip":"203.0.113.88","country":"Japan","country_code":"JP","region":"Osaka","city":"Osaka","asn":2514,"organization_name":"NTT PC Communications, Inc.","latitude":"34.69","longitude":"135.50","timezone":"Asia/Tokyo"}`,
			wantIP:      "203.0.113.88",
			wantCountry: "Japan",
			wantCode:    "JP",
			wantRegion:  "Osaka",
			wantCity:    "Osaka",
			wantASN:     2514,
			wantAS:      "AS2514",
			wantOrg:     "NTT PC Communications, Inc.",
			wantTZ:      "Asia/Tokyo",
			wantLat:     34.69,
			wantLon:     135.50,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.NotEmpty(t, r.Header.Get("User-Agent"))
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.payload))
			}))
			defer srv.Close()

			info, err := fetchCurrentIPInfoWithProviders(context.Background(), srv.Client(), []ipInfoProvider{tc.provider(srv.URL)})
			require.NoError(t, err)
			require.Equal(t, tc.wantIP, info.IP)
			require.Equal(t, tc.wantCountry, info.Country)
			require.Equal(t, tc.wantCode, info.CountryCode)
			require.Equal(t, tc.wantRegion, info.RegionName)
			require.Equal(t, tc.wantCity, info.City)
			require.Equal(t, tc.wantASN, info.ASN)
			require.Equal(t, tc.wantAS, info.AS)
			require.Equal(t, tc.wantOrg, info.Organization)
			require.Equal(t, tc.wantISP, info.ISP)
			require.Equal(t, tc.wantTZ, info.Timezone)
			require.InDelta(t, tc.wantLat, info.Latitude, 0.001)
			require.InDelta(t, tc.wantLon, info.Longitude, 0.001)
		})
	}
}

func TestFetchCurrentIPInfo_FallbackAcrossProviders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rate-limited":
			w.WriteHeader(http.StatusTooManyRequests)
		case "/unsuccessful":
			_, _ = w.Write([]byte(`{"success":false,"message":"quota exceeded"}`))
		case "/bad-ip":
			_, _ = w.Write([]byte(`{"ip":"not-an-ip"}`))
		case "/ok":
			_, _ = w.Write([]byte(`{"ip":"203.0.113.42","country":"Japan","country_code":"JP","city":"Tokyo","asn":2514,"organization":"NTT PC"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	providers := []ipInfoProvider{
		{name: "p1", endpoint: srv.URL + "/rate-limited", parse: parseIPSBInfo},
		{name: "p2", endpoint: srv.URL + "/unsuccessful", parse: parseIPWhoisInfo},
		{name: "p3", endpoint: srv.URL + "/bad-ip", parse: parseIPSBInfo},
		{name: "p4", endpoint: srv.URL + "/ok", parse: parseIPSBInfo},
	}

	info, err := fetchCurrentIPInfoWithProviders(context.Background(), srv.Client(), providers)
	require.NoError(t, err)
	require.Equal(t, "203.0.113.42", info.IP)
	require.Equal(t, "JP", info.CountryCode)
	require.Equal(t, "AS2514", info.AS)
}

func TestFetchCurrentIPInfo_UsesProxyAddress(t *testing.T) {
	var proxyHits atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ip":"198.51.100.99","country":"Japan","country_code":"JP","city":"Tokyo"}`))
	}))
	defer proxy.Close()

	client, err := newIPInfoHTTPClient(proxy.URL)
	require.NoError(t, err)

	providers := []ipInfoProvider{
		{name: "proxy-test", endpoint: "http://ip-info.example.test/geoip", parse: parseIPSBInfo},
	}
	info, err := fetchCurrentIPInfoWithProviders(context.Background(), client, providers)
	require.NoError(t, err)
	require.Equal(t, int32(1), proxyHits.Load())
	require.Equal(t, "198.51.100.99", info.IP)
}

func TestFetchCurrentIPInfo_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := fetchCurrentIPInfoWithProviders(ctx, http.DefaultClient, []ipInfoProvider{
		{name: "p1", endpoint: "http://127.0.0.1:1", parse: parseIPSBInfo},
	})
	require.ErrorIs(t, err, context.Canceled)
}
