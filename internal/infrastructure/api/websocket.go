package api

import (
	"context"
	"encoding/json"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// MemoryData 内存数据
type MemoryData struct {
	Generation uint64 `json:"-"`
	Inuse      int64  `json:"inuse"`
	OSLimit    int64  `json:"oslimit"`
}

// TrafficData 流量数据
type TrafficData struct {
	Generation uint64 `json:"-"`
	Up         int64  `json:"up"`
	Down       int64  `json:"down"`
}

// ConnectionsData 连接数据（与ConnectionsResponse结构相同）
type ConnectionsData struct {
	Generation    uint64           `json:"-"`
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
	Generation uint64 `json:"-"`
	Type       string `json:"type"`    // debug, info, warning, error, silent
	Payload    string `json:"payload"` // 日志内容
}

// WSClient WebSocket客户端
//
// baseURL/secret 支持运行时热更新（UpdateEndpoint）：调用后主动关闭所有
// 现有连接，让 connectStream 在下一轮用新端点重连，无需重启 mihosh。
type WSClient struct {
	mu         sync.RWMutex
	baseURL    string
	secret     string
	generation uint64

	memoryHandler      func(MemoryData)
	trafficHandler     func(TrafficData)
	connectionsHandler func(ConnectionsData)
	logsHandler        func(LogData)

	logLevel string
	run      *wsRun
}

type wsRun struct {
	ctx          context.Context
	cancel       context.CancelFunc
	target       context.Context
	cancelTarget context.CancelFunc
	done         chan struct{}
}

// NewWSClient 创建WebSocket客户端
func NewWSClient(baseURL, secret string) *WSClient {
	return &WSClient{baseURL: baseURL, secret: secret}
}

// UpdateEndpoint cancels the previous target's dials and sockets. The same
// workers then connect to the new target.
func (c *WSClient) UpdateEndpoint(baseURL, secret string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.baseURL = baseURL
	c.secret = secret
	c.generation++
	if c.run != nil {
		c.run.cancelTarget()
		c.run.target, c.run.cancelTarget = context.WithCancel(c.run.ctx)
	}
}

// SetMemoryHandler 设置内存数据处理器
func (c *WSClient) SetMemoryHandler(handler func(MemoryData)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.memoryHandler = handler
}

// SetTrafficHandler 设置流量数据处理器
func (c *WSClient) SetTrafficHandler(handler func(TrafficData)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.trafficHandler = handler
}

// SetConnectionsHandler 设置连接数据处理器
func (c *WSClient) SetConnectionsHandler(handler func(ConnectionsData)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connectionsHandler = handler
}

// SetLogsHandler 设置日志数据处理器
func (c *WSClient) SetLogsHandler(handler func(LogData)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.logsHandler = handler
}

func buildWSURL(baseURL, secret, endpoint string) string {
	// external-controller 原值无 scheme：先补 http:// 再转换为 ws scheme。
	wsURL := ControllerHTTPURL(baseURL)
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

// Start 启动WebSocket连接。取消中的上一次启动退出后才能再次启动。
func (c *WSClient) Start(ctx context.Context) error {
	c.mu.Lock()
	for c.run != nil {
		run := c.run
		if run.ctx.Err() == nil {
			c.mu.Unlock()
			return nil
		}
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil
		case <-run.done:
		}
		c.mu.Lock()
	}
	if ctx.Err() != nil {
		c.mu.Unlock()
		return nil
	}
	run := &wsRun{done: make(chan struct{})}
	run.ctx, run.cancel = context.WithCancel(ctx)
	run.target, run.cancelTarget = context.WithCancel(run.ctx)
	c.run = run
	level := c.logLevel
	c.mu.Unlock()
	if level == "" {
		level = "debug"
	}

	var streams sync.WaitGroup
	streams.Go(func() {
		connectStream(c, run, "memory", func(d MemoryData, generation uint64) {
			d.Generation = generation
			deliverWS(c, &c.memoryHandler, d)
		})
	})
	streams.Go(func() {
		connectStream(c, run, "traffic", func(d TrafficData, generation uint64) {
			d.Generation = generation
			deliverWS(c, &c.trafficHandler, d)
		})
	})
	streams.Go(func() {
		connectStream(c, run, "connections", func(d ConnectionsData, generation uint64) {
			d.Generation = generation
			deliverWS(c, &c.connectionsHandler, d)
		})
	})
	streams.Go(func() {
		connectStream(c, run, "logs?level="+url.QueryEscape(level), func(d LogData, generation uint64) {
			d.Generation = generation
			deliverWS(c, &c.logsHandler, d)
		})
	})
	go func() {
		streams.Wait()
		run.cancel()
		c.mu.Lock()
		c.run = nil
		close(run.done)
		c.mu.Unlock()
	}()
	return nil
}

func deliverWS[T any](c *WSClient, handler *func(T), data T) {
	c.mu.RLock()
	fn := *handler
	c.mu.RUnlock()
	if fn != nil {
		fn(data)
	}
}

// Stop cancels this run and waits for all four workers to release their sockets.
func (c *WSClient) Stop() {
	c.mu.RLock()
	run := c.run
	c.mu.RUnlock()
	if run != nil {
		run.cancel()
		<-run.done
	}
}

func (c *WSClient) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.run != nil && c.run.ctx.Err() == nil
}

// connectStream owns one endpoint through dials, reads and reconnections.
func connectStream[T any](c *WSClient, run *wsRun, endpoint string, handler func(T, uint64)) {
	var readLimit int64
	if strings.HasPrefix(endpoint, "logs?") {
		readLimit = 128 << 10
	}
	for run.ctx.Err() == nil {
		c.mu.RLock()
		ctx, generation := run.target, c.generation
		baseURL, secret := c.baseURL, c.secret
		c.mu.RUnlock()

		wait := 2 * time.Second
		if readWSStream(ctx, buildWSURL(baseURL, secret, endpoint), generation, readLimit, handler) {
			wait = time.Second
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
		case <-timer.C:
		}
		timer.Stop()
	}
}

func readWSStream[T any](ctx context.Context, wsURL string, generation uint64, readLimit int64, handler func(T, uint64)) bool {
	if wsURL == "" || ctx.Err() != nil {
		return false
	}
	// Closing the TCP connection on cancellation also interrupts a handshake
	// blocked after dialing and an established stream blocked in ReadMessage.
	var stopClose func() bool
	dialer := websocket.Dialer{
		HandshakeTimeout: 45 * time.Second,
		NetDialContext: func(dialCtx context.Context, network, addr string) (net.Conn, error) {
			conn, err := (&net.Dialer{}).DialContext(dialCtx, network, addr)
			if err == nil {
				stopClose = context.AfterFunc(ctx, func() { _ = conn.Close() })
			}
			return conn, err
		},
	}
	conn, resp, err := dialer.DialContext(ctx, wsURL, nil)
	if stopClose != nil {
		defer stopClose()
	}
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return false
	}
	defer func() { _ = conn.Close() }()
	conn.SetReadLimit(readLimit)
	for ctx.Err() == nil {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var data T
		if json.Unmarshal(message, &data) == nil && ctx.Err() == nil {
			handler(data, generation)
		}
	}
	return true
}

// Generation identifies the controller snapshot that produced stream messages.
func (c *WSClient) Generation() uint64 { c.mu.RLock(); defer c.mu.RUnlock(); return c.generation }
