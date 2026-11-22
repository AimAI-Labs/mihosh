package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
)

const (
	ipInfoProviderTimeout = 4 * time.Second
	maxIPInfoBodyBytes    = 64 << 10
	ipInfoUserAgent       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
)

type ipInfoProvider struct {
	name     string
	endpoint string
	parse    func([]byte) (*model.IPInfo, error)
}

var defaultIPInfoProviders = []ipInfoProvider{
	{name: "ip.sb", endpoint: "https://api.ip.sb/geoip", parse: parseIPSBInfo},
	{name: "ipwho.is", endpoint: "https://ipwho.is/", parse: parseIPWhoisInfo},
	{name: "ip-api.com", endpoint: model.IPApiURL, parse: parseIPAPIInfo},
	{name: "ipapi.co", endpoint: "https://ipapi.co/json/", parse: parseIPAPICoInfo},
	{name: "geojs.io", endpoint: "https://get.geojs.io/v1/ip/geo.json", parse: parseGeoJSInfo},
}

// FetchCurrentIPInfo fetches the current public exit IP geolocation through proxyAddr
// (or directly when proxyAddr is empty), falling back across multiple providers like Clash Verge Rev.
func FetchCurrentIPInfo(ctx context.Context, proxyAddr string) (*model.IPInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	client, err := newIPInfoHTTPClient(proxyAddr)
	if err != nil {
		return nil, err
	}
	defer client.CloseIdleConnections()
	return fetchCurrentIPInfoWithProviders(ctx, client, defaultIPInfoProviders)
}

func newIPInfoHTTPClient(proxyAddr string) (*http.Client, error) {
	addr := strings.TrimSpace(proxyAddr)
	if addr == "" {
		return &http.Client{Timeout: ipInfoProviderTimeout}, nil
	}
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") && !strings.HasPrefix(addr, "socks5://") {
		addr = "http://" + addr
	}
	proxyURL, err := url.Parse(addr)
	if err != nil || proxyURL.Hostname() == "" {
		return nil, fmt.Errorf("invalid proxy address: %s", proxyAddr)
	}
	return &http.Client{
		Timeout: ipInfoProviderTimeout,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}, nil
}

func fetchCurrentIPInfoWithProviders(ctx context.Context, client *http.Client, providers []ipInfoProvider) (*model.IPInfo, error) {
	if len(providers) == 0 {
		return nil, errors.New("no IP info providers configured")
	}
	var lastErr error
	for _, provider := range providers {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		reqCtx, cancel := context.WithTimeout(ctx, ipInfoProviderTimeout)
		info, err := queryIPInfoProvider(reqCtx, client, provider)
		cancel()
		if err == nil && info != nil {
			return info, nil
		}
		lastErr = fmt.Errorf("%s: %w", provider.name, err)
	}
	if lastErr == nil {
		lastErr = errors.New("failed to fetch current IP info")
	}
	return nil, lastErr
}

func queryIPInfoProvider(ctx context.Context, client *http.Client, provider ipInfoProvider) (*model.IPInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, provider.endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", ipInfoUserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxIPInfoBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxIPInfoBodyBytes {
		return nil, errors.New("response exceeds size limit")
	}
	info, err := provider.parse(body)
	if err != nil {
		return nil, err
	}
	return normalizeIPInfo(info)
}

func parseIPSBInfo(data []byte) (*model.IPInfo, error) {
	var raw model.IPInfo
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return &raw, nil
}

