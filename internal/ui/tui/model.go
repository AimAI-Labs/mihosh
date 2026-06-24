package tui

import (
	"context"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/logs"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/nodes"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/rules"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/settings"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/sub"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
)

// newRulesState 构造规则页面初始状态，并注入当前 Mihomo 配置文件路径。
// 路径解析失败时静默留空（用户尝试添加规则时会得到明确错误）。
func newRulesState() rules.State {
	s := rules.State{}
	if path, err := config.GetMihomoConfigPath(); err == nil {
		s = s.SetConfigPath(path)
	}
	return s
}

// Model TUI 主模型（仅保留全局共享状态）
type Model struct {
	// 基础设施
	client    *api.Client
	config    *config.Config
	proxySvc  *service.ProxyService
	configSvc *service.ConfigService
	connSvc   *service.ConnectionService
	profileSvc *service.ProfileService

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

	// 六个页面子状态
	nodesState    nodes.State
	connsState    connections.State
	logsState     logs.State
	rulesState    rules.State
	subState      sub.State
	settingsState settings.State
}

// NewModel 创建新的 TUI 模型
func NewModel(client *api.Client, testURL string, timeout int) Model {
	common.InitKeyBindings()
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		cfg = &config.DefaultConfig
	}

	// 启动时根据配置初始化主题
	if cfg.Theme != "" {
		theme.SetTheme(cfg.Theme)
	}

	proxySvc := service.NewProxyService(client, testURL, timeout)
	configSvc := service.NewConfigService()
	connSvc := service.NewConnectionService(client)
	profileSvc := service.NewProfileService(client)

	// 连接信息由 mihomo 配置文件自动发现（external-controller/secret/mixed-port）
	endpoint := config.ResolveMihomoEndpoint()
	wsClient := api.NewWSClient(endpoint.ExternalController, endpoint.Secret)
	wsCtx, wsCancel := context.WithCancel(context.Background())
	ipResolver := service.NewIPResolver()

	return Model{
		client:               client,
		config:               cfg,
		proxySvc:             proxySvc,
		configSvc:            configSvc,
		connSvc:              connSvc,
		profileSvc:           profileSvc,
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
		connsState:           connections.NewState(config.MixedPortToProxyURL(endpoint.MixedPort), model.DefaultSiteTests()),
		logsState:            logs.NewState(),
		rulesState:           newRulesState(),
		subState:             sub.State{},
		settingsState:        settings.State{},
		autoRefreshRemaining: cfg.AutoRefreshInterval,
	}
}
