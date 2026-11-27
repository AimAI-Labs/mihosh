package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/stretchr/testify/require"
)

func TestDelayRequestsUseDistinctRoutesAndResponseSlack(t *testing.T) {
	const target = "Proxy / EU"
	const testURL = "https://test.example/204?fixture=yes"
	const timeout = 30
	for _, prefix := range []string{"/group/", "/proxies/"} {
		t.Run(prefix, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				wantPath := prefix + url.PathEscape(target) + "/delay"
				if r.Method != http.MethodGet || r.URL.EscapedPath() != wantPath {
					t.Errorf("request: %s %s, want GET %s", r.Method, r.URL.EscapedPath(), wantPath)
					http.NotFound(w, r)
					return
				}
				if r.URL.Query().Get("timeout") != strconv.Itoa(timeout) || r.URL.Query().Get("url") != testURL {
					t.Errorf("unexpected query: %s", r.URL.RawQuery)
				}
				if r.Header.Get("Authorization") != "Bearer fixture" {
					t.Error("delay request lost its controller credentials")
				}
				// Simulate the response arriving shortly after the controller's test budget.
				time.Sleep((timeout + 20) * time.Millisecond)
				if prefix == "/group/" {
					_, _ = io.WriteString(w, `{"proxy":42}`)
				} else {
					_, _ = io.WriteString(w, `{"delay":42}`)
				}
			}))
			defer server.Close()
			client := NewClient(config.MihomoEndpoint{ExternalController: server.URL, Secret: "fixture"}, timeout)
			if prefix == "/group/" {
				require.NoError(t, client.TestGroupDelay(target, testURL, timeout))
			} else {
				delay, err := client.TestProxyDelay(target, testURL, timeout)
				require.NoError(t, err)
				require.Equal(t, 42, delay)
			}
			require.Equal(t, timeout*time.Millisecond, client.httpClient.Timeout, "regular API calls keep their original timeout")
		})
	}
}
