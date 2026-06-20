package profile

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// makeVmessURI 构造测试用 vmess:// URI。
func makeVmessURI(v vmessJSON) string {
	data, _ := json.Marshal(v)
	return prefixVmess + base64.StdEncoding.EncodeToString(data)
}

// encodeSubscription 将多行 URI 编码为 base64 订阅文本。
func encodeSubscription(lines ...string) []byte {
	content := strings.Join(lines, "\n")
	return []byte(base64.StdEncoding.EncodeToString([]byte(content)))
}

func TestTryConvertV2Ray_StandardVmess(t *testing.T) {
	uri := makeVmessURI(vmessJSON{
		V:   "2",
		PS:  "测试节点",
		Add: "example.com",
		Port: 443,
		ID:  "12345678-abcd-1234-abcd-1234567890ab",
		Aid: 0,
		Net: "tcp",
		TLS: "tls",
		SNI: "example.com",
	})
	sub := encodeSubscription(uri)

	yamlData, isV2Ray, err := TryConvertV2Ray(sub)
	require.True(t, isV2Ray, "应识别为 v2ray 订阅")
	require.NoError(t, err)
	require.NotEmpty(t, yamlData)

	// 解析结果校验
	var config map[string]any
	require.NoError(t, yaml.Unmarshal(yamlData, &config))
	proxies, ok := config["proxies"].([]any)
	require.True(t, ok, "应包含 proxies 键")
	require.Len(t, proxies, 1)

	proxy := proxies[0].(map[string]any)
	assert.Equal(t, "测试节点", proxy["name"])
	assert.Equal(t, "vmess", proxy["type"])
	assert.Equal(t, "example.com", proxy["server"])
	assert.Equal(t, 443, proxy["port"])
	assert.Equal(t, "12345678-abcd-1234-abcd-1234567890ab", proxy["uuid"])
	assert.Equal(t, true, proxy["tls"])
	assert.Equal(t, "example.com", proxy["servername"])
}

func TestTryConvertV2Ray_WsTransport(t *testing.T) {
	uri := makeVmessURI(vmessJSON{
		V:    "2",
		PS:   "WS节点",
		Add:  "ws.example.com",
		Port: 8080,
		ID:   "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		Aid:  0,
		Net:  "ws",
		Host: "cdn.example.com",
		Path: "/ws-path",
	})
	sub := encodeSubscription(uri)

	yamlData, isV2Ray, err := TryConvertV2Ray(sub)
	require.True(t, isV2Ray)
	require.NoError(t, err)

	var config map[string]any
	require.NoError(t, yaml.Unmarshal(yamlData, &config))
	proxies := config["proxies"].([]any)
	proxy := proxies[0].(map[string]any)

	assert.Equal(t, "ws", proxy["network"])
	wsOpts, ok := proxy["ws-opts"].(map[string]any)
	require.True(t, ok, "应包含 ws-opts")
	assert.Equal(t, "/ws-path", wsOpts["path"])
	headers, ok := wsOpts["headers"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "cdn.example.com", headers["Host"])
}

func TestTryConvertV2Ray_GrpcTransport(t *testing.T) {
	uri := makeVmessURI(vmessJSON{
		V:    "2",
		PS:   "gRPC节点",
		Add:  "grpc.example.com",
		Port: 443,
		ID:   "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		Net:  "grpc",
		Path: "my-grpc-service",
		TLS:  "tls",
	})
	sub := encodeSubscription(uri)

	yamlData, isV2Ray, err := TryConvertV2Ray(sub)
	require.True(t, isV2Ray)
	require.NoError(t, err)

	var config map[string]any
	require.NoError(t, yaml.Unmarshal(yamlData, &config))
	proxies := config["proxies"].([]any)
	proxy := proxies[0].(map[string]any)

	assert.Equal(t, "grpc", proxy["network"])
	grpcOpts, ok := proxy["grpc-opts"].(map[string]any)
	require.True(t, ok, "应包含 grpc-opts")
	assert.Equal(t, "my-grpc-service", grpcOpts["grpc-service-name"])
}

