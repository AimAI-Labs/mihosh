package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
)

func TestQueryDNS(t *testing.T) {
	var gotName, gotType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dns/query" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		gotName = r.URL.Query().Get("name")
		gotType = r.URL.Query().Get("type")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"Status": 0,
			"Question": [{"Name": "example.com.", "Qtype": 1, "Qclass": 1}],
			"Answer": [
				{"name": "example.com.", "type": 1, "TTL": 60, "data": "198.18.0.12"}
			]
		}`))
	}))
	defer srv.Close()

	client := NewClient(config.MihomoEndpoint{ExternalController: strings.TrimPrefix(srv.URL, "http://")}, 5000)
	resp, err := client.QueryDNS("example.com", "a")
	if err != nil {
		t.Fatalf("QueryDNS failed: %v", err)
	}
	if gotName != "example.com" || gotType != "A" {
		t.Fatalf("unexpected query params: name=%q type=%q", gotName, gotType)
	}
	if resp.Status != 0 || len(resp.Answer) != 1 || resp.Answer[0].Data != "198.18.0.12" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if !resp.HasFakeIP() || model.ClassifyDNSIP(resp.Answer[0].Data) != "fake-ip" {
		t.Fatalf("expected 198.18.0.12 to be detected as fake-ip")
	}
}

func TestQueryDNSRejectsEmptyDomain(t *testing.T) {
	client := NewClient(config.MihomoEndpoint{ExternalController: "127.0.0.1:9090"}, 1000)
	if _, err := client.QueryDNS("   ", "A"); err == nil {
		t.Fatal("expected error for empty domain")
	}
}
