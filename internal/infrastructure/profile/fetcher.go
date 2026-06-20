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

	"gopkg.in/yaml.v3"
)

// fetchTimeout 远程拉取超时。
const fetchTimeout = 30 * time.Second

// fetchUserAgent 远程拉取使用的 User-Agent，便于订阅端识别。
const fetchUserAgent = "mihosh/1.0"

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
	if err := yaml.Unmarshal(data, &probe); err != nil {
		return fmt.Errorf("订阅内容非合法 YAML: %w", err)
	}
	if topLevelMapping(&probe) == nil {
		return fmt.Errorf("订阅内容顶层不是配置映射（可能是错误页或纯文本），已拒绝")
	}
	return WriteRaw(p.UID, data)
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
	url = strings.TrimSpace(url)
	if url == "" {
		return nil, fmt.Errorf("订阅 URL 不能为空")
	}
	lower := strings.ToLower(url)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		// 禁止 file:// 等其它协议，防止与 local 混淆绕过。
		return nil, fmt.Errorf("仅支持 http/https 协议的订阅 URL")
	}

	client := &http.Client{Timeout: fetchTimeout}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("User-Agent", fetchUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("拉取订阅失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("订阅服务器返回非成功状态码: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取订阅响应失败: %w", err)
	}
	return data, nil
}

// fetchLocal 读取本地文件。
func fetchLocal(path string) ([]byte, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("本地配置路径不能为空")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取本地配置失败: %w", err)
	}
	return data, nil
}