func TestTryConvertV2Ray_H2Transport(t *testing.T) {
	uri := makeVmessURI(vmessJSON{
		V:    "2",
		PS:   "H2节点",
		Add:  "h2.example.com",
		Port: 443,
		ID:   "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		Net:  "h2",
		Host: "h2.example.com",
		Path: "/h2-path",
		TLS:  "tls",
	})
	sub := encodeSubscription(uri)

	yamlData, isV2Ray, err := TryConvertV2Ray(sub)
	require.True(t, isV2Ray)
	require.NoError(t, err)

	var config map[string]any
	require.NoError(t, yaml.Unmarshal(yamlData, &config))
	proxies := config["proxies"].([]any)
	proxy := proxies[0].(map[string]any)

	assert.Equal(t, "h2", proxy["network"])
	h2Opts, ok := proxy["h2-opts"].(map[string]any)
	require.True(t, ok, "应包含 h2-opts")
	assert.Equal(t, "/h2-path", h2Opts["path"])
	hosts, ok := h2Opts["host"].([]any)
	require.True(t, ok)
	assert.Contains(t, hosts, "h2.example.com")
}

func TestTryConvertV2Ray_PortAsString(t *testing.T) {
	// 部分机场返回 port 为 string 类型
	uri := makeVmessURI(vmessJSON{
		V:    "2",
		PS:   "StringPort",
		Add:  "example.com",
		Port: "8443",
		ID:   "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		Net:  "tcp",
	})
	sub := encodeSubscription(uri)

	yamlData, isV2Ray, err := TryConvertV2Ray(sub)
	require.True(t, isV2Ray)
	require.NoError(t, err)

	var config map[string]any
	require.NoError(t, yaml.Unmarshal(yamlData, &config))
	proxies := config["proxies"].([]any)
	proxy := proxies[0].(map[string]any)
	assert.Equal(t, 8443, proxy["port"])
}

func TestTryConvertV2Ray_EmptyData(t *testing.T) {
	_, isV2Ray, _ := TryConvertV2Ray([]byte(""))
	assert.False(t, isV2Ray, "空内容不应识别为 v2ray")
}

func TestTryConvertV2Ray_PlainYAML(t *testing.T) {
	// 普通 YAML 不应被识别为 v2ray
	yamlContent := []byte("mode: rule\nproxies: []\n")
	_, isV2Ray, _ := TryConvertV2Ray(yamlContent)
	assert.False(t, isV2Ray, "普通 YAML 不应识别为 v2ray")
}

func TestTryConvertV2Ray_Base64ButNoProtocol(t *testing.T) {
	// base64 编码的普通文本，解码后没有协议前缀
	plain := base64.StdEncoding.EncodeToString([]byte("hello world\nfoo bar\n"))
	_, isV2Ray, _ := TryConvertV2Ray([]byte(plain))
	assert.False(t, isV2Ray, "无协议前缀不应识别为 v2ray")
}

func TestTryConvertV2Ray_MixedProtocols(t *testing.T) {
	vmessURI := makeVmessURI(vmessJSON{
		V:    "2",
		PS:   "有效节点",
		Add:  "example.com",
		Port: 443,
		ID:   "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		Net:  "tcp",
	})
	unknownLine := "vless://some-unknown-content"
	sub := encodeSubscription(vmessURI, unknownLine)

	yamlData, isV2Ray, err := TryConvertV2Ray(sub)
	require.True(t, isV2Ray, "包含 vmess 应识别为 v2ray")
	require.NoError(t, err, "有效节点应成功（跳过不支持的协议）")

	var config map[string]any
	require.NoError(t, yaml.Unmarshal(yamlData, &config))
	proxies := config["proxies"].([]any)
	assert.Len(t, proxies, 1, "仅 vmess 节点应被解析")
}

func TestTryConvertV2Ray_AllInvalid(t *testing.T) {
	// 全部行解析失败
	line1 := "vmess://invalid-base64!"
	sub := encodeSubscription(line1)

	_, isV2Ray, err := TryConvertV2Ray(sub)
	assert.True(t, isV2Ray, "包含 vmess 前缀应识别为 v2ray")
	assert.Error(t, err, "全部解析失败应报错")
}

