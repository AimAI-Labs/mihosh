package api

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
)

// GetProxies 获取所有代理信息
func (c *Client) GetProxies() (map[string]model.Proxy, error) {
	data, err := c.DoRequest("GET", "/proxies", nil)
	if err != nil {
		return nil, err
	}

	var resp model.ProxiesResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	return resp.Proxies, nil
}

// GetProxy 获取指定代理信息
func (c *Client) GetProxy(name string) (*model.Proxy, error) {
	data, err := c.DoRequest("GET", "/proxies/"+url.PathEscape(name), nil)
	if err != nil {
		return nil, err
	}

	var proxy model.Proxy
	if err := json.Unmarshal(data, &proxy); err != nil {
		return nil, err
	}

	return &proxy, nil
}

// SelectProxy 选择代理节点
func (c *Client) SelectProxy(group, proxy string) error {
	req := model.SelectProxyRequest{Name: proxy}
	_, err := c.DoRequest("PUT", "/proxies/"+url.PathEscape(group), req)
	return err
}

// TestProxyDelay 测试单个代理延迟
func (c *Client) TestProxyDelay(name, testURL string, timeout int) (int, error) {
	path := fmt.Sprintf("/proxies/%s/delay?url=%s&timeout=%d",
		url.PathEscape(name), url.QueryEscape(testURL), timeout)
	data, err := c.DoRequest("GET", path, nil)
	if err != nil {
		return 0, err
	}

	var resp model.DelayTestResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, err
	}

	return resp.Delay, nil
}

// TestGroupDelay 测试策略组内所有节点延迟
func (c *Client) TestGroupDelay(group, testURL string, timeout int) error {
	path := fmt.Sprintf("/proxies/%s/delay?url=%s&timeout=%d",
		url.PathEscape(group), url.QueryEscape(testURL), timeout)
	_, err := c.DoRequest("GET", path, nil)
	return err
}

// GetGroups 获取所有策略组，返回策略组map和按配置文件顺序排列的组名列表
func (c *Client) GetGroups() (map[string]model.Group, []string, error) {
	proxies, err := c.GetProxies()
	if err != nil {
		return nil, nil, err
	}

	groups := make(map[string]model.Group)
	for name, proxy := range proxies {
		// 策略组有 all 字段
		if len(proxy.All) > 0 {
			groups[name] = model.Group{
				Name:    name,
				Type:    proxy.Type,
				Now:     proxy.Now,
				All:     proxy.All,
				History: proxy.History,
			}
		}
	}

	// 利用 GLOBAL proxy 的 all 字段获取配置文件中的原始顺序
	var orderedNames []string
	if global, ok := proxies["GLOBAL"]; ok {
		for _, name := range global.All {
			if _, isGroup := groups[name]; isGroup {
				orderedNames = append(orderedNames, name)
			}
		}
	}

	// 兜底：如果没有 GLOBAL 或顺序不完整，补充遗漏的组
	if len(orderedNames) != len(groups) {
		existing := make(map[string]bool, len(orderedNames))
		for _, name := range orderedNames {
			existing[name] = true
		}
		for name := range groups {
			if !existing[name] {
				orderedNames = append(orderedNames, name)
			}
		}
	}

	return groups, orderedNames, nil
}

// GetConnections 获取连接信息
func (c *Client) GetConnections() (*model.ConnectionsResponse, error) {
	data, err := c.DoRequest("GET", "/connections", nil)
	if err != nil {
		return nil, err
	}

	var resp model.ConnectionsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// CloseConnection 关闭指定连接
func (c *Client) CloseConnection(id string) error {
	_, err := c.DoRequest("DELETE", "/connections/"+url.PathEscape(id), nil)
	return err
}

// CloseAllConnections 关闭所有连接
func (c *Client) CloseAllConnections() error {
	_, err := c.DoRequest("DELETE", "/connections", nil)
	return err
}

// GetMemory 获取内存使用信息
func (c *Client) GetMemory() (*model.MemoryResponse, error) {
	httpResp, err := c.doRawRequest("GET", "/memory", nil)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	var mem model.MemoryResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&mem); err != nil {
		return nil, err
	}

	return &mem, nil
}

// GetRules 获取规则列表
func (c *Client) GetRules() (*model.RulesResponse, error) {
	data, err := c.DoRequest("GET", "/rules", nil)
	if err != nil {
		return nil, err
	}

	var resp model.RulesResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// ReloadConfig 通知 mihomo 核心重新加载配置文件
func (c *Client) ReloadConfig(configPath string) error {
	payload := map[string]string{"path": configPath}
	_, err := c.DoRequest("PUT", "/configs?force=true", payload)
	return err
}

// GetConfigs 获取代理配置
func (c *Client) GetConfigs() (*model.ConfigsResponse, error) {
	data, err := c.DoRequest("GET", "/configs", nil)
	if err != nil {
		return nil, err
	}

	var resp model.ConfigsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// UpdateConfig 更新代理配置
func (c *Client) UpdateConfig(reqBody model.UpdateConfigRequest) error {
	_, err := c.DoRequest("PATCH", "/configs", reqBody)
	return err
}
