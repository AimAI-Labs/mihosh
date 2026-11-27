package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
)

var errUpdateRedirect = errors.New("invalid update redirect")

type updateRequestClient interface {
	Do(*http.Request) (*http.Response, error)
}

type updateHTTPClient struct {
	primary, fallback *http.Client
}

func configuredUpdateHTTPClient(timeout time.Duration) updateHTTPClient {
	client := updateHTTPClient{
		primary: &http.Client{
			Timeout:       timeout,
			Transport:     http.DefaultTransport.(*http.Transport).Clone(),
			CheckRedirect: checkUpdateRedirect,
		},
	}
	cfg, err := config.Load()
	if err != nil || cfg == nil || strings.TrimSpace(cfg.Core.Proxy) == "" {
		return client
	}
	proxy, err := coreinfra.ParseDownloadProxy(strings.TrimSpace(cfg.Core.Proxy))
	if err != nil {
		return client
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = http.ProxyURL(proxy)
	client.primary.Timeout = timeout / 2
	client.fallback = &http.Client{Timeout: timeout / 2, Transport: transport, CheckRedirect: checkUpdateRedirect}
	return client
}

func checkUpdateRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 || req.URL.Scheme != "https" || req.URL.Hostname() == "" || req.URL.User != nil {
		return errUpdateRedirect
	}
	return nil
}

func (c updateHTTPClient) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.primary.Do(req)
	if c.fallback == nil || req.Context().Err() != nil || errors.Is(err, errUpdateRedirect) ||
		(err == nil && resp.StatusCode == http.StatusOK) {
		return resp, err
	}
	if err == nil {
		_ = resp.Body.Close()
	}
	return c.fallback.Do(req)
}

func (c updateHTTPClient) CloseIdleConnections() {
	c.primary.CloseIdleConnections()
	if c.fallback != nil {
		c.fallback.CloseIdleConnections()
	}
}

func safeUpdateRequestError(err error) error {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded, errUpdateRedirect} {
		if errors.Is(err, cause) {
			return cause
		}
	}
	return errors.New("network request failed")
}
