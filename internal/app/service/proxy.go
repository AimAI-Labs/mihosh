package service

import (
	"encoding/json"
	"sync"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
)

// ProxyService 代理管理服务
type ProxyService struct {
	client  *api.Client
	testURL string
	timeout int
}

// NewProxyService 创建代理服务
func NewProxyService(client *api.Client, testURL string, timeout int) *ProxyService {
	return &ProxyService{
		client:  client,
		testURL: testURL,
		timeout: timeout,
	}
}

// GetGroups 获取所有策略组
func (s *ProxyService) GetGroups() (map[string]model.Group, []string, error) {
	return s.client.GetGroups()
}

// GetProxies 获取所有代理
func (s *ProxyService) GetProxies() (map[string]model.Proxy, error) {
	return s.client.GetProxies()
}

// SelectProxy 选择代理节点
func (s *ProxyService) SelectProxy(group, proxy string) error {
	return s.client.SelectProxy(group, proxy)
}

// TestProxyDelay 测试单个代理延迟
func (s *ProxyService) TestProxyDelay(name string) (int, error) {
	return s.client.TestProxyDelay(name, s.testURL, s.timeout)
}

// TestGroupDelay 测试策略组内所有节点延迟
func (s *ProxyService) TestGroupDelay(group string) error {
	return s.client.TestGroupDelay(group, s.testURL, s.timeout)
}

// TestAllProxies 批量测试代理延迟（返回每个代理的测试结果）
func (s *ProxyService) TestAllProxies(proxies []string) map[string]int {
	results := make(map[string]int)
	var mu sync.Mutex
	var wg sync.WaitGroup

	sem := make(chan struct{}, model.TestConcurrency)

	for _, p := range proxies {
		wg.Add(1)
		go func(proxy string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			delay, err := s.TestProxyDelay(proxy)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				results[proxy] = -1 // 失败时记录为 -1
			} else {
				results[proxy] = delay
			}
		}(p)
	}

	wg.Wait()
	return results
}

// GetNodeChain 获取当前活跃节点链路
func (s *ProxyService) GetNodeChain() ([]string, error) {
	var chain []string
	current := "GLOBAL"

	for {
		proxy, err := s.client.GetProxy(current)
		if err != nil {
			// 如果没有 GLOBAL，尝试找常见的 Proxy 组
			if current == "GLOBAL" {
				current = "Proxy"
				continue
			}
			return nil, err
		}

		// 检查是否已经存在（防止死循环）
		for _, name := range chain {
			if name == current {
				return chain, nil
			}
		}

		chain = append(chain, current)

		// 如果节点没有 Now 字段，说明是叶子节点（代理节点）
		if proxy.Now == "" {
			break
		}

		current = proxy.Now
	}

	return chain, nil
}

// GetIPInfo 获取当前出口 IP 信息
func (s *ProxyService) GetIPInfo(proxyAddr string) (*model.IPInfo, error) {
	httpClient, err := s.client.NewHTTPClientWithProxy(proxyAddr)
	if err != nil {
		return nil, err
	}

	// 使用 ip-api.com 获取详细信息
	// fields=61439 包含：status, message, country, countryCode, region, regionName, city, zip, lat, lon, timezone, isp, org, as, query
	resp, err := httpClient.Get(model.IPApiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var info model.IPInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	// 字段校准：将 ip-api.com 的字段映射到通用字段名
	if info.IP == "" && info.Query != "" {
		info.IP = info.Query
	}
	if info.CountryCode == "" && info.CountryCodeApi != "" {
		info.CountryCode = info.CountryCodeApi
	}
	if info.Organization == "" && info.Org != "" {
		info.Organization = info.Org
	}
	if info.Latitude == 0 && info.Lat != 0 {
		info.Latitude = info.Lat
	}
	if info.Longitude == 0 && info.Lon != 0 {
		info.Longitude = info.Lon
	}

	return &info, nil
}
