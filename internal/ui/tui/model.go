package tui

import (
	"context"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/logs"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/nodes"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/rules"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/settings"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
)

// Model TUI 主模型（仅保留全局共享状态）
type Model struct {
	// 基础设施
	client    *api.Client
	config    *config.Config
	proxySvc  *service.ProxyService
	configSvc *service.ConfigService
	connSvc   *service.ConnectionService

	// 路由与布局
	currentPage          layout.PageType
	width                int
	height               int
	showHelp             bool
	autoRefreshRemaining int
	autoRefreshSynced    bool
	autoRefreshSyncedTTL int

	// 测速参数（供 NodesState 使用）
	testURL string
	timeout int

	// 共享图表数据（Connections 页面和 StatusBar 共用）
	chartData *model.ChartData

	// 全局错误（状态栏显示）
	err         error
	notice      string
	noticeTicks int

	// WebSocket
	wsClient  *api.WSClient
	wsMsgChan chan interface{}
	wsCtx     context.Context
	wsCancel  context.CancelFunc

	// IP 解析器
	ipResolver *service.IPResolver

	// 五个页面子状态
	nodesState    nodes.State
	connsState    connections.State
	logsState     logs.State
	rulesState    rules.State
	settingsState settings.State
}

// NewModel 创建新的 TUI 模型
func NewModel(client *api.Client, testURL string, timeout int) Model {
	common.InitKeyBindings()
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		cfg = &config.DefaultConfig
	}

	proxySvc := service.NewProxyService(client, testURL, timeout)
	configSvc := service.NewConfigService()
	connSvc := service.NewConnectionService(client)

	wsClient := api.NewWSClient(cfg.APIAddress, cfg.Secret)
	wsCtx, wsCancel := context.WithCancel(context.Background())
	ipResolver := service.NewIPResolver()

	return Model{
		client:               client,
		config:               cfg,
		proxySvc:             proxySvc,
		configSvc:            configSvc,
		connSvc:              connSvc,
		testURL:              testURL,
		timeout:              timeout,
		currentPage:          layout.PageNodes,
		chartData:            model.NewChartData(common.ChartPoints),
		wsClient:             wsClient,
		wsMsgChan:            make(chan interface{}, common.WSMsgChanCap),
		wsCtx:                wsCtx,
		wsCancel:             wsCancel,
		ipResolver:           ipResolver,
		nodesState:           nodes.State{},
		connsState:           connections.NewState(cfg.ProxyAddress, model.DefaultSiteTests()),
		logsState:            logs.NewState(),
		rulesState:           rules.State{},
		settingsState:        settings.State{},
		autoRefreshRemaining: cfg.AutoRefreshInterval,
	}
}
