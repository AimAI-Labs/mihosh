package api

import (
	"encoding/json"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WSMessage WebSocket消息类型
type WSMessage struct {
	Type string
	Data []byte
}

// MemoryData 内存数据
type MemoryData struct {
	Inuse   int64 `json:"inuse"`
	OSLimit int64 `json:"oslimit"`
}

// TrafficData 流量数据
type TrafficData struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

// ConnectionsData 连接数据（与ConnectionsResponse结构相同）
type ConnectionsData struct {
	DownloadTotal int64            `json:"downloadTotal"`
	UploadTotal   int64            `json:"uploadTotal"`
	Connections   []ConnectionData `json:"connections"`
}

// ConnectionData 单个连接数据
type ConnectionData struct {
	ID            string         `json:"id"`
	Metadata      ConnectionMeta `json:"metadata"`
	Upload        int64          `json:"upload"`
	Download      int64          `json:"download"`
	Start         string         `json:"start"`
	Chains        []string       `json:"chains"`
	Rule          string         `json:"rule"`
	RulePayload   string         `json:"rulePayload"`
	DownloadSpeed int64          `json:"downloadSpeed"`
	UploadSpeed   int64          `json:"uploadSpeed"`
}

// ConnectionMeta 连接元数据
type ConnectionMeta struct {
	Network         string `json:"network"`
	Type            string `json:"type"`
	SourceIP        string `json:"sourceIP"`
	DestinationIP   string `json:"destinationIP"`
	SourcePort      string `json:"sourcePort"`
	DestinationPort string `json:"destinationPort"`
	Host            string `json:"host"`
	Process         string `json:"process"`
	ProcessPath     string `json:"processPath"`
}

// LogData 日志数据
type LogData struct {
	Type    string `json:"type"`    // debug, info, warning, error, silent
	Payload string `json:"payload"` // 日志内容
}

// WSClient WebSocket客户端
//
// baseURL/secret 支持运行时热更新（UpdateEndpoint）：调用后主动关闭所有
// 现有连接，让 connectStream 在下一轮用新端点重连，无需重启 mihosh。
type WSClient struct {
	mu      sync.RWMutex
	baseURL string
	secret  string

	connsMu sync.Mutex
	conns   map[string]*websocket.Conn // key: 端点名称

	memoryHandler      func(MemoryData)
	trafficHandler     func(TrafficData)
	connectionsHandler func(ConnectionsData)
	logsHandler        func(LogData)

	logLevel  string // 日志级别过滤
	stopChan  chan struct{}
	isRunning bool
	runningMu sync.Mutex
}

// NewWSClient 创建WebSocket客户端
func NewWSClient(baseURL, secret string) *WSClient {
	return &WSClient{
		baseURL:  baseURL,
		secret:   secret,
		conns:    make(map[string]*websocket.Conn),
		stopChan: make(chan struct{}),
	}
}

// UpdateEndpoint 原子更新 baseURL 与 secret，并主动断开所有现有连接，
// 触发 connectStream 在下一轮用新端点重连。
// 若 client 未启动（Start 未调用或已 Stop），仅更新字段，不重连。
func (c *WSClient) UpdateEndpoint(baseURL, secret string) {
	c.mu.Lock()
	c.baseURL = baseURL
	c.secret = secret
	c.mu.Unlock()

	// 关闭现有连接：connectStream 的 ReadMessage 会返回错误，
	// 进入重连循环，下一轮 buildWSURL 读到新值。
	c.connsMu.Lock()
	for key, conn := range c.conns {
		conn.Close()
		delete(c.conns, key)
	}
	c.connsMu.Unlock()
}

// SetMemoryHandler 设置内存数据处理器
func (c *WSClient) SetMemoryHandler(handler func(MemoryData)) {
	c.memoryHandler = handler
}

// SetTrafficHandler 设置流量数据处理器
func (c *WSClient) SetTrafficHandler(handler func(TrafficData)) {
	c.trafficHandler = handler
}

// SetConnectionsHandler 设置连接数据处理器
func (c *WSClient) SetConnectionsHandler(handler func(ConnectionsData)) {
	c.connectionsHandler = handler
}

// SetLogsHandler 设置日志数据处理器
func (c *WSClient) SetLogsHandler(handler func(LogData)) {
	c.logsHandler = handler
}

// SetLogLevel 设置日志级别过滤
func (c *WSClient) SetLogLevel(level string) {
	c.logLevel = level
}

// buildWSURL 构建WebSocket URL
func (c *WSClient) buildWSURL(endpoint string) string {
	c.mu.RLock()
	baseURL := c.baseURL
	secret := c.secret
	c.mu.RUnlock()

	// external-controller 原值无 scheme：先补 http:// 再转换为 ws scheme。
	wsURL := ensureScheme(baseURL)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)

	u, err := url.Parse(wsURL + "/" + endpoint)
	if err != nil {
		return ""
	}

	if secret != "" {
		q := u.Query()
		q.Set("token", secret)
		u.RawQuery = q.Encode()
	}

	return u.String()
}

