package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
)

func TestProxyService_TestAllProxies(t *testing.T) {
	// Setup a mock server that returns different delays or errors based on the proxy name
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyName := r.URL.Path[len("/proxies/") : len(r.URL.Path)-len("/delay")]
		if proxyName == "TimeoutProxy" {
			time.Sleep(100 * time.Millisecond) // Simulate slow response, test handles this via context or client timeout
			w.WriteHeader(http.StatusGatewayTimeout)
			return
		}
		if proxyName == "ErrorProxy" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		
		// Success case
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"delay": 42}`)
	}))
	defer ts.Close()

	endpoint := config.MihomoEndpoint{
		ExternalController: ts.URL,
		Secret:             "",
	}
	client := api.NewClient(endpoint, 2000)
	svc := NewProxyService(client, "http://www.gstatic.com/generate_204", 2000)

	tests := []struct {
		name    string
		proxies []string
		want    map[string]int
	}{
		{
			name:    "all successful",
			proxies: []string{"Proxy1", "Proxy2", "Proxy3"},
			want: map[string]int{
				"Proxy1": 42,
				"Proxy2": 42,
				"Proxy3": 42,
			},
		},
		{
			name:    "with errors",
			proxies: []string{"Proxy1", "ErrorProxy", "TimeoutProxy"},
			want: map[string]int{
				"Proxy1":       42,
				"ErrorProxy":   -1,
				"TimeoutProxy": -1,
			},
		},
		{
			name:    "empty list",
			proxies: []string{},
			want:    map[string]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.TestAllProxies(tt.proxies)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ProxyService.TestAllProxies() = %v, want %v", got, tt.want)
			}
		})
	}
}
