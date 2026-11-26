package service

import (
	"context"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/domain/port"
)

// ProxyService 代理管理服务
type ProxyService struct {
	client  port.ProxyAPI
	testURL string
	timeout int
}

// NewProxyService 创建代理服务
func NewProxyService(client port.ProxyAPI, testURL string, timeout int) *ProxyService {
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

// GetIPInfo 获取当前出口 IP 信息
func (s *ProxyService) GetIPInfo(proxyAddr string) (*model.IPInfo, error) {
	ctx := context.Background()
	if s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(s.timeout)*time.Millisecond)
		defer cancel()
	}
	return FetchCurrentIPInfo(ctx, proxyAddr)
}
