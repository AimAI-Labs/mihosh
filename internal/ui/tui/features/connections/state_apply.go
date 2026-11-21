package connections

import (
	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
)

// ApplyWSConnections 处理 WebSocket 连接推送（含历史记录检测与相邻样本速率估算）
func (s State) ApplyWSConnections(data service.ConnectionsData) State {
	return s.applyWSConnectionsAt(data, time.Now())
}

func (s State) applyWSConnectionsAt(data service.ConnectionsData, now time.Time) State {
	s.connGen++
	dt := 0.0
	if !s.lastWSAt.IsZero() {
		dt = now.Sub(s.lastWSAt).Seconds()
	}

	currentIDs := make(map[string]model.Connection, len(data.Connections))
	conns := make([]model.Connection, len(data.Connections))
	for i, conn := range data.Connections {
		dlSpeed := conn.DownloadSpeed
		ulSpeed := conn.UploadSpeed
		if dlSpeed == 0 && ulSpeed == 0 && dt > 0 {
			if prev, exists := s.PrevConnIDs[conn.ID]; exists {
				if d := conn.Download - prev.Download; d > 0 {
					dlSpeed = int64(float64(d) / dt)
				}
				if u := conn.Upload - prev.Upload; u > 0 {
					ulSpeed = int64(float64(u) / dt)
				}
			}
		}
		item := model.Connection{
			ID:            conn.ID,
			Upload:        conn.Upload,
			Download:      conn.Download,
			Start:         conn.Start,
			Chains:        conn.Chains,
			Rule:          conn.Rule,
			RulePayload:   conn.RulePayload,
			DownloadSpeed: dlSpeed,
			UploadSpeed:   ulSpeed,
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
		currentIDs[conn.ID] = item
		conns[i] = item
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
	s.lastWSAt = now
	s.Connections = &model.ConnectionsResponse{
		DownloadTotal: data.DownloadTotal,
		UploadTotal:   data.UploadTotal,
		Connections:   conns,
	}
	if s.paused {
		s.pausedUpdates++
	}
	return s
}

// ApplyConnections 应用 REST API 返回的连接数据
func (s State) ApplyConnections(resp *model.ConnectionsResponse) State {
	s.connGen++
	s.Connections = resp
	if s.paused {
		s.pausedUpdates++
	}
	return s
}

// ApplyConnectionClosed 处理单连接关闭后的索引调整
func (s State) ApplyConnectionClosed() State {
	if s.filterList.Cursor > 0 {
		// 临时借用 SetCursor 只更新值，实际限制由随后的 SetItemCount 保证
		s.filterList.Cursor--
	}
	// 确保选中索引不超过连接数量上限
	connCount := s.filteredConnCount()
	s.filterList.SetItemCount(connCount)
	return s
}

// ApplyAllConnectionsClosed 所有连接关闭后重置状态
func (s State) ApplyAllConnectionsClosed() State {
	s.filterList.SetCursor(0)
	s.filterList.ScrollTop = 0
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
