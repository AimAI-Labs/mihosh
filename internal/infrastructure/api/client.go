package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
)

// Client mihomo API 客户端
//
// baseURL/secret 支持运行时热更新（UpdateEndpoint），用 mu 保护，
// 使得用户在设置页改完 external-controller/secret 后无需重启 mihosh 即可生效。
// baseURL 存原值（无 scheme，与 mihomo 配置文件一致）；仅 HTTP 层补 scheme。
type Client struct {
	mu         sync.RWMutex
	baseURL    string
	secret     string
	httpClient *http.Client
}

// NewClient 创建新的 API 客户端。
// endpoint.ExternalController 存原值（无 scheme），由 doRawRequest 在请求时补 http://。
func NewClient(endpoint config.MihomoEndpoint, timeout int) *Client {
	return &Client{
		baseURL: endpoint.ExternalController,
		secret:  endpoint.Secret,
		httpClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Millisecond,
		},
	}
}

// UpdateEndpoint 原子更新 baseURL 与 secret。
// 后续 DoRequest 会使用新值；httpClient 的 timeout 等保持不变。
func (c *Client) UpdateEndpoint(baseURL, secret string) {
	c.mu.Lock()
	c.baseURL = baseURL
	c.secret = secret
	c.mu.Unlock()
}

// ensureScheme 为 baseURL 补 http:// 前缀（若无 http:///https://）。
// external-controller 原值无 scheme，与 mihomo 配置文件一致；仅 HTTP 层补全。
func ensureScheme(baseURL string) string {
	if strings.HasPrefix(baseURL, "http://") || strings.HasPrefix(baseURL, "https://") {
		return baseURL
	}
	return "http://" + baseURL
}

// DoRequest 执行 HTTP 请求（导出供 endpoints 使用）
func (c *Client) DoRequest(method, path string, body interface{}) ([]byte, error) {
	resp, err := c.doRawRequest(method, path, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (c *Client) doRawRequest(method, path string, body interface{}) (*http.Response, error) {
	// 快照当前 baseURL/secret：避免持锁执行 HTTP，且对单次请求保证地址一致。
	c.mu.RLock()
	baseURL := c.baseURL
	secret := c.secret
	c.mu.RUnlock()

	reqURL := ensureScheme(baseURL) + path

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, reqURL, reqBody)
	if err != nil {
		return nil, err
	}

	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API 请求失败: %s - %s", resp.Status, string(bodyBytes))
	}

	return resp, nil
}

// NewHTTPClientWithProxy 创建一个带代理的 HTTP 客户端
func (c *Client) NewHTTPClientWithProxy(proxyAddr string) (*http.Client, error) {
	if proxyAddr == "" {
		return &http.Client{Timeout: c.httpClient.Timeout}, nil
	}

	// 统一处理协议前缀
	if !strings.HasPrefix(proxyAddr, "http://") && !strings.HasPrefix(proxyAddr, "https://") && !strings.HasPrefix(proxyAddr, "socks5://") {
		proxyAddr = "http://" + proxyAddr
	}

	proxyURL, err := url.Parse(proxyAddr)
	if err != nil {
		return nil, err
	}

	return &http.Client{
		Timeout: c.httpClient.Timeout,
		Transport: &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				return proxyURL, nil
			},
		},
	}, nil
}

// VersionInfo 表示 Mihomo 的版本信息
type VersionInfo struct {
	Premium bool   `json:"premium"`
	Version string `json:"version"`
	Meta    bool   `json:"meta"`
}

// GetVersion 获取 Mihomo 内核版本信息
func (c *Client) GetVersion() (*VersionInfo, error) {
	data, err := c.DoRequest(http.MethodGet, "/version", nil)
	if err != nil {
		return nil, err
	}

	var info VersionInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	return &info, nil
}
