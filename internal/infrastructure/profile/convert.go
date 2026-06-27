package profile

// convert.go — v2ray base64 订阅 → Mihomo YAML 转换器。
//
// v2ray 订阅格式：HTTP 响应体为 base64 编码文本，解码后每行一条协议 URI。
// 目前支持：vmess://（最常见）。架构预留扩展点供后续添加 vless/trojan/ss。
//
// TryConvertV2Ray 为唯一入口：
//   - 非 v2ray 格式 → (nil, false, nil)；
//   - 是 v2ray 格式但解析失败 → (nil, true, err)；
//   - 成功 → (yamlBytes, true, nil)。

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// v2ray 协议前缀。
const (
	prefixVmess = "vmess://"
	prefixVless = "vless://"
)

// TryConvertV2Ray 尝试将 data 识别为 v2ray base64 订阅并转换为 Mihomo YAML。
// 返回值：
//   - yamlData: 转换后的 Mihomo YAML 字节（成功时非 nil）
//   - isV2Ray:  data 是否被识别为 v2ray 订阅格式
//   - err:      转换过程中的错误（仅 isV2Ray==true 时有意义）
func TryConvertV2Ray(data []byte) ([]byte, bool, error) {
	// 1. 尝试 base64 解码
	decoded, ok := tryBase64Decode(data)
	if !ok {
		return nil, false, nil
	}

	// 2. 按行分割，过滤空行
	lines := splitLines(decoded)
	if len(lines) == 0 {
		return nil, false, nil
	}

	// 3. 检测是否包含已知协议前缀
	if !hasKnownProtocol(lines) {
		return nil, false, nil
	}

	// 4. 逐行解析
	var proxies []map[string]any
	var skipped int
	for _, line := range lines {
		proxy, err := parseLine(line)
		if err != nil {
			skipped++
			continue
		}
		if proxy != nil {
			proxies = append(proxies, proxy)
		}
	}

	if len(proxies) == 0 {
		return nil, true, fmt.Errorf("所有节点解析失败（共 %d 行）", len(lines))
	}

	// 5. 组装为 Mihomo YAML
	yamlData, err := buildMihomoYAML(proxies)
	if err != nil {
		return nil, true, fmt.Errorf("生成 Mihomo 配置失败: %w", err)
	}

	return yamlData, true, nil
}

// SkippedCount 返回最近一次 TryConvertV2Ray 中跳过的节点数。
// （非线程安全，仅供 UI 层单次调用后读取。后续若需要可改为返回值。）
// 注意：当前设计不暴露 skipped count，错误信息已足够。

// tryBase64Decode 尝试标准 base64 和无 padding base64 解码。
// 真实订阅响应的 base64 字符串常包含换行符（76 字符折行），需先剥离所有空白。
func tryBase64Decode(data []byte) (string, bool) {
	// 剥离所有空白字符（\n \r \t 空格），兼容各种折行格式
	cleaned := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return -1 // 删除
		}
		return r
	}, string(data))
	if cleaned == "" {
		return "", false
	}

	// 尝试标准 base64（含 padding）
	decoded, err := base64.StdEncoding.DecodeString(cleaned)
	if err == nil && len(decoded) > 0 {
		return string(decoded), true
	}

	// 尝试无 padding（RawStdEncoding）
	decoded, err = base64.RawStdEncoding.DecodeString(cleaned)
	if err == nil && len(decoded) > 0 {
		return string(decoded), true
	}

	// 尝试 URL-safe base64（有些订阅用这种）
	decoded, err = base64.URLEncoding.DecodeString(cleaned)
	if err == nil && len(decoded) > 0 {
		return string(decoded), true
	}

	decoded, err = base64.RawURLEncoding.DecodeString(cleaned)
	if err == nil && len(decoded) > 0 {
		return string(decoded), true
	}

	return "", false
}

