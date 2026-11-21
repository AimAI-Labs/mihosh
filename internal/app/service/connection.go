package service

import (
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/domain/port"
)

// ConnectionService 连接监控服务
type ConnectionService struct {
	client port.ConnectionAPI
}

// NewConnectionService 创建连接服务
func NewConnectionService(client port.ConnectionAPI) *ConnectionService {
	return &ConnectionService{
		client: client,
	}
}

// GetConnections 获取连接信息
func (s *ConnectionService) GetConnections() (*model.ConnectionsResponse, error) {
	return s.client.GetConnections()
}

// CloseConnection 关闭指定连接
func (s *ConnectionService) CloseConnection(id string) error {
	return s.client.CloseConnection(id)
}

// CloseAllConnections 关闭所有连接
func (s *ConnectionService) CloseAllConnections() error {
	return s.client.CloseAllConnections()
}