func TestTryConvertV2Ray_NoPadding(t *testing.T) {
	// 无 padding 的 base64（部分订阅源使用）
	uri := makeVmessURI(vmessJSON{
		V:    "2",
		PS:   "NoPad",
		Add:  "example.com",
		Port: 443,
		ID:   "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		Net:  "tcp",
	})
	content := uri
	encoded := base64.RawStdEncoding.EncodeToString([]byte(content))

	yamlData, isV2Ray, err := TryConvertV2Ray([]byte(encoded))
	require.True(t, isV2Ray)
	require.NoError(t, err)
	require.NotEmpty(t, yamlData)
}

func TestTryConvertV2Ray_MultipleNodes(t *testing.T) {
	uri1 := makeVmessURI(vmessJSON{
		V: "2", PS: "节点1", Add: "a.com", Port: 443,
		ID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", Net: "tcp",
	})
	uri2 := makeVmessURI(vmessJSON{
		V: "2", PS: "节点2", Add: "b.com", Port: 8080,
		ID: "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", Net: "ws", Path: "/ws",
	})
	sub := encodeSubscription(uri1, uri2)

	yamlData, isV2Ray, err := TryConvertV2Ray(sub)
	require.True(t, isV2Ray)
	require.NoError(t, err)

	var config map[string]any
	require.NoError(t, yaml.Unmarshal(yamlData, &config))

	proxies := config["proxies"].([]any)
	assert.Len(t, proxies, 2)

	// proxy-groups 应包含所有节点
	groups := config["proxy-groups"].([]any)
	require.GreaterOrEqual(t, len(groups), 2, "应有 auto + select 两个代理组")
}

func TestTryConvertV2Ray_ProxyGroups(t *testing.T) {
	uri := makeVmessURI(vmessJSON{
		V: "2", PS: "MyNode", Add: "example.com", Port: 443,
		ID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", Net: "tcp",
	})
	sub := encodeSubscription(uri)

	yamlData, _, _ := TryConvertV2Ray(sub)
	var config map[string]any
	require.NoError(t, yaml.Unmarshal(yamlData, &config))

	groups := config["proxy-groups"].([]any)

	// auto 组
	autoGroup := groups[0].(map[string]any)
	assert.Equal(t, "auto", autoGroup["name"])
	assert.Equal(t, "url-test", autoGroup["type"])

	// select 组应包含 auto + 所有节点
	selectGroup := groups[1].(map[string]any)
	assert.Equal(t, "select", selectGroup["name"])
	selectProxies := selectGroup["proxies"].([]any)
	assert.Contains(t, selectProxies, "auto")
	assert.Contains(t, selectProxies, "MyNode")
}

func TestTryConvertV2Ray_TcpHttpObfs(t *testing.T) {
	uri := makeVmessURI(vmessJSON{
		V: "2", PS: "HTTP伪装", Add: "example.com", Port: 80,
		ID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		Net: "tcp", Type: "http", Host: "cdn.example.com", Path: "/path",
	})
	sub := encodeSubscription(uri)

	yamlData, isV2Ray, err := TryConvertV2Ray(sub)
	require.True(t, isV2Ray)
	require.NoError(t, err)

	var config map[string]any
	require.NoError(t, yaml.Unmarshal(yamlData, &config))
	proxies := config["proxies"].([]any)
	proxy := proxies[0].(map[string]any)

	assert.Equal(t, "http", proxy["network"], "tcp+http 应映射为 http network")
	httpOpts, ok := proxy["http-opts"].(map[string]any)
	require.True(t, ok)
	paths := httpOpts["path"].([]any)
	assert.Contains(t, paths, "/path")
}

func TestParseVmess_MissingFields(t *testing.T) {
	// 缺 add
	uri := makeVmessURI(vmessJSON{V: "2", PS: "NoAdd", Port: 443, ID: "uuid"})
	_, err := parseVmess(uri)
	assert.Error(t, err, "缺少 add 应报错")

	// 缺 id
	uri = makeVmessURI(vmessJSON{V: "2", PS: "NoID", Add: "x.com", Port: 443})
	_, err = parseVmess(uri)
	assert.Error(t, err, "缺少 id 应报错")
}

