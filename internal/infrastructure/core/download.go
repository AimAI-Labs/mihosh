// Package core implements installation and lifecycle primitives for managed Mihomo.
package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const MaxAssetBytes int64 = 256 << 20

// Accelerator describes a prefix gateway's supported resources.
type Accelerator struct {
	URL      string `mapstructure:"url" json:"url"`
	Metadata bool   `mapstructure:"metadata" json:"metadata"`
	Asset    bool   `mapstructure:"asset" json:"asset"`
}

func DefaultAccelerators() []Accelerator {
	return []Accelerator{{"https://gh-proxy.org/", true, true}}
}

// Progress is a snapshot; consumers should retain only the latest value.
type Progress struct {
	BytesPerSecond int64  `json:"bytes_per_second"`
	Route          string `json:"route"`
	Downloaded     int64  `json:"downloaded"`
	Total          int64  `json:"total"`
}

type Route struct {
	Name    string
	Gateway string
	Proxy   *url.URL
}

func (r Route) Trusted() bool { return r.Gateway == "" }

// Fetcher never uses environment proxy settings during the direct or gateway stage.
// Its settings are immutable while a request is running.
type Fetcher struct {
	DownloadTimeout time.Duration
	Proxy           string
	Accelerators    []Accelerator
	RunningProxy    string
	Notify          func(Progress)
	clientFor       func(Route, bool) *http.Client
	envProxy        func(*http.Request) (*url.URL, error)
}

func NewFetcher() *Fetcher {
	return &Fetcher{Accelerators: DefaultAccelerators(), envProxy: http.ProxyFromEnvironment}
}

func validProxy(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("invalid HTTP(S) download proxy")
	}
	return u, nil
}

// Only canonical URLs belonging to the official repository may enter the gateway chain.
func validateSource(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" {
		return errors.New("invalid official Mihomo URL")
	}
	switch u.Hostname() {
	case "api.github.com":
		if u.Path == "/repos/MetaCubeX/mihomo/releases" {
			q, err := url.ParseQuery(u.RawQuery)
			page, pageErr := strconv.Atoi(q.Get("page"))
			if err == nil && len(q) == 2 && len(q["page"]) == 1 && len(q["per_page"]) == 1 &&
				q.Get("per_page") == strconv.Itoa(ReleasePageSize) && pageErr == nil && page >= 1 {
				return nil
			}
			return errors.New("invalid release list query")
		}
		if u.RawQuery != "" {
			return errors.New("unexpected release query")
		}
		if strings.HasPrefix(u.Path, "/repos/MetaCubeX/mihomo/releases/") {
			return nil
		}
	case "github.com":
		if u.RawQuery != "" {
			return errors.New("unexpected asset query")
		}
		if strings.HasPrefix(u.Path, "/MetaCubeX/mihomo/releases/download/") {
			return nil
		}
	}
	return errors.New("URL does not belong to official Mihomo releases")
}

func (f *Fetcher) routes(raw string, metadata bool) ([]Route, error) {
	if f.DownloadTimeout != 0 && (f.DownloadTimeout < 30*time.Second || f.DownloadTimeout > time.Hour) {
		return nil, errors.New("download timeout must be between 30 and 3600 seconds")
	}
	if err := validateSource(raw); err != nil {
		return nil, err
	}
	routes := []Route{{Name: "direct"}}
	seen := map[string]bool{}
	addProxy := func(p *url.URL) {
		if p == nil || seen[p.String()] {
			return
		}
		seen[p.String()] = true
		routes = append(routes, Route{Name: "proxy:" + p.Host, Proxy: p})
	}
	if f.Proxy != "" {
		p, err := validProxy(f.Proxy)
		if err != nil {
			return nil, err
		}
		addProxy(p)
	}
	if f.envProxy != nil {
		req, _ := http.NewRequest(http.MethodGet, raw, nil)
		p, err := f.envProxy(req)
		if err == nil && p != nil {
			if p, err = validProxy(p.String()); err == nil {
				addProxy(p)
			}
		}
	}
	if f.RunningProxy != "" {
		p, err := validProxy(f.RunningProxy)
		if err == nil {
			addProxy(p)
		}
	}
	for _, a := range f.Accelerators {
		if metadata && !a.Metadata || !metadata && !a.Asset {
			continue
		}
		u, err := url.Parse(a.URL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.Port() != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" {
			return nil, errors.New("invalid HTTPS accelerator address")
		}
		base := strings.TrimRight(u.String(), "/") + "/"
		if seen[base] {
			continue
		}
		seen[base] = true
		routes = append(routes, Route{Name: "accelerator:" + u.Host, Gateway: base})
	}
	return routes, nil
}

