package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type probeRoundTripper func(*http.Request) (*http.Response, error)

func (probe probeRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return probe(request)
}

func TestProxyProbeOnlyUsesExplicitHTTPProxy(t *testing.T) {
	t.Setenv("NO_PROXY", "*")
	t.Setenv("no_proxy", "*")
	client, err := newProxyProbeClient("http://127.0.0.1:7890")
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodGet, "https://api.ip.sb/geoip", nil)
	require.NoError(t, err)
	transport := client.Transport.(*http.Transport)
	proxy, err := transport.Proxy(request)
	require.NoError(t, err)
	require.Equal(t, "http://127.0.0.1:7890", proxy.String())
	require.ErrorIs(t, client.CheckRedirect(request, nil), http.ErrUseLastResponse)

	for _, invalid := range []string{"", "127.0.0.1:7890", "socks5://127.0.0.1:7890", "http://:7890", "http://127.0.0.1:0", "http://127.0.0.1:65536", "https://proxy.example/path"} {
		results, err := TestProxySources(context.Background(), invalid, time.Second)
		require.Error(t, err, "%q", invalid)
		require.Empty(t, results)
	}
}

func TestProxyProbeKeepsResultsAcrossSourceFailures(t *testing.T) {
	sources := []proxyProbeSource{
		jsonProxyProbe{name: "blocked", endpoint: "https://blocked.example"},
		jsonProxyProbe{name: "invalid", endpoint: "https://invalid.example"},
		jsonProxyProbe{name: "large", endpoint: "https://large.example"},
		jsonProxyProbe{name: "valid", endpoint: "https://valid.example"},
	}
	var requested []string
	client := &http.Client{Transport: probeRoundTripper(func(request *http.Request) (*http.Response, error) {
		_, bounded := request.Context().Deadline()
		require.True(t, bounded)
		require.Equal(t, "application/json", request.Header.Get("Accept"))
		host := request.URL.Hostname()
		requested = append(requested, host)
		status := http.StatusOK
		body := `{"ip":"2001:db8::1","country":"Example"}`
		switch host {
		case "blocked.example":
			status = http.StatusTooManyRequests
		case "invalid.example":
			body = `{"ip":"not an IP"}`
		case "large.example":
			body += strings.Repeat(" ", maxProxyProbeBody)
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	results := probeProxySources(context.Background(), client, time.Second, sources)
	require.Equal(t, []string{"blocked.example", "invalid.example", "large.example", "valid.example"}, requested)
	require.Len(t, results, len(sources))
	require.Equal(t, "blocked", results[0].Source)
	require.Contains(t, results[0].Error, "429")
	require.Contains(t, results[1].Error, "valid address")
	require.Contains(t, results[2].Error, "size limit")
	require.Equal(t, "valid", results[3].Source)
	require.Equal(t, "2001:db8::1", results[3].IP)
	require.Equal(t, "Example", results[3].Country)
	require.Empty(t, results[3].Error)
}
