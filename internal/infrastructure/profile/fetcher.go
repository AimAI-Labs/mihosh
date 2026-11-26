package profile

// fetcher.go — 订阅原始配置的拉取/读取。
//
// SourceRemote: 独立 http.Client（30s 超时，不走 mihomo api.Client），UA: mihosh/<ver>；
//               token 仅支持 URL 内嵌（?token= 或 basic auth in URL）；
//               响应按 YAML 解析校验，非法则拒绝并保留旧 raw。
// SourceLocal:  直接读文件，同样做 YAML 校验。

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"maps"
	"math"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"gopkg.in/yaml.v3"
)

// fetchTimeout 远程拉取超时。
const fetchTimeout = 30 * time.Second

// MaxRemoteSubscriptionBytes bounds the decoded HTTP response before parsing it.
const MaxRemoteSubscriptionBytes = 8 << 20

// fetchUserAgent 远程拉取使用的 User-Agent，伪装成 clash-verge 以获取订阅后端的完整 Clash 规则。
const fetchUserAgent = "clash-verge/v2.4.5"

// Fetch 拉取订阅原始配置，写回 raw.yaml。
//   - remote: HTTP GET，状态码非 2xx 视为失败；
//   - local: 读取文件。
//
// 写入前对内容做 YAML 合法性校验，并要求顶层为 mapping（mihomo 配置必须是键值结构），
// 非法则返回错误且不覆盖已有 raw。这能拦截订阅端返回的 HTML 错误页等非配置响应。
func Fetch(p Profile) error { return FetchContext(context.Background(), p) }

func FetchContext(ctx context.Context, p Profile) error {
	raw, _, err := Download(ctx, p.Source)
	if err != nil {
		return err
	}
	return WriteRawForSource(p, raw)
}

// DownloadOptions carries runtime transport context resolved by the caller
// (e.g., Mihomo's active mixed-port proxy URL when SubSource.UseProxy is true).
type DownloadOptions struct {
	ProxyURL string
}

// Download validates subscription content and returns the usage reported by the same response.
func Download(ctx context.Context, source SubSource) ([]byte, *SubscriptionUsage, error) {
	return DownloadWithOptions(ctx, source, DownloadOptions{})
}