func allowedRedirect(u *url.URL, route Route) bool {
	if u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	switch host {
	case "github.com", "api.github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com":
		return true
	}
	if route.Gateway != "" {
		base, _ := url.Parse(route.Gateway)
		if host == base.Hostname() {
			return true
		}
		if (base.Hostname() == "gh-proxy.org" || base.Hostname() == "gh-proxy.com") && (host == "gh-proxy.org" || host == "gh-proxy.com") {
			return true
		}
	}
	return false
}

func (f *Fetcher) client(route Route, metadata bool) *http.Client {
	if f.clientFor != nil {
		return f.clientFor(route, metadata)
	}
	client := newDownloadClient(route, metadata)
	if !metadata && f.DownloadTimeout > 0 {
		client.Timeout = client.Timeout * (f.DownloadTimeout / time.Second) / 600
	}
	return client
}

func newDownloadClient(route Route, metadata bool) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	if route.Proxy != nil {
		transport.Proxy = http.ProxyURL(route.Proxy)
	}
	transport.DialContext = (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	transport.TLSHandshakeTimeout = 5 * time.Second
	transport.ResponseHeaderTimeout = 15 * time.Second
	timeout := 3 * time.Minute
	if route.Gateway == "" {
		timeout = 60 * time.Second
		if route.Proxy != nil {
			timeout = 90 * time.Second
		}
	}
	if metadata {
		timeout = 15 * time.Second
	}
	return &http.Client{Transport: transport, Timeout: timeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || !allowedRedirect(req.URL, route) {
			return errors.New("unapproved download redirect")
		}
		req.Header.Del("Authorization")
		req.Header.Del("Cookie")
		return nil
	}}
}

