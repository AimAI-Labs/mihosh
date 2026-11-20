package profile

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDownloadWithOptionsCustomUserAgentProxyAndSkipTLS(t *testing.T) {
	// 1. Custom User-Agent
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte("proxies: []\n"))
	}))
	defer srv.Close()

	_, _, err := DownloadWithOptions(context.Background(), SubSource{
		Kind:      SourceRemote,
		URL:       srv.URL,
		UserAgent: "CustomMihoshUA/1.0",
	}, DownloadOptions{})
	if err != nil {
		t.Fatalf("custom UA download failed: %v", err)
	}
	if gotUA != "CustomMihoshUA/1.0" {
		t.Fatalf("expected custom UA CustomMihoshUA/1.0, got %q", gotUA)
	}

	// 2. UseProxy routes through the configured HTTP proxy
	proxyHit := false
	proxySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHit = true
		_, _ = w.Write([]byte("proxies: []\n"))
	}))
	defer proxySrv.Close()

	_, _, err = DownloadWithOptions(context.Background(), SubSource{
		Kind:     SourceRemote,
		URL:      "http://sub.invalid/config.yaml",
		UseProxy: true,
	}, DownloadOptions{ProxyURL: proxySrv.URL})
	if err != nil {
		t.Fatalf("proxied download failed: %v", err)
	}
	if !proxyHit {
		t.Fatal("expected request to pass through configured HTTP proxy")
	}

	// 3. InsecureSkipVerify allows self-signed HTTPS while default rejects it
	tlsSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("proxies: []\n"))
	}))
	defer tlsSrv.Close()

	if _, _, err := DownloadWithOptions(context.Background(), SubSource{
		Kind: SourceRemote,
		URL:  tlsSrv.URL,
	}, DownloadOptions{}); err == nil {
		t.Fatal("expected self-signed TLS server to fail verification by default")
	}

	if _, _, err := DownloadWithOptions(context.Background(), SubSource{
		Kind:               SourceRemote,
		URL:                tlsSrv.URL,
		InsecureSkipVerify: true,
	}, DownloadOptions{}); err != nil {
		t.Fatalf("expected InsecureSkipVerify=true to succeed, got %v", err)
	}
}
