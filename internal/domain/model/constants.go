package model

// 业务逻辑常量 (跨层级共享)
const (
	// 测速相关
	TestConcurrency = 20
	IPApiURL        = "http://ip-api.com/json?fields=61439"

	// 数据容量 (Ring Buffer)
	ClosedConnCap = 1000
	LogsCap       = 1000
	ChartPoints   = 1000
)
