package common

import "github.com/AimAI-Labs/mihosh/internal/domain/model"

const (
	// 布局相关
	StatusBarHeight  = 2 // 分隔线 + 信息行
	SidebarWidth     = 6 // 侧边栏宽度 (与 sidebar.go 同步)
	MinContentHeight = 5
	MinMainWidth     = 20

	// 文本提示相关
	SymbolScrollbarThumb = "┃"
	SymbolScrollbarTrack = "│"
	SymbolSelectActive   = "► "
	SymbolSelectInactive = "  "
	SymbolCheck          = "✓ "

	// Ring Buffer 容量 (引用 model 层的全局定义)
	ClosedConnCap = model.ClosedConnCap
	LogsCap       = model.LogsCap
	ChartPoints   = model.ChartPoints

	// 业务逻辑常量
	TestConcurrency = model.TestConcurrency
	IPApiURL        = model.IPApiURL
	WSMsgChanCap    = 100

	// 常用边距
	DefaultPadding = 1
	TableMargin    = 2
)
