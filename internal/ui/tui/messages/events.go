package messages

import (
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
)

// ========= Global Lifecycle & API Messages =========

type GroupsMsg struct {
	Groups       map[string]model.Group
	OrderedNames []string
}

type ProxiesMsg map[string]model.Proxy

type ErrMsg struct{ Err error }

func (e ErrMsg) Error() string { return e.Err.Error() }

type ConfigSavedMsg struct{}

type ConfigModeMsg struct {
	Mode string
}

// ========= Node / Proxy Testing Messages =========

type TestDoneMsg struct {
	Name    string
	Delay   int
	Err     error
	TestURL string
}

type TestAllDoneMsg struct {
	Results map[string]int
}

// ========= Connections Messages =========

type ConnectionsMsg struct {
	Resp *model.ConnectionsResponse
}

type ConnectionClosedMsg struct {
	ID string
}

type AllConnectionsClosedMsg struct{}

type IPInfoMsg struct {
	Info *model.IPInfo
	Err  error
}

type SiteTestMsg struct {
	Name  string
	Delay int
	Err   error
}

// ========= Rules Messages =========

type RulesMsg []model.Rule

// RuleAddedMsg 自定义规则已成功写入配置文件（调用方负责热重载与刷新）。
type RuleAddedMsg struct{}

// RuleAddErrorMsg 写入自定义规则失败（解析/校验/写盘）。
type RuleAddErrorMsg struct{ Err error }

func (m RuleAddErrorMsg) Error() string { return m.Err.Error() }

// ConfigReloadedMsg 调用 mihomo 核心 ReloadConfig 的结果。
type ConfigReloadedMsg struct{ Err error }

// ========= WebSocket Streaming Messages =========

type MemoryWSMsg struct {
	Memory int64
}

type TrafficWSMsg struct {
	Up   int64
	Down int64
}

type ConnectionsWSMsg struct {
	Data api.ConnectionsData
}

type LogsWSMsg struct {
	LogType string
	Payload string
}

// ========= Logs Messages =========

type LogIPResolvedMsg struct {
	IP       string
	Resolved *model.ResolvedIP
}

// ========= UI Ticks =========

type ConnTickMsg time.Time
type LogsTickMsg time.Time

type AutoRefreshTickMsg time.Time

type MihomoVersionMsg struct {
	Version string
}
