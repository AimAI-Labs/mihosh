package tui

import (
	"context"

	corepage "github.com/AimAI-Labs/mihosh/internal/ui/tui/features/core"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/guide"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/help"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/logs"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/nodes"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/rules"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/settings"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/sub"

	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/notice"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/placeholder"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/bootstrap"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
)

// Model TUI 主模型（仅保留全局共享状态）
type Model struct {
	// 基础设施
	client     *service.Client
	config     *service.Config
	configSvc  *service.ConfigService
	profileSvc *service.ProfileService
	coreSvc    *service.CoreService

	// 路由与布局
	currentPage           layout.PageType
	width                 int
	height                int
	showHelp              bool
	autoRefreshRemaining  int
	autoRefreshSynced     bool
	autoRefreshSyncedTTL  int
	connTickActive        bool
	logsTickActive        bool
	placeholderTickActive bool

	// 共享图表数据（Connections 页面和 StatusBar 共用）
	chartData *model.ChartData

	notice notice.State

	// 内核未连接占位屏的像素动画帧
	placeholderState placeholder.State

	// WebSocket
	connectionEpoch    uint64
	connectionChecked  bool
	connectionChecking bool
	connectionReady    bool
	targetRequest      uint64
	targetPending      *coreTargetRequest
	wsStarted          bool
	wsClient           *service.WSClient
	wsInbox            *wsInbox
	wsCtx              context.Context
	wsCancel           context.CancelFunc

	// 页面子状态
	nodesState    nodes.State
	connsState    connections.State
	logsState     logs.State
	rulesState    rules.State
	subState      sub.State
	settingsState settings.State
	guideState    guide.State
	coreState     corepage.State
	helpState     help.State

	// 自动更新
	updateInfo       *model.UpdateInfo
	hasUpdate        bool
	showUpdateDialog bool
	isUpdating       bool
	updateError      error
}

// NewModel 创建新的 TUI 模型
func NewModel(app *bootstrap.App) Model {
	common.InitKeyBindings()
	cfg := app.Config
	client, testURL, timeout := app.Client, cfg.TestURL, cfg.Timeout

	// 启动时根据配置初始化主题
	if cfg.Theme != "" {
		theme.SetTheme(cfg.Theme)
	}
	theme.SetBorderStyle(cfg.BorderStyle)

	wsCtx, wsCancel := context.WithCancel(context.Background())
	chartData := model.NewChartData(common.ChartPoints)

	initialPage := layout.PageNodes
	if app.ConfigErr != nil {
		initialPage = layout.PageCore
	}
	return Model{
		client:               client,
		connectionChecked:    false,
		coreSvc:              app.CoreSvc,
		coreState:            corepage.NewState(app.CoreSvc, app.ConfigSvc, client),
		config:               cfg,
		configSvc:            app.ConfigSvc,
		profileSvc:           app.ProfileSvc,
		currentPage:          initialPage,
		chartData:            chartData,
		wsClient:             app.WSClient,
		wsInbox:              newWSInbox(),
		wsCtx:                wsCtx,
		wsCancel:             wsCancel,
		nodesState:           nodes.NewState(client, testURL, timeout).SetServices(app.ConfigSvc, app.CoreSvc),
		connsState:           connections.NewState(service.MixedPortToProxyURL(app.Target.Endpoint.MixedPort), model.DefaultSiteTests(), client, timeout, chartData),
		logsState:            logs.NewState(app.IPResolver),
		rulesState:           rules.NewState(client, app.ConfigSvc, app.CoreSvc).SetConfigPath(app.Target.ConfigPath),
		subState:             sub.NewState(app.ProfileSvc),
		settingsState:        settings.NewState(app.ConfigSvc, client),
		autoRefreshRemaining: cfg.AutoRefreshInterval,
	}
}