// setConn 线程安全地保存当前连接
func (c *WSClient) setConn(key string, conn *websocket.Conn) {
	c.connsMu.Lock()
	if conn == nil {
		delete(c.conns, key)
	} else {
		c.conns[key] = conn
	}
	c.connsMu.Unlock()
}

// Start 启动WebSocket连接
func (c *WSClient) Start() error {
	c.runningMu.Lock()
	if c.isRunning {
		c.runningMu.Unlock()
		return nil
	}
	c.isRunning = true
	c.stopChan = make(chan struct{})
	c.runningMu.Unlock()

	level := c.logLevel
	if level == "" {
		level = "debug"
	}

	go connectStream(c, "memory", func(d MemoryData) {
		if c.memoryHandler != nil {
			c.memoryHandler(d)
		}
	})
	go connectStream(c, "traffic", func(d TrafficData) {
		if c.trafficHandler != nil {
			c.trafficHandler(d)
		}
	})
	go connectStream(c, "connections", func(d ConnectionsData) {
		if c.connectionsHandler != nil {
			c.connectionsHandler(d)
		}
	})
	go connectStream(c, "logs?level="+level, func(d LogData) {
		if c.logsHandler != nil {
			c.logsHandler(d)
		}
	})

	return nil
}

// Stop 停止所有WebSocket连接
func (c *WSClient) Stop() {
	c.runningMu.Lock()
	if !c.isRunning {
		c.runningMu.Unlock()
		return
	}
	c.isRunning = false
	close(c.stopChan)
	c.runningMu.Unlock()

	c.connsMu.Lock()
	for key, conn := range c.conns {
		conn.Close()
		delete(c.conns, key)
	}
	c.connsMu.Unlock()
}

// IsRunning 检查是否正在运行
func (c *WSClient) IsRunning() bool {
	c.runningMu.Lock()
	defer c.runningMu.Unlock()
	return c.isRunning
}

// connectStream 通用WebSocket流连接，处理重连生命周期。
// endpoint 同时作为 conns map 的 key，因此带查询参数的端点（如 "logs?level=debug"）
// 与其他端点不会冲突。
func connectStream[T any](c *WSClient, endpoint string, handler func(T)) {
	for {
		select {
		case <-c.stopChan:
			return
		default:
		}

		wsURL := c.buildWSURL(endpoint)
		if wsURL == "" {
			select {
			case <-c.stopChan:
				return
			case <-time.After(2 * time.Second):
				continue
			}
		}

		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			select {
			case <-c.stopChan:
				return
			case <-time.After(2 * time.Second):
				continue
			}
		}

		c.setConn(endpoint, conn)

		// 读取消息，直到连接断开或收到停止信号
		for {
			select {
			case <-c.stopChan:
				conn.Close()
				c.setConn(endpoint, nil)
				return
			default:
			}

			_, message, err := conn.ReadMessage()
			if err != nil {
				break
			}

			var data T
			if err := json.Unmarshal(message, &data); err != nil {
				continue
			}
			handler(data)
		}

		c.setConn(endpoint, nil)

		// 断线后等待重连，期间响应停止信号
		select {
		case <-c.stopChan:
			return
		case <-time.After(1 * time.Second):
		}
	}
}