func parseIPWhoisInfo(data []byte) (*model.IPInfo, error) {
	var raw struct {
		Success     bool    `json:"success"`
		Message     string  `json:"message"`
		IP          string  `json:"ip"`
		Country     string  `json:"country"`
		CountryCode string  `json:"country_code"`
		Region      string  `json:"region"`
		City        string  `json:"city"`
		Latitude    float64 `json:"latitude"`
		Longitude   float64 `json:"longitude"`
		Connection  struct {
			ASN int    `json:"asn"`
			Org string `json:"org"`
			ISP string `json:"isp"`
		} `json:"connection"`
		Timezone struct {
			ID string `json:"id"`
		} `json:"timezone"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if !raw.Success {
		if raw.Message != "" {
			return nil, errors.New(cleanIPInfoField(raw.Message))
		}
		return nil, errors.New("lookup unsuccessful")
	}
	return &model.IPInfo{
		IP:           raw.IP,
		Country:      raw.Country,
		CountryCode:  raw.CountryCode,
		Region:       raw.Region,
		RegionName:   raw.Region,
		City:         raw.City,
		ASN:          raw.Connection.ASN,
		Organization: raw.Connection.Org,
		Org:          raw.Connection.Org,
		ISP:          raw.Connection.ISP,
		Latitude:     raw.Latitude,
		Longitude:    raw.Longitude,
		Timezone:     raw.Timezone.ID,
	}, nil
}

func parseIPAPIInfo(data []byte) (*model.IPInfo, error) {
	var raw struct {
		model.IPInfo
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if raw.Status != "" && !strings.EqualFold(raw.Status, "success") {
		if raw.Message != "" {
			return nil, errors.New(cleanIPInfoField(raw.Message))
		}
		return nil, fmt.Errorf("status %s", raw.Status)
	}
	return &raw.IPInfo, nil
}

func parseIPAPICoInfo(data []byte) (*model.IPInfo, error) {
	var raw struct {
		Error       bool    `json:"error"`
		Reason      string  `json:"reason"`
		IP          string  `json:"ip"`
		CountryName string  `json:"country_name"`
		CountryCode string  `json:"country_code"`
		Region      string  `json:"region"`
		City        string  `json:"city"`
		ASN         string  `json:"asn"`
		Org         string  `json:"org"`
		Latitude    float64 `json:"latitude"`
		Longitude   float64 `json:"longitude"`
		Timezone    string  `json:"timezone"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if raw.Error {
		if raw.Reason != "" {
			return nil, errors.New(cleanIPInfoField(raw.Reason))
		}
		return nil, errors.New("lookup error")
	}
	return &model.IPInfo{
		IP:           raw.IP,
		Country:      raw.CountryName,
		CountryCode:  raw.CountryCode,
		Region:       raw.Region,
		RegionName:   raw.Region,
		City:         raw.City,
		AS:           raw.ASN,
		Organization: raw.Org,
		Org:          raw.Org,
		Latitude:     raw.Latitude,
		Longitude:    raw.Longitude,
		Timezone:     raw.Timezone,
	}, nil
}

func parseGeoJSInfo(data []byte) (*model.IPInfo, error) {
	var raw struct {
		IP               string `json:"ip"`
		Country          string `json:"country"`
		CountryCode      string `json:"country_code"`
		Region           string `json:"region"`
		City             string `json:"city"`
		ASN              int    `json:"asn"`
		OrganizationName string `json:"organization_name"`
		Organization     string `json:"organization"`
		Latitude         string `json:"latitude"`
		Longitude        string `json:"longitude"`
		Timezone         string `json:"timezone"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	lat, _ := strconv.ParseFloat(strings.TrimSpace(raw.Latitude), 64)
	lon, _ := strconv.ParseFloat(strings.TrimSpace(raw.Longitude), 64)
	org := raw.OrganizationName
	if org == "" {
		org = raw.Organization
	}
	return &model.IPInfo{
		IP:           raw.IP,
		Country:      raw.Country,
		CountryCode:  raw.CountryCode,
		Region:       raw.Region,
		RegionName:   raw.Region,
		City:         raw.City,
		ASN:          raw.ASN,
		Organization: org,
		Org:          org,
		Latitude:     lat,
		Longitude:    lon,
		Timezone:     raw.Timezone,
	}, nil
}

func normalizeIPInfo(raw *model.IPInfo) (*model.IPInfo, error) {
	if raw == nil {
		return nil, errors.New("empty IP info")
	}
	info := *raw
	ipStr := strings.TrimSpace(info.IP)
	if ipStr == "" {
		ipStr = strings.TrimSpace(info.Query)
	}
	parsedIP := net.ParseIP(ipStr)
	if parsedIP == nil {
		return nil, errors.New("response contains no valid IP address")
	}
	info.IP = parsedIP.String()
	info.Query = info.IP

	info.Country = cleanIPInfoField(info.Country)
	code := cleanIPInfoField(info.CountryCode)
	if code == "" {
		code = cleanIPInfoField(info.CountryCodeApi)
	}
	info.CountryCode = strings.ToUpper(code)
	info.CountryCodeApi = info.CountryCode

	region := cleanIPInfoField(info.RegionName)
	if region == "" {
		region = cleanIPInfoField(info.Region)
	}
	info.RegionName = region
	info.Region = region
	info.City = cleanIPInfoField(info.City)

	info.AS = cleanIPInfoField(info.AS)
	if info.ASN <= 0 && info.AS != "" {
		info.ASN = parseASNNumber(info.AS)
	}
	if info.ASN > 0 {
		info.AS = fmt.Sprintf("AS%d", info.ASN)
	}

	info.ISP = cleanIPInfoField(info.ISP)
	info.ASNOrganization = cleanIPInfoField(info.ASNOrganization)
	info.Organization = cleanIPInfoField(info.Organization)
	info.Org = cleanIPInfoField(info.Org)
	if info.Organization == "" {
		info.Organization = info.Org
	}
	if info.Organization == "" {
		info.Organization = info.ASNOrganization
	}
	if info.Org == "" {
		info.Org = info.Organization
	}

	if info.Latitude == 0 && info.Lat != 0 {
		info.Latitude = info.Lat
	}
	if info.Longitude == 0 && info.Lon != 0 {
		info.Longitude = info.Lon
	}
	info.Lat = info.Latitude
	info.Lon = info.Longitude
	info.Timezone = cleanIPInfoField(info.Timezone)

	return &info, nil
}

func parseASNNumber(asText string) int {
	token := strings.TrimSpace(asText)
	if fields := strings.Fields(token); len(fields) > 0 {
		token = fields[0]
	}
	token = strings.TrimPrefix(strings.ToUpper(token), "AS")
	n, err := strconv.Atoi(token)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func cleanIPInfoField(value string) string {
	if strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return ""
	}
	return strings.TrimSpace(value)
}
