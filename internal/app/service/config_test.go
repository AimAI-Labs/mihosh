package service

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
)

func managedConfigFile(t *testing.T, contents string) string {
	t.Helper()
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	cfg := config.DefaultConfig
	cfg.Core.Mode = "managed"
	if err := config.Save(&cfg); err != nil {
		t.Fatal(err)
	}
	path, err := config.ManagedMihomoConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestConfigReadUsesFileValues(t *testing.T) {
	managedConfigFile(t, "external-controller: 127.0.0.1:9191\nsecret: from-file\nmixed-port: 7891\nallow-lan: true\nlog-level: debug\n")
	result := NewConfigService().ReadMihomoConfig()
	if result.ExternalController != "127.0.0.1:9191" || result.Secret != "from-file" || result.MixedPort != 7891 || !result.AllowLan || result.LogLevel != "debug" {
		t.Fatalf("unexpected file configuration: %+v", result)
	}
}

func TestConfigReadRetainsDefaultFallback(t *testing.T) {
	path := managedConfigFile(t, "")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	result := NewConfigService().ReadMihomoConfig()
	if result == nil || result.ExternalController != config.DefaultMihomoEndpoint.ExternalController || result.MixedPort != config.DefaultMihomoEndpoint.MixedPort {
		t.Fatalf("default fallback changed: %+v", result)
	}
}

func TestConfigSaveWriteAndApplyOutcomes(t *testing.T) {
	failure := errors.New("injected failure")
	for _, tc := range []struct {
		name                          string
		pathErr, writeErr, restartErr error
		status                        int
		nilClient                     bool
		writeOK, applied, wantErr     bool
		order                         []string
	}{
		{"path failure", failure, nil, nil, 204, false, false, false, true, []string{"path"}},
		{"write failure", nil, failure, nil, 204, false, false, false, true, []string{"path", "write"}},
		{"reload success skips service restart", nil, nil, nil, 204, false, true, true, false, []string{"path", "write", "reload"}},
		{"fallback restart success after reload failure", nil, nil, nil, 500, false, true, true, false, []string{"path", "write", "reload", "restart"}},
		{"all apply methods fail", nil, nil, failure, 500, false, true, false, true, []string{"path", "write", "reload", "restart"}},
		{"offline file-only save skips service restart", nil, nil, failure, 204, true, true, false, false, []string{"path", "write"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
			events := make(chan string, 4)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				events <- "reload"
				if r.Method != http.MethodPut || r.URL.Path != "/configs" || r.URL.Query().Get("force") != "true" {
					t.Errorf("unexpected apply request: %s %s", r.Method, r.URL)
				}
				w.WriteHeader(tc.status)
			}))
			defer server.Close()
			client := api.NewClient(config.MihomoEndpoint{ExternalController: server.URL}, 1000)
			if tc.nilClient {
				client = nil
			}
			s := NewConfigService()
			s.mihomoConfigPath = func() (string, error) { events <- "path"; return "config.yaml", tc.pathErr }
			s.writeMihomoField = func(path, key string, value interface{}) error {
				events <- "write"
				if path != "config.yaml" || key != "mixed-port" || value != 7891 {
					t.Errorf("write arguments changed: %s %s %v", path, key, value)
				}
				return tc.writeErr
			}
			s.restartMihomo = func() error { events <- "restart"; return tc.restartErr }
			result := s.SaveMihomoConfigField(client, "mixed-port", 7891)
			if result.WriteOK != tc.writeOK || result.Applied != tc.applied || (result.Err != nil) != tc.wantErr {
				t.Fatalf("unexpected outcome: %+v", result)
			}
			var order []string
			for len(events) > 0 {
				order = append(order, <-events)
			}
			if !reflect.DeepEqual(order, tc.order) {
				t.Fatalf("side effects=%v, want %v", order, tc.order)
			}
			if (tc.pathErr != nil || tc.writeErr != nil) && !errors.Is(result.Err, failure) {
				t.Fatal("write/path error lost its cause")
			}
		})
	}
}

func TestConfigManagedFirstWriteSkipsSudoAndPreservesFileOnReloadFailure(t *testing.T) {
	path := managedConfigFile(t, "")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "offline", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	s := NewConfigService()
	s.restartMihomo = func() error { t.Fatal("managed mode should not invoke system service restart"); return nil }
	client := api.NewClient(config.MihomoEndpoint{ExternalController: server.URL}, 1000)
	result := s.SaveMihomoConfigField(client, "allow-lan", true)
	if !result.WriteOK || result.Applied || result.Err == nil {
		t.Fatalf("partial save lost its write fact: %+v", result)
	}
	data, err := config.ReadMihomoYAML(path)
	if err != nil || data["allow-lan"] != true {
		t.Fatalf("first write was not preserved: %v, %v", data, err)
	}
}

func TestConfigReloadRetainsAPIOnlyPolicy(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusServiceUnavailable} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
		client := api.NewClient(config.MihomoEndpoint{ExternalController: server.URL}, 1000)
		s := NewConfigService()
		s.restartMihomo = func() error { t.Fatal("CLI reload attempted a service restart"); return nil }
		s.writeMihomoField = func(string, string, interface{}) error { t.Fatal("reload attempted another write"); return nil }
		result := s.ReloadMihomoConfig(client, "already-edited.yaml")
		server.Close()
		if !result.WriteOK || result.Applied != (status == http.StatusNoContent) || (result.Err == nil) != result.Applied {
			t.Fatalf("unexpected API-only outcome: %+v", result)
		}
	}
}
