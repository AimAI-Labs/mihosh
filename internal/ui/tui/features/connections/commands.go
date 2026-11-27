package connections

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
)

func FetchConnections(client *service.Client) tea.Cmd {
	if client == nil {
		return nil
	}
	return messages.CoreRequest(client, func(client *service.Client) tea.Msg {
		conns, err := client.GetConnections()
		if err != nil {
			return messages.ErrMsg{Err: err}
		}
		return messages.ConnectionsMsg{Resp: conns}
	})
}

// CloseConnection 关闭单个连接
func CloseConnection(client *service.Client, id string) tea.Cmd {
	if client == nil {
		return nil
	}
	return messages.CoreRequest(client, func(client *service.Client) tea.Msg {
		if err := client.CloseConnection(id); err != nil {
			return messages.ErrMsg{Err: err}
		}
		return messages.ConnectionClosedMsg{ID: id}
	})
}

// CloseAllConnections 关闭所有连接
func CloseAllConnections(client *service.Client) tea.Cmd {
	if client == nil {
		return nil
	}
	return messages.CoreRequest(client, func(client *service.Client) tea.Msg {
		if err := client.CloseAllConnections(); err != nil {
			return messages.ErrMsg{Err: err}
		}
		return messages.AllConnectionsClosedMsg{}
	})
}

// CloseConnectionsByIDs 按活跃连接 ID 批量关闭筛选命中的连接（限制并发为 model.TestConcurrency）
func CloseConnectionsByIDs(client *service.Client, ids []string) tea.Cmd {
	if client == nil {
		return nil
	}
	unique := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return messages.CoreRequest(client, func(client *service.Client) tea.Msg {
		if len(unique) == 0 {
			return messages.FilteredConnectionsClosedMsg{}
		}
		sem := make(chan struct{}, model.TestConcurrency)
		var (
			wg     sync.WaitGroup
			mu     sync.Mutex
			closed int
			failed int
		)
		for _, id := range unique {
			sem <- struct{}{}
			wg.Add(1)
			go func(connID string) {
				defer wg.Done()
				defer func() { <-sem }()
				err := client.CloseConnection(connID)
				mu.Lock()
				if err != nil {
					failed++
				} else {
					closed++
				}
				mu.Unlock()
			}(id)
		}
		wg.Wait()
		return messages.FilteredConnectionsClosedMsg{Closed: closed, Failed: failed}
	})
}

// FetchIPInfo 获取IP地理位置信息
func FetchIPInfo(ip string) tea.Cmd {
	return fetchIPInfoWithID(0, ip)
}

func fetchIPInfoWithID(requestID uint64, ip string) tea.Cmd {
	return func() tea.Msg {
		if ip == "" {
			return messages.IPInfoMsg{RequestID: requestID, Info: nil, Err: nil}
		}

		client := &http.Client{Timeout: 5 * time.Second}
		apiURL := fmt.Sprintf("https://service.ip.sb/geoip/%s", ip)

		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return messages.IPInfoMsg{RequestID: requestID, Info: nil, Err: err}
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		req.Header.Set("Accept", "*/*")

		resp, err := client.Do(req)
		if err != nil {
			return messages.IPInfoMsg{RequestID: requestID, Info: nil, Err: err}
		}
		defer func() { _ = resp.Body.Close() }()

		var info model.IPInfo
		if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
			return messages.IPInfoMsg{RequestID: requestID, Info: nil, Err: err}
		}

		return messages.IPInfoMsg{RequestID: requestID, Info: &info, Err: nil}
	}
}

// TestSiteDelay 测试单个网站延迟（通过代理）
func TestSiteDelay(ctx context.Context, proxyAddr string, siteName string, siteURL string, timeout int) tea.Cmd {
	return func() tea.Msg {
		// 创建带代理的HTTP客户端
		client := &http.Client{
			Timeout: time.Duration(timeout) * time.Millisecond,
		}

		// 如果配置了代理地址，使用代理
		if proxyAddr != "" {
			proxyURL, err := ParseProxyURL(proxyAddr)
			if err == nil {
				client.Transport = &http.Transport{
					Proxy: http.ProxyURL(proxyURL),
				}
			}
		}

		start := time.Now()
		req, err := http.NewRequestWithContext(ctx, "GET", siteURL, nil)
		if err != nil {
			return messages.SiteTestMsg{Name: siteName, Delay: 0, Err: err}
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

		resp, err := client.Do(req)
		if err != nil {
			return messages.SiteTestMsg{Name: siteName, Delay: 0, Err: err}
		}
		defer func() { _ = resp.Body.Close() }()

		delay := int(time.Since(start).Milliseconds())
		return messages.SiteTestMsg{Name: siteName, Delay: delay, Err: nil}
	}
}

// ParseProxyURL 解析代理地址
func ParseProxyURL(addr string) (*url.URL, error) {
	// 确保有协议前缀
	if !hasScheme(addr) {
		addr = "http://" + addr
	}
	return url.Parse(addr)
}

// hasScheme 检查地址是否有协议前缀
func hasScheme(addr string) bool {
	return (len(addr) >= 7 && addr[:7] == "http://") ||
		(len(addr) >= 8 && addr[:8] == "https://") ||
		(len(addr) >= 9 && addr[:9] == "socks5://")
}

// ConvertToConnectionsResponse 将 service.ConnectionsData 转换为 model.ConnectionsResponse
func ConvertToConnectionsResponse(data service.ConnectionsData) *model.ConnectionsResponse {
	connections := make([]model.Connection, len(data.Connections))
	for i, conn := range data.Connections {
		connections[i] = model.Connection{
			ID:            conn.ID,
			Upload:        conn.Upload,
			Download:      conn.Download,
			Start:         conn.Start,
			Chains:        conn.Chains,
			Rule:          conn.Rule,
			RulePayload:   conn.RulePayload,
			DownloadSpeed: conn.DownloadSpeed,
			UploadSpeed:   conn.UploadSpeed,
			Metadata: model.Metadata{
				Network:         conn.Metadata.Network,
				Type:            conn.Metadata.Type,
				SourceIP:        conn.Metadata.SourceIP,
				DestinationIP:   conn.Metadata.DestinationIP,
				SourcePort:      conn.Metadata.SourcePort,
				DestinationPort: conn.Metadata.DestinationPort,
				Host:            conn.Metadata.Host,
				Process:         conn.Metadata.Process,
				ProcessPath:     conn.Metadata.ProcessPath,
			},
		}
	}
	return &model.ConnectionsResponse{
		DownloadTotal: data.DownloadTotal,
		UploadTotal:   data.UploadTotal,
		Connections:   connections,
	}
}