// splitLines 按换行符分割并过滤空行。
func splitLines(s string) []string {
	raw := strings.Split(s, "\n")
	var out []string
	for _, l := range raw {
		l = strings.TrimSpace(l)
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

// hasKnownProtocol 检查行中是否至少有一行以已知协议前缀开头。
func hasKnownProtocol(lines []string) bool {
	for _, l := range lines {
		if strings.HasPrefix(l, prefixVmess) || strings.HasPrefix(l, prefixVless) {
			return true
		}
		// 后续扩展：trojan://, ss://
	}
	return false
}

// parseLine 按协议前缀分发解析单行 URI。
func parseLine(line string) (map[string]any, error) {
	switch {
	case strings.HasPrefix(line, prefixVmess):
		return parseVmess(line)
	case strings.HasPrefix(line, prefixVless):
		return parseVless(line)
	// 后续扩展：
	// case strings.HasPrefix(line, "trojan://"):
	//     return parseTrojan(line)
	// case strings.HasPrefix(line, "ss://"):
	//     return parseSS(line)
	default:
		return nil, fmt.Errorf("不支持的协议: %s", line[:min(20, len(line))])
	}
}

// vmessJSON vmess:// URI 中 base64 解码后的 JSON 结构。
type vmessJSON struct {
	V    any    `json:"v"`    // 版本号（可能是 string 或 int）
	PS   string `json:"ps"`   // 备注/节点名
	Add  string `json:"add"`  // 服务器地址
	Port any    `json:"port"` // 端口（可能是 string 或 int）
	ID   string `json:"id"`   // UUID
	Aid  any    `json:"aid"`  // alterId（可能是 string 或 int）
	Scy  string `json:"scy"`  // 加密方式
	Net  string `json:"net"`  // 传输协议 (tcp/ws/grpc/h2)
	Type string `json:"type"` // 伪装类型
	Host string `json:"host"` // 主机名
	Path string `json:"path"` // 路径
	TLS  string `json:"tls"`  // tls
	SNI  string `json:"sni"`  // SNI
	ALPN string `json:"alpn"` // ALPN
}

// parseVmess 解析 vmess:// URI 为 Mihomo 代理节点配置 map。
func parseVmess(uri string) (map[string]any, error) {
	payload := strings.TrimPrefix(uri, prefixVmess)
	payload = strings.TrimSpace(payload)

	// vmess:// 后的内容也是 base64
	decoded, ok := tryBase64Decode([]byte(payload))
	if !ok {
		return nil, fmt.Errorf("vmess payload base64 解码失败")
	}

	var v vmessJSON
	if err := json.Unmarshal([]byte(decoded), &v); err != nil {
		return nil, fmt.Errorf("vmess JSON 解析失败: %w", err)
	}

	if v.Add == "" || v.ID == "" {
		return nil, fmt.Errorf("vmess 缺少必要字段 (add/id)")
	}

	port := toInt(v.Port)
	if port <= 0 || port > 65535 {
		return nil, fmt.Errorf("vmess 端口无效: %v", v.Port)
	}

	alterId := toInt(v.Aid)
	cipher := v.Scy
	if cipher == "" {
		cipher = "auto"
	}

	name := v.PS
	if name == "" {
		name = fmt.Sprintf("%s:%d", v.Add, port)
	}

	proxy := map[string]any{
		"name":    name,
		"type":    "vmess",
		"server":  v.Add,
		"port":    port,
		"uuid":    v.ID,
		"alterId": alterId,
		"cipher":  cipher,
	}

	// UDP 默认启用
	proxy["udp"] = true

	// TLS
	if strings.EqualFold(v.TLS, "tls") {
		proxy["tls"] = true
		if v.SNI != "" {
			proxy["servername"] = v.SNI
		}
		if v.ALPN != "" {
			proxy["alpn"] = strings.Split(v.ALPN, ",")
		}
	}

	// 传输层选项
	network := v.Net
	if network == "" {
		network = "tcp"
	}
	proxy["network"] = network

	switch network {
	case "ws":
		wsOpts := map[string]any{}
		if v.Path != "" {
			wsOpts["path"] = v.Path
		}
		if v.Host != "" {
			wsOpts["headers"] = map[string]any{"Host": v.Host}
		}
		if len(wsOpts) > 0 {
			proxy["ws-opts"] = wsOpts
		}

	case "grpc":
		grpcOpts := map[string]any{}
		if v.Path != "" {
			grpcOpts["grpc-service-name"] = v.Path
		}
		if len(grpcOpts) > 0 {
			proxy["grpc-opts"] = grpcOpts
		}

	case "h2":
		h2Opts := map[string]any{}
		if v.Path != "" {
			h2Opts["path"] = v.Path
		}
		if v.Host != "" {
			h2Opts["host"] = []string{v.Host}
		}
		if len(h2Opts) > 0 {
			proxy["h2-opts"] = h2Opts
		}

	case "tcp":
		// tcp + http 伪装
		if v.Type == "http" {
			proxy["network"] = "http"
			httpOpts := map[string]any{}
			if v.Path != "" {
				httpOpts["path"] = []string{v.Path}
			}
			if v.Host != "" {
				httpOpts["headers"] = map[string]any{"Host": []string{v.Host}}
			}
			if len(httpOpts) > 0 {
				proxy["http-opts"] = httpOpts
			}
		}
	}

	return proxy, nil
}

// toInt 将可能是 string 或 float64/int 的值转为 int。
func toInt(v any) int {
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case string:
		n, _ := strconv.Atoi(val)
		return n
	case json.Number:
		n, _ := val.Int64()
		return int(n)
	default:
		return 0
	}
}

// buildMihomoYAML 从解析好的代理节点列表组装完整的 Mihomo YAML 配置。
func buildMihomoYAML(proxies []map[string]any) ([]byte, error) {
	// 收集所有节点名称
	var names []string
	for _, p := range proxies {
		if name, ok := p["name"].(string); ok {
			names = append(names, name)
		}
	}

	// 构建 proxy-groups：auto（自动选择）+ select（手动选择）
	proxyGroups := []map[string]any{
		{
			"name":     "auto",
			"type":     "url-test",
			"proxies":  names,
			"url":      "https://www.gstatic.com/generate_204",
			"interval": 300,
		},
		{
			"name":    "select",
			"type":    "select",
			"proxies": append([]string{"auto"}, names...),
		},
	}

	// 基本规则
	rules := []string{
		"MATCH,select",
	}

	config := map[string]any{
		"proxies":      proxies,
		"proxy-groups": proxyGroups,
		"rules":        rules,
	}

	data, err := marshalYAML2Spaces(config)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// parseVless 解析 vless:// URI 为 Mihomo 代理节点配置 map。
func parseVless(uri string) (map[string]any, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("解析 vless URI 失败: %w", err)
	}

	if u.Scheme != "vless" {
		return nil, fmt.Errorf("非 vless 协议")
	}

	uuid := u.User.Username()
	if uuid == "" {
		return nil, fmt.Errorf("vless 缺少 uuid")
	}

	server := u.Hostname()
	portStr := u.Port()
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return nil, fmt.Errorf("vless 端口无效: %s", portStr)
	}

	name := u.Fragment
	if name == "" {
		name = fmt.Sprintf("%s:%d", server, port)
	}

	q := u.Query()
	network := q.Get("type")
	if network == "" {
		network = "tcp"
	}

	proxy := map[string]any{
		"name":   name,
		"type":   "vless",
		"server": server,
		"port":   port,
		"uuid":   uuid,
		"udp":    true, // UDP 默认开启
	}

	// 安全相关 (tls / reality)
	security := q.Get("security")
	if security == "tls" || security == "reality" {
		proxy["tls"] = true

		sni := q.Get("sni")
		if sni != "" {
			proxy["servername"] = sni
		}

		fp := q.Get("fp")
		if fp != "" {
			proxy["client-fingerprint"] = fp
		}

		alpn := q.Get("alpn")
		if alpn != "" {
			proxy["alpn"] = strings.Split(alpn, ",")
		}

		if security == "reality" {
			realityOpts := map[string]any{}
			pbk := q.Get("pbk")
			if pbk != "" {
				realityOpts["public-key"] = pbk
			}
			sid := q.Get("sid")
			if sid != "" {
				realityOpts["short-id"] = sid
			}
			if len(realityOpts) > 0 {
				proxy["reality-opts"] = realityOpts
			}
		}
	}

	// flow
	flow := q.Get("flow")
	if flow != "" {
		proxy["flow"] = flow
	}

	// 传输层
	proxy["network"] = network
	switch network {
	case "ws":
		wsOpts := map[string]any{}
		path := q.Get("path")
		if path != "" {
			wsOpts["path"] = path
		}
		host := q.Get("host")
		if host != "" {
			wsOpts["headers"] = map[string]any{"Host": host}
		}
		if len(wsOpts) > 0 {
			proxy["ws-opts"] = wsOpts
		}
	case "grpc":
		grpcOpts := map[string]any{}
		serviceName := q.Get("serviceName")
		if serviceName != "" {
			grpcOpts["grpc-service-name"] = serviceName
		}
		if len(grpcOpts) > 0 {
			proxy["grpc-opts"] = grpcOpts
		}
	case "h2":
		h2Opts := map[string]any{}
		path := q.Get("path")
		if path != "" {
			h2Opts["path"] = path
		}
		host := q.Get("host")
		if host != "" {
			h2Opts["host"] = []string{host}
		}
		if len(h2Opts) > 0 {
			proxy["h2-opts"] = h2Opts
		}
	}

	return proxy, nil
}
