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
)

const maxProxyProbeBody = 16 << 10

type ProxyProbeResult struct {
	Source    string `json:"source"`
	IP        string `json:"ip,omitempty"`
	Country   string `json:"country,omitempty"`
	City      string `json:"city,omitempty"`
	ASN       uint32 `json:"asn,omitempty"`
	Org       string `json:"org,omitempty"`
	LatencyMS int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

type proxyProbeSource interface {
	lookup(context.Context, *http.Client) (ProxyProbeResult, error)
}

type jsonProxyProbe struct {
	name     string
	endpoint string
}

func (source jsonProxyProbe) lookup(ctx context.Context, client *http.Client) (ProxyProbeResult, error) {
	result := ProxyProbeResult{Source: source.name}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.endpoint, nil)
	if err != nil {
		return result, err
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return result, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return result, fmt.Errorf("HTTP status %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxProxyProbeBody+1))
	if err != nil {
		return result, err
	}
	if len(data) > maxProxyProbeBody {
		return result, errors.New("IP response exceeds size limit")
	}
	var info struct {
		IP              string `json:"ip"`
		Country         string `json:"country"`
		City            string `json:"city"`
		ASN             uint32 `json:"asn"`
		Organization    string `json:"organization"`
		ASNOrganization string `json:"asn_organization"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return result, fmt.Errorf("invalid IP response: %w", err)
	}
	ip := net.ParseIP(info.IP)
	if ip == nil {
		return result, errors.New("IP response contains no valid address")
	}
	result.IP = ip.String()
	result.Country = cleanProbeField(info.Country)
	result.City = cleanProbeField(info.City)
	result.ASN = info.ASN
	result.Org = cleanProbeField(info.Organization)
	if result.Org == "" {
		result.Org = cleanProbeField(info.ASNOrganization)
	}
	return result, nil
}

func cleanProbeField(value string) string {
	if strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return ""
	}
	return strings.TrimSpace(value)
}

// TestProxySources probes each exit-IP source through proxyURL. Source failures
// are reported individually so one unavailable endpoint cannot hide other results.
func TestProxySources(ctx context.Context, proxyURL string, timeout time.Duration) ([]ProxyProbeResult, error) {
	client, err := newProxyProbeClient(proxyURL)
	if err != nil {
		return nil, err
	}
	defer client.CloseIdleConnections()
	sources := []proxyProbeSource{
		jsonProxyProbe{name: "ip.sb", endpoint: "https://api.ip.sb/geoip"},
		jsonProxyProbe{name: "ipify", endpoint: "https://api64.ipify.org?format=json"},
	}
	return probeProxySources(ctx, client, timeout, sources), nil
}

func newProxyProbeClient(proxyURL string) (*http.Client, error) {
	proxy, err := url.Parse(proxyURL)
	if err != nil || (proxy.Scheme != "http" && proxy.Scheme != "https") ||
		proxy.Hostname() == "" || strings.HasSuffix(proxy.Host, ":") ||
		(proxy.Path != "" && proxy.Path != "/") ||
		proxy.RawQuery != "" || proxy.ForceQuery || proxy.Fragment != "" {
		return nil, errors.New("a valid HTTP(S) proxy URL is required")
	}
	if port := proxy.Port(); port != "" {
		if value, err := strconv.ParseUint(port, 10, 16); err != nil || value == 0 {
			return nil, errors.New("invalid HTTP proxy port")
		}
	}
	return &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxy)},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}, nil
}

func probeProxySources(ctx context.Context, client *http.Client, timeout time.Duration, sources []proxyProbeSource) []ProxyProbeResult {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	results := make([]ProxyProbeResult, 0, len(sources))
	for _, source := range sources {
		sourceCtx, cancel := context.WithTimeout(ctx, timeout)
		start := time.Now()
		result, err := source.lookup(sourceCtx, client)
		cancel()
		result.LatencyMS = time.Since(start).Milliseconds()
		if err != nil {
			result.Error = err.Error()
		}
		results = append(results, result)
	}
	return results
}