// request validates the complete response within each route before accepting success.
// A rejected response is discarded before the next route, including interrupted files.
func (f *Fetcher) request(ctx context.Context, raw string, metadata bool, consume func(*http.Response, Route) error) (Route, error) {
	routes, err := f.routes(raw, metadata)
	if err != nil {
		return Route{}, err
	}
	budget := 10 * time.Minute
	if f.DownloadTimeout > 0 {
		budget = f.DownloadTimeout
	}
	if metadata {
		budget = 90 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	var failures []string
	attemptedGateways := map[string]bool{}
	for _, route := range routes {
		if route.Gateway != "" && attemptedGateways[route.Gateway] {
			continue
		}
		if err := ctx.Err(); err != nil {
			return Route{}, err
		}
		if f.Notify != nil {
			f.Notify(Progress{Route: route.Name, Total: -1})
		}
		target := raw
		if route.Gateway != "" {
			target = route.Gateway + raw
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		req.Header.Set("User-Agent", "mihosh-core-installer")
		req.Header.Set("Accept", "application/octet-stream")
		if metadata {
			req.Header.Set("Accept", "application/vnd.github+json")
		}
		client := f.client(route, metadata)
		routeCtx := ctx
		routeCancel := func() {}
		if client.Timeout > 0 {
			routeCtx, routeCancel = context.WithTimeout(ctx, client.Timeout)
		}
		req = req.WithContext(routeCtx)
		resp, requestErr := client.Do(req)
		// One retry shares this route's deadline; it cannot consume the time reserved
		// for later proxy/gateway stages. Only idempotent public GETs are issued here.
		retry := requestErr != nil
		if resp != nil {
			status := resp.StatusCode
			retry = status == 408 || status == 429 || status >= 500 || (status == 403 && (route.Gateway != "" || resp.Header.Get("X-RateLimit-Remaining") == "0"))
		}
		if retry && routeCtx.Err() == nil {
			wait := time.Second
			if resp != nil {
				wait = max(wait, retryDelay(resp.Header.Get("Retry-After")))
				_ = resp.Body.Close()
			}
			timer := time.NewTimer(wait)
			select {
			case <-routeCtx.Done():
				timer.Stop()
			case <-timer.C:
				resp, requestErr = client.Do(req.Clone(routeCtx))
			}
		}
		if routeCtx.Err() != nil {
			requestErr = routeCtx.Err()
			if resp != nil {
				_ = resp.Body.Close()
			}
		}
		fatal := false
		detail := "network request failed" // Transport errors can contain proxy credentials or signed URLs.
		if requestErr == nil {
			if route.Gateway != "" && resp.Request != nil {
				final := resp.Request.URL
				attemptedGateways[final.Scheme+"://"+final.Host+"/"] = true
			}
			if resp.StatusCode == http.StatusOK {
				requestErr = consume(resp, route)
				if requestErr != nil {
					detail = requestErr.Error()
				}
			} else {
				detail = fmt.Sprintf("HTTP %d", resp.StatusCode)
				requestErr = errors.New(detail)
				fatal = route.Trusted() && (resp.StatusCode == 404 || resp.StatusCode == 401 || resp.StatusCode == 400 || resp.StatusCode == 422)
				if resp.StatusCode == 429 || resp.StatusCode == 403 && resp.Header.Get("X-RateLimit-Remaining") == "0" {
					// Do not immediately issue the same upstream request through another route.
					wait := retryDelay(resp.Header.Get("Retry-After"))
					if wait > 0 {
						_ = resp.Body.Close()
						timer := time.NewTimer(wait)
						select {
						case <-routeCtx.Done():
							timer.Stop()
							if ctx.Err() != nil {
								routeCancel()
								client.CloseIdleConnections()
								return Route{}, ctx.Err()
							}
						case <-timer.C:
						}
					}
				}
			}
			_ = resp.Body.Close()
		}
		client.CloseIdleConnections()
		routeCancel()
		if requestErr == nil {
			return route, nil
		}
		if err := ctx.Err(); err != nil {
			return Route{}, err
		}
		failures = append(failures, route.Name+": "+detail)
		if fatal {
			break
		}
	}
	return Route{}, errors.New(strings.Join(failures, "; "))
}

func retryDelay(value string) time.Duration {
	if n, err := strconv.Atoi(value); err == nil && n > 0 {
		return time.Duration(min(n, 600)) * time.Second
	}
	if date, err := http.ParseTime(value); err == nil && date.After(time.Now()) {
		return min(time.Until(date), 10*time.Minute)
	}
	return 0
}

func (f *Fetcher) Metadata(ctx context.Context, raw string, validate func([]byte) error) ([]byte, Route, error) {
	var data []byte
	route, err := f.request(ctx, raw, true, func(resp *http.Response, _ Route) error {
		b, err := io.ReadAll(io.LimitReader(resp.Body, (4<<20)+1))
		if err != nil {
			return errors.New("metadata transfer interrupted")
		}
		if len(b) > 4<<20 {
			return errors.New("metadata exceeds size limit")
		}
		if err = validate(b); err != nil {
			return err
		}
		data = b
		return nil
	})
	return data, route, err
}

// Download requires a SHA-256 independently trusted by the caller. Each route starts
// from zero; only a fully checked file is renamed to destination.
func (f *Fetcher) Download(ctx context.Context, raw, destination, expected string) (Route, error) {
	digest, err := hex.DecodeString(strings.TrimPrefix(expected, "sha256:"))
	if err != nil || len(digest) != sha256.Size {
		return Route{}, errors.New("a trusted SHA-256 digest is required")
	}
	part := destination + ".part"
	defer func() { _ = os.Remove(part) }() // The verified file is renamed on success; failed transfers are best-effort cleanup.
	route, err := f.request(ctx, raw, false, func(resp *http.Response, route Route) error {
		if resp.ContentLength > MaxAssetBytes {
			return errors.New("asset exceeds size limit")
		}
		file, err := os.OpenFile(part, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err != nil {
			return errors.New("cannot create download file")
		}
		hash := sha256.New()
		var done int64
		started := time.Now()
		reader := io.LimitReader(resp.Body, MaxAssetBytes+1)
		buf := make([]byte, 64<<10)
		// Cancel a stalled body independently of the overall download deadline.
		idle := time.AfterFunc(30*time.Second, func() { _ = resp.Body.Close() })
		defer idle.Stop()
		for {
			n, readErr := reader.Read(buf)
			if n > 0 {
				idle.Reset(30 * time.Second)
				if _, err = file.Write(buf[:n]); err != nil {
					_ = file.Close()
					return errors.New("cannot write download file")
				}
				hash.Write(buf[:n])
				done += int64(n)
				if f.Notify != nil {
					f.Notify(Progress{Route: route.Name, Downloaded: done, Total: resp.ContentLength, BytesPerSecond: int64(float64(done) / max(time.Since(started).Seconds(), 0.001))})
				}
			}
			if readErr != nil {
				if readErr != io.EOF {
					_ = file.Close()
					return errors.New("asset transfer interrupted")
				}
				break
			}
		}
		if err = file.Close(); err != nil {
			return errors.New("cannot close download file")
		}
		if done > MaxAssetBytes {
			return errors.New("asset exceeds size limit")
		}
		if hex.EncodeToString(hash.Sum(nil)) != hex.EncodeToString(digest) {
			return errors.New("asset SHA-256 mismatch")
		}
		return nil
	})
	if err != nil {
		return Route{}, err
	}
	if err = os.Rename(part, destination); err != nil {
		return Route{}, errors.New("cannot commit verified download")
	}
	return route, nil
}

// ValidateSettings checks syntax without accessing the network.
func (f *Fetcher) ValidateSettings() error {
	_, err := f.routes(releaseAPI+"latest", true)
	if err != nil {
		return err
	}
	_, err = f.routes(testSettingsAssetURL, false)
	return err
}

const testSettingsAssetURL = "https://github.com/MetaCubeX/mihomo/releases/download/v1.0.0/asset.gz"