// DownloadWithOptions validates subscription content using source fetch options and caller options.
func DownloadWithOptions(ctx context.Context, source SubSource, opts DownloadOptions) ([]byte, *SubscriptionUsage, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	var data []byte
	var usage *SubscriptionUsage
	var err error
	if source.Kind == SourceLocal {
		data, err = fetchLocal(source.Path)
	} else {
		data, usage, err = fetchRemoteContext(ctx, source, opts)
	}
	if err != nil {
		return nil, nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	// 校验 YAML 合法性 + 顶层必须为 mapping。
	var probe yaml.Node
	yamlErr := yaml.Unmarshal(data, &probe)
	if yamlErr == nil && topLevelMapping(&probe) != nil {
		// 合法 Mihomo YAML，直接写入
		return data, usage, nil
	}

	// 非合法 Mihomo YAML，尝试作为 v2ray base64 订阅解析
	converted, isV2Ray, convErr := TryConvertV2Ray(data)
	if isV2Ray && convErr == nil {
		return converted, usage, nil
	}
	if isV2Ray {
		return nil, nil, fmt.Errorf("v2ray 订阅解析失败: %w", convErr)
	}

	// 两者皆非，返回原始 YAML 错误
	if yamlErr != nil {
		return nil, nil, fmt.Errorf("订阅内容非合法 YAML: %w", yamlErr)
	}
	return nil, nil, fmt.Errorf("订阅内容顶层不是配置映射（可能是错误页或纯文本），已拒绝")
}

func fetchRemoteContext(ctx context.Context, source SubSource, opts DownloadOptions) ([]byte, *SubscriptionUsage, error) {
	subURL := strings.TrimSpace(source.URL)
	if subURL == "" {
		return nil, nil, fmt.Errorf("%s", i18n.T("profile.fetcher.err_empty_url"))
	}

	if !strings.HasPrefix(subURL, "http://") && !strings.HasPrefix(subURL, "https://") {
		// 禁止 file:// 等其它协议，防止与 local 混淆绕过。
		return nil, nil, fmt.Errorf("%s", i18n.T("profile.fetcher.err_only_http"))
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	defer transport.CloseIdleConnections()
	if source.UseProxy {
		proxyRaw := strings.TrimSpace(opts.ProxyURL)
		if proxyRaw != "" {
			if !strings.Contains(proxyRaw, "://") {
				proxyRaw = "http://" + proxyRaw
			}
			parsedProxy, err := url.Parse(proxyRaw)
			if err != nil {
				return nil, nil, fmt.Errorf(i18n.T("profile.fetcher.err_build_req"), err)
			}
			transport.Proxy = http.ProxyURL(parsedProxy)
		} else {
			transport.Proxy = http.ProxyFromEnvironment
		}
	}
	if source.InsecureSkipVerify {
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{}
		}
		transport.TLSClientConfig.InsecureSkipVerify = true
	}

	client := &http.Client{Timeout: fetchTimeout, Transport: transport}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, subURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf(i18n.T("profile.fetcher.err_build_req"), err)
	}
	ua := strings.TrimSpace(source.UserAgent)
	if ua == "" {
		ua = fetchUserAgent
	}
	req.Header.Set("User-Agent", ua)

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf(i18n.T("profile.fetcher.err_fetch"), err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf(i18n.T("profile.fetcher.err_status_code"), resp.StatusCode)
	}

	if resp.ContentLength > MaxRemoteSubscriptionBytes {
		return nil, nil, fmt.Errorf("%s", i18n.Tf("profile.fetcher.err_size_limit", MaxRemoteSubscriptionBytes>>20))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxRemoteSubscriptionBytes+1))
	if err != nil {
		return nil, nil, fmt.Errorf(i18n.T("profile.fetcher.err_read_resp"), err)
	}
	if len(body) > MaxRemoteSubscriptionBytes {
		return nil, nil, fmt.Errorf("%s", i18n.Tf("profile.fetcher.err_size_limit", MaxRemoteSubscriptionBytes>>20))
	}
	return body, subscriptionUsage(resp.Header), nil
}

func subscriptionUsage(headers http.Header) *SubscriptionUsage {
	if usage := parseSubscriptionUsage(strings.Join(headers.Values("Subscription-Userinfo"), ";")); usage != nil {
		return usage
	}
	// Object storage providers can prefix custom response metadata.
	for _, key := range slices.Sorted(maps.Keys(headers)) {
		if strings.HasSuffix(strings.ToLower(key), "-subscription-userinfo") {
			if usage := parseSubscriptionUsage(strings.Join(headers[key], ";")); usage != nil {
				return usage
			}
		}
	}
	return nil
}

func parseSubscriptionUsage(header string) *SubscriptionUsage {
	var upload, download int64
	var usage SubscriptionUsage
	for field := range strings.SplitSeq(header, ";") {
		key, value, ok := strings.Cut(field, "=")
		number, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if !ok || err != nil || number < 0 {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "upload":
			upload = number
			usage.UploadBytes = &number
		case "download":
			download = number
			usage.DownloadBytes = &number
		case "total":
			usage.TotalBytes = number
		case "expire":
			usage.ExpiresAt = number
		}
	}
	usage.UsedBytes = min(upload, math.MaxInt64-download) + download
	if usage == (SubscriptionUsage{}) {
		return nil
	}
	return &usage
}

// fetchLocal 读取本地文件。
func fetchLocal(path string) ([]byte, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("%s", i18n.T("profile.fetcher.err_empty_local"))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("profile.fetcher.err_read_local"), err)
	}
	return data, nil
}