func TestParseVmess_InvalidPort(t *testing.T) {
	uri := makeVmessURI(vmessJSON{V: "2", Add: "x.com", Port: 0, ID: "uuid"})
	_, err := parseVmess(uri)
	assert.Error(t, err, "端口 0 应报错")

	uri = makeVmessURI(vmessJSON{V: "2", Add: "x.com", Port: 99999, ID: "uuid"})
	_, err = parseVmess(uri)
	assert.Error(t, err, "端口超范围应报错")
}

func TestToInt(t *testing.T) {
	assert.Equal(t, 443, toInt(float64(443)))
	assert.Equal(t, 443, toInt(443))
	assert.Equal(t, 443, toInt("443"))
	assert.Equal(t, 0, toInt(nil))
	assert.Equal(t, 0, toInt("abc"))
}

func TestTryConvertV2Ray_Base64WithLineBreaks(t *testing.T) {
	// 模拟真实订阅：base64 内容包含 \r\n 换行（HTTP 76字符折行）
	uri := makeVmessURI(vmessJSON{
		V: "2", PS: "LineBreak节点", Add: "example.com", Port: 443,
		ID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", Net: "tcp",
	})
	raw := []byte(uri)
	encoded := base64.StdEncoding.EncodeToString(raw)

	// 手动插入换行符模拟 76 字符折行
	var wrapped strings.Builder
	for i, c := range encoded {
		wrapped.WriteByte(byte(c))
		if (i+1)%76 == 0 {
			wrapped.WriteString("\r\n")
		}
	}

	yamlData, isV2Ray, err := TryConvertV2Ray([]byte(wrapped.String()))
	require.True(t, isV2Ray, "含换行的 base64 应被识别")
	require.NoError(t, err)
	require.NotEmpty(t, yamlData)
}

func TestParseVless_Reality(t *testing.T) {
	uri := "vless://a1cd5cea-899e-4502-82fa-85dbf4dc4c88@1sg001.344211.cc:8443?type=tcp&encryption=none&host=&path=&headerType=none&quicSecurity=none&serviceName=&mode=gun&security=reality&flow=xtls-rprx-vision&fp=chrome&sni=www.python.org&pbk=2a0ONLRiBeHJdr9qCruq5tPVf8_3c4fmZsg7YQorFSE&sid=04d59340#剩余流量：298.81 GB"
	proxy, err := parseVless(uri)
	require.NoError(t, err)

	assert.Equal(t, "剩余流量：298.81 GB", proxy["name"])
	assert.Equal(t, "vless", proxy["type"])
	assert.Equal(t, "1sg001.344211.cc", proxy["server"])
	assert.Equal(t, 8443, proxy["port"])
	assert.Equal(t, "a1cd5cea-899e-4502-82fa-85dbf4dc4c88", proxy["uuid"])
	assert.Equal(t, "tcp", proxy["network"])
	assert.Equal(t, true, proxy["tls"])
	assert.Equal(t, "xtls-rprx-vision", proxy["flow"])
	assert.Equal(t, "chrome", proxy["client-fingerprint"])
	assert.Equal(t, "www.python.org", proxy["servername"])

	realityOpts, ok := proxy["reality-opts"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "2a0ONLRiBeHJdr9qCruq5tPVf8_3c4fmZsg7YQorFSE", realityOpts["public-key"])
	assert.Equal(t, "04d59340", realityOpts["short-id"])
}

func TestTryConvertV2Ray_Vless(t *testing.T) {
	// Add test using base64 encoded vless subscription
	uri := "vless://a1cd5cea-899e-4502-82fa-85dbf4dc4c88@1sg001.344211.cc:8443?type=tcp&security=reality#my-vless"
	sub := encodeSubscription(uri)

	yamlData, isV2Ray, err := TryConvertV2Ray(sub)
	require.True(t, isV2Ray)
	require.NoError(t, err)

	var config map[string]any
	require.NoError(t, yaml.Unmarshal(yamlData, &config))
	proxies := config["proxies"].([]any)
	assert.Len(t, proxies, 1)

	proxy := proxies[0].(map[string]any)
	assert.Equal(t, "vless", proxy["type"])
	assert.Equal(t, "my-vless", proxy["name"])
}
