package messages

import (
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
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

type MihomoConfigMsg struct {
	Config   *model.MihomoConfig
	Err      error
	FromFile bool // API 不可达时从 YAML 文件降级读取
}

type MihomoConfigSavedMsg struct {
	Err     error
	WriteOK bool // YAML 已写入成功（即使 reload 失败也应刷新端点）
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

// RuleDeletedMsg 规则已从配置文件删除（调用方负责热重载与刷新）。
type RuleDeletedMsg struct{}

// RuleDeleteErrorMsg 删除规则失败（未找到 / 写盘失败）。
type RuleDeleteErrorMsg struct{ Err error }

func (m RuleDeleteErrorMsg) Error() string { return m.Err.Error() }

// RuleEditedMsg 规则已成功修改（调用方负责热重载与刷新）。
type RuleEditedMsg struct{}

// RuleEditErrorMsg 修改规则失败（未找到旧规则 / 写盘失败）。
type RuleEditErrorMsg struct{ Err error }

func (m RuleEditErrorMsg) Error() string { return m.Err.Error() }

// ConfigReloadedMsg 调用 mihomo 核心 ReloadConfig 的结果。
type ConfigReloadedMsg struct{ Err error }

// ConfigEditFinishedMsg 外部编辑器编辑配置文件结束。
// Err 为 nil 表示编辑器正常退出（调用方负责热重载核心与刷新）；非 nil 表示启动或退出失败。
type ConfigEditFinishedMsg struct{ Err error }

// CoreActionDoneMsg 核心管理操作（升级、重启、重载等）完成。
type CoreActionDoneMsg struct {
	Action        string
	NeedReloadAll bool
	DelayMs       int
}

// CoreActionErrorMsg 核心管理操作失败。
type CoreActionErrorMsg struct {
	Action string
	Err    error
}

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

// ThemeChangedMsg 主题已切换，触发全屏重绘
type ThemeChangedMsg struct{}

// NoticeMsg 底栏通知消息，页面通过 tea.Cmd 发送至主 Model 设置 notice。
type NoticeMsg struct{ Text string }

// ========= Sub (订阅管理) Messages =========

// SubsLoadedMsg 订阅列表已从配置加载。
type SubsLoadedMsg struct {
	Subs   []profile.Profile // 当前订阅元数据列表
	Active string            // 当前激活订阅 UID（空=未激活）
}

// SubFetchDoneMsg 订阅原始配置已成功拉取/读取。
type SubFetchDoneMsg struct{ UID string }

// SubFetchErrorMsg 拉取订阅失败。
type SubFetchErrorMsg struct {
	UID string
	Err error
}

func (m SubFetchErrorMsg) Error() string { return m.Err.Error() }

// SubActivatedMsg 激活订阅流程结束。
// Err != nil 表示生成/备份/写入/重载中某步失败（配置可能已写入，见 ProfileService.Activate）。
// MergeErr != nil 表示 merge.yaml 语法错误（已降级忽略覆写，非致命）。
type SubActivatedMsg struct {
	UID        string
	BackupName string // 产生的备份文件名（首次生成时为空）
	Err        error
	MergeErr   error
}

// SubAddDoneMsg 订阅已新增（调用方负责刷新列表）。
type SubAddDoneMsg struct{ UID string }

// SubAddErrorMsg 新增订阅失败。
type SubAddErrorMsg struct{ Err error }

func (m SubAddErrorMsg) Error() string { return m.Err.Error() }

// SubDeletedMsg 订阅已删除（调用方负责刷新列表）。
type SubDeletedMsg struct{ UID string }

// SubDeleteErrorMsg 删除订阅失败。
type SubDeleteErrorMsg struct{ Err error }

func (m SubDeleteErrorMsg) Error() string { return m.Err.Error() }

// SubMergeSavedMsg 订阅覆写已保存。
type SubMergeSavedMsg struct{ UID string }

// SubMergeSaveErrorMsg 保存覆写失败。
type SubMergeSaveErrorMsg struct {
	UID string
	Err error
}

func (m SubMergeSaveErrorMsg) Error() string { return m.Err.Error() }

// MergeEditFinishedMsg 外部编辑器编辑全局 merge.yaml 结束。
// Err 为 nil 表示编辑器正常退出（文件已写盘）；非 nil 表示启动或退出失败。
type MergeEditFinishedMsg struct {
	Err                         error
	HasUnsupportedManagedFields bool
}

// SubMergeAppliedMsg 激活订阅流程（覆写编辑后触发的重载）结束。
// Err != nil 表示生成/备份/写入/重载中某步失败；
// MergeErr != nil 表示 merge.yaml 语法错误（已降级忽略覆写，非致命）。
type SubMergeAppliedMsg struct {
	UID                         string
	BackupName                  string
	Err                         error
	MergeErr                    error
	HasUnsupportedManagedFields bool
}

// SubRenameDoneMsg 订阅已重命名。
type SubRenameDoneMsg struct{ UID string }

// SubEditDoneMsg 订阅元数据已编辑（调用方负责刷新列表）。
type SubEditDoneMsg struct{ UID string }

// SubEditErrorMsg 编辑订阅失败。
type SubEditErrorMsg struct {
	UID string
	Err error
}

func (m SubEditErrorMsg) Error() string { return m.Err.Error() }

// LocalSubImportedMsg 首次启动时自动导入本地 mihomo 配置为本地订阅完成。
// Profile 非空表示导入成功；Err 非 nil 表示导入失败（非致命，静默忽略）。
type LocalSubImportedMsg struct {
	Profile *profile.Profile
	Err     error
}

// SubRawEditFinishedMsg 外部编辑器查看/编辑具体的订阅 raw.yaml 结束。
type SubRawEditFinishedMsg struct {
	UID string
	Err error
}

// ========= System Status Messages =========

type SysStatusTickMsg time.Time
type SysStatusResultMsg struct {
	Output string
	Err    error
}
