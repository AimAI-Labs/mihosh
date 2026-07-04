package profile

// fetcher.go — 订阅原始配置的拉取/读取。
//
// SourceRemote: 独立 http.Client（30s 超时，不走 mihomo api.Client），UA: mihosh/<ver>；
//               token 仅支持 URL 内嵌（?token= 或 basic auth in URL）；
//               响应按 YAML 解析校验，非法则拒绝并保留旧 raw。
// SourceLocal:  直接读文件，同样做 YAML 校验。
//
// 并发：v1 仅单个更新；批量更新（若后续支持）由调用方用 semaphore 限制。

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"gopkg.in/yaml.v3"
)

// fetchTimeout 远程拉取超时。
const fetchTimeout = 30 * time.Second

// fetchUserAgent 远程拉取使用的 User-Agent，伪装成 clash-verge 以获取订阅后端的完整 Clash 规则。
const fetchUserAgent = "clash-verge/v2.4.5"

// Fetch 拉取订阅原始配置，写回 raw.yaml。
//   - remote: HTTP GET，状态码非 2xx 视为失败；
//   - local: 读取文件。
// 写入前对内容做 YAML 合法性校验，并要求顶层为 mapping（mihomo 配置必须是键值结构），
// 非法则返回错误且不覆盖已有 raw。这能拦截订阅端返回的 HTML 错误页等非配置响应。
func Fetch(p Profile) error {
	data, err := fetchBytes(p.Source)
	if err != nil {
		return err
	}
	// 校验 YAML 合法性 + 顶层必须为 mapping。
	var probe yaml.Node
	yamlErr := yaml.Unmarshal(data, &probe)
	if yamlErr == nil && topLevelMapping(&probe) != nil {
		// 合法 Mihomo YAML，直接写入
		return WriteRaw(p.UID, data)
	}

	// 非合法 Mihomo YAML，尝试作为 v2ray base64 订阅解析
	converted, isV2Ray, convErr := TryConvertV2Ray(data)
	if isV2Ray && convErr == nil {
		return WriteRaw(p.UID, converted)
	}
	if isV2Ray {
		return fmt.Errorf("v2ray 订阅解析失败: %w", convErr)
	}

	// 两者皆非，返回原始 YAML 错误
	if yamlErr != nil {
		return fmt.Errorf("订阅内容非合法 YAML: %w", yamlErr)
	}
	return fmt.Errorf("订阅内容顶层不是配置映射（可能是错误页或纯文本），已拒绝")
}

// fetchBytes 按 Source 类型获取原始字节。
func fetchBytes(src SubSource) ([]byte, error) {
	switch src.Kind {
	case SourceLocal:
		return fetchLocal(src.Path)
	default:
		return fetchRemote(src.URL)
	}
}

// fetchRemote 拉取远程 URL。
func fetchRemote(url string) ([]byte, error) {
	subURL := strings.TrimSpace(url)
	if subURL == "" {
		return nil, fmt.Errorf("%s", i18n.T("profile.fetcher.err_empty_url"))
	}

	if !strings.HasPrefix(subURL, "http://") && !strings.HasPrefix(subURL, "https://") {
		// 禁止 file:// 等其它协议，防止与 local 混淆绕过。
		return nil, fmt.Errorf("%s", i18n.T("profile.fetcher.err_only_http"))
	}

	client := &http.Client{Timeout: fetchTimeout}
	req, err := http.NewRequest(http.MethodGet, subURL, nil)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("profile.fetcher.err_build_req"), err)
	}
	req.Header.Set("User-Agent", fetchUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("profile.fetcher.err_fetch"), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf(i18n.T("profile.fetcher.err_status_code"), resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("profile.fetcher.err_read_resp"), err)
	}
	return body, nil
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
