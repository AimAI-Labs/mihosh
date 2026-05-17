package service

import (
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
)

// ConnectionService 连接监控服务
type ConnectionService struct {
	client *api.Client
}

// NewConnectionService 创建连接服务
func NewConnectionService(client *api.Client) *ConnectionService {
	return &ConnectionService{
		client: client,
	}
}

// GetConnections 获取连接信息
func (s *ConnectionService) GetConnections() (*model.ConnectionsResponse, error) {
	return s.client.GetConnections()
}
