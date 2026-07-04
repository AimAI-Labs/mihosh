package connections

import (
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
)

// ApplyWSConnections 处理 WebSocket 连接推送（含历史记录检测）
func (s State) ApplyWSConnections(data api.ConnectionsData) State {
	s.connGen++
	currentIDs := make(map[string]model.Connection, len(data.Connections))
	for _, conn := range data.Connections {
		currentIDs[conn.ID] = model.Connection{
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

	// 检测已关闭的连接写入 Ring Buffer
	if s.PrevConnIDs != nil {
		hasClosed := false
		for id, conn := range s.PrevConnIDs {
			if _, exists := currentIDs[id]; !exists {
				s.appendClosed(conn)
				hasClosed = true
			}
		}
		if hasClosed {
			s.rebuildCachedClosedConns()
		}
	}
	s.PrevConnIDs = currentIDs
	s.Connections = ConvertToConnectionsResponse(data)
	return s
}

// ApplyConnections 应用 REST API 返回的连接数据
func (s State) ApplyConnections(resp *model.ConnectionsResponse) State {
	s.connGen++
	s.Connections = resp
	return s
}

// ApplySiteTestResult 应用网站测速结果
func (s State) ApplySiteTestResult(name string, delay int, err error) State {
	for i := range s.siteTests {
		if s.siteTests[i].Name == name {
			s.siteTests[i].Testing = false
			if err != nil {
				s.siteTests[i].Delay = 0
				s.siteTests[i].Error = "timeout"
			} else {
				s.siteTests[i].Delay = delay
				s.siteTests[i].Error = ""
			}
			break
		}
	}
	return s
}

// ApplyConnectionClosed 处理单连接关闭后的索引调整
func (s State) ApplyConnectionClosed() State {
	if s.selectedConn > 0 {
		s.selectedConn--
	}
	// 确保选中索引不超过连接数量上限
	connCount := s.filteredConnCount()
	if connCount > 0 && s.selectedConn >= connCount {
		s.selectedConn = connCount - 1
	}
	if s.selectedConn < 0 {
		s.selectedConn = 0
	}
	return s
}

// ApplyAllConnectionsClosed 所有连接关闭后重置状态
func (s State) ApplyAllConnectionsClosed() State {
	s.selectedConn = 0
	s.connScrollTop = 0
	return s
}

// ApplyIPInfo 更新 IP 地理信息
func (s State) ApplyIPInfo(info *model.IPInfo) State {
	s.connIPInfo = info
	return s
}

// ResetPrevConnIDs 重置连接快照（切换到连接页时调用）
func (s State) ResetPrevConnIDs() State {
	s.PrevConnIDs = nil
	return s
}

// UpdateProxyAddr 更新代理地址（设置页保存后调用）
func (s State) UpdateProxyAddr(addr string) State {
	s.proxyAddr = addr
	return s
}
