package messages

import (
	"time"

	"github.com/AimAI-Labs/mihosh/internal/app/service"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
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
	Target   *service.MihomoTarget
	Err      error
	FromFile bool // API 不可达时从 YAML 文件降级读取
}

type MihomoConfigSavedMsg struct {
	Err     error
	WriteOK bool // YAML 已写入成功（即使 reload 失败也应刷新端点）
}

type MihomoOverridesMsg struct {
	Overrides map[string]string
	ReadAt    time.Time
	Err       error
}

type MihomoOverridesSavedMsg struct {
	Overrides          map[string]string
	WriteOK            bool
	Applied            bool
	Restarted          bool
	NeedsRestart       bool
	EndpointChanged    bool
	Remote             bool
	RemoteNotPersisted bool
	Restored           bool
	Err                error
}

type ShowGuideModalMsg struct {
	Endpoint     string
	SecretMasked string
	ErrMessage   string
}

type HideGuideModalMsg struct{}

// PlaceholderTickMsg 推进内核未连接占位屏的像素动画。
type PlaceholderTickMsg struct{}

// ========= Node / Proxy Testing Messages =========

type TestDoneMsg struct {
	Name    string
	Delay   int
	Err     error
	TestURL string
	BatchID uint64
	TaskID  int
}

// ========= Connections Messages =========

type ConnectionsMsg struct {
	Resp *model.ConnectionsResponse
}

type ConnectionClosedMsg struct {
	ID string
}

type AllConnectionsClosedMsg struct{}

type FilteredConnectionsClosedMsg struct {
	Closed int
	Failed int
}

type DNSQueryResultMsg struct {
	Domain     string
	RecordType string
	Resp       *model.DNSQueryResponse
	Err        error
}

type IPInfoMsg struct {
	RequestID uint64
	Info      *model.IPInfo
	Err       error
}

type CurrentIPInfoMsg struct {
	RequestID uint64
	Info      *model.IPInfo
	Err       error
}

type SiteTestMsg struct {
	Name    string
	Delay   int
	Err     error
	BatchID uint64
	TaskID  int
}

type UnlockTestMsg struct {
	BatchID uint64
	TaskID  int
	Result  model.UnlockResult
}

// ========= Rules Messages =========

type RulesMsg []model.Rule

type RuleEditAppliedMsg struct {
	NoticeKey       string
	Applied         bool
	Remote          bool
	NeedsRestart    bool
	Restarted       bool
	EndpointChanged bool
	Err             error
}

type ProxyChainAppliedMsg struct {
	Group           string
	ExitNode        string
	Cleared         bool
	Applied         bool
	Remote          bool
	NeedsRestart    bool
	Restarted       bool
	EndpointChanged bool
	Err             error
}

type SyncDoneMsg struct {
	Action   string
	Summary  string
	Reloaded bool
	Err      error
}

type UninstallPlanMsg struct {
	Plan service.UninstallPlan
	Err  error
}

type UninstallDoneMsg struct {
	Result service.UninstallResult
	Err    error
}

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
	Data service.ConnectionsData
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

// AutoRefreshResult carries the data fetched during an auto-refresh cycle.
type AutoRefreshResult struct {
	Groups       map[string]model.Group
	OrderedNames []string
	Proxies      map[string]model.Proxy
	Mode         string
}

// AutoRefreshMsg is sent after a periodic auto-refresh fetch completes.
type AutoRefreshMsg struct {
	Result  AutoRefreshResult
	Changed bool
}

type MihomoVersionMsg struct {
	Version string
}

// ThemeChangedMsg 主题已切换，触发全屏重绘
type ThemeChangedMsg struct{}

type NoticeLevel uint8

const (
	NoticeInfo NoticeLevel = iota
	NoticeSuccess
	NoticeWarning
	NoticeError
)

type NoticeMsg struct {
	Text   string
	Level  NoticeLevel
	Detail string
}

type NoticeExpiredMsg struct{ ID uint64 }

// ========= Sub (订阅管理) Messages =========

// SubsLoadedMsg 订阅列表已从配置加载。
type SubsLoadedMsg struct {
	Subs   []service.Profile // 当前订阅元数据列表
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

// SubBatchFetchDoneMsg 批量或定时自动更新远程订阅完成。
type SubBatchFetchDoneMsg struct {
	Total   int
	Updated int
	Failed  int
	Auto    bool
	Err     error
}

// SubAutoUpdateTickMsg 触发一次远程订阅定时过期检查。
type SubAutoUpdateTickMsg time.Time

// SubHistoryLoadedMsg 订阅或覆写历史快照列表加载完成。
type SubHistoryLoadedMsg struct {
	Request   uint64
	Kind      service.SnapshotKind
	UID       string
	Snapshots []service.Snapshot
	Err       error
}

// SubHistoryDiffMsg 历史快照与当前文件的差异结果。
type SubHistoryDiffMsg struct {
	Request uint64
	Rows    []service.FileDiffRow
	Err     error
}

// SubRollbackDoneMsg 历史快照回滚结果。
type SubRollbackDoneMsg struct {
	Kind         service.SnapshotKind
	UID          string
	SnapshotID   string
	MergeErr     error
	Restarted    bool
	NeedsRestart bool
	Remote       bool
	Err          error
}

// SubActivatedMsg 激活订阅流程结束。
// Err != nil 表示激活流程中某步失败。CoreApplied 表示内核已完成重载。
// MergeErr != nil 表示 merge.yaml 语法错误（已降级忽略覆写，非致命）。
type SubActivatedMsg struct {
	UID          string
	BackupName   string // 产生的备份文件名（首次生成时为空）
	Err          error
	MergeErr     error
	Restarted    bool
	NeedsRestart bool
	Remote       bool
	CoreApplied  bool
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
	Err              error
	ClearedOverrides []string
}

// SubMergeAppliedMsg 激活订阅流程（覆写编辑后触发的重载）结束。
// Err != nil 表示生成/备份/写入/重载中某步失败；
// MergeErr != nil 表示 merge.yaml 语法错误（已降级忽略覆写，非致命）。
type SubMergeAppliedMsg struct {
	UID          string
	BackupName   string
	Err          error
	MergeErr     error
	Restarted    bool
	NeedsRestart bool
	Remote       bool
}

// SubRenameDoneMsg 订阅已重命名。
type SubRenameDoneMsg struct{ UID string }

// SubEditDoneMsg 订阅元数据已编辑（调用方负责刷新列表）。
type SubEditDoneMsg struct{ UID string }

type SubSourceCopiedMsg struct{ Success bool }

// SubEditErrorMsg 编辑订阅失败。
type SubEditErrorMsg struct {
	UID string
	Err error
}

func (m SubEditErrorMsg) Error() string { return m.Err.Error() }

// LocalSubImportedMsg 首次启动时自动导入本地 mihomo 配置为本地订阅完成。
// Profile 非空表示导入成功；Err 非 nil 表示导入失败（非致命，静默忽略）。
type LocalSubImportedMsg struct {
	Profile *service.Profile
	Err     error
}

type SubFileLoadedMsg struct {
	Request uint64
	Diff    []service.FileDiffRow
	Err     error
}

type SubFileViewerClosedMsg struct {
	Request uint64
	Err     error
}

// ========= System Status Messages =========

type SysStatusTickMsg time.Time
type SysStatusResultMsg struct {
	Output string
	Err    error
}

// ========= Auto Update Messages =========

type UpdateCheckedMsg struct {
	Info *model.UpdateInfo
}

type UpdateAppliedMsg struct {
	ServiceBinary string
	Err           error
}

// ========= UI Events =========

// PageResizeMsg 页面尺寸变化消息
type PageResizeMsg struct {
	Width  int
	Height int
}

// PageMouseClickMsg 统一派发给子页面的鼠标点击事件
type PageMouseClickMsg struct {
	X      int // 页面内相对X坐标
	Y      int // 页面内相对Y坐标
	Width  int // 页面总宽度
	Height int // 页面总高度
}

// PageMouseScrollMsg 统一派发给子页面的鼠标滚轮事件
type PageMouseScrollMsg struct {
	Up     bool
	X      int // 页面内相对X坐标，若无法解析为 -1
	Y      int // 页面内相对Y坐标，若无法解析为 -1
	Width  int // 页面总宽度
	Height int // 页面总高度
}

// Core operations are delivered globally even when the user changes pages.
type CoreStatusMsg struct {
	Status service.Status
	Err    error
}
type CoreServiceLogMsg struct {
	Unit   string
	Output string
	Err    error
}
type CoreGeoDataMsg struct {
	Files []service.GeoDataFile
	Err   error
}
type CoreOperationMsg struct {
	ID        uint64
	Action    string
	ShellPath string
	Release   *service.Release
	Releases  *service.ReleasePage
	Err       error
}
type CoreProgressTickMsg struct{ ID uint64 }

type CoreProcessLogsMsg struct {
	Start  uint64
	Output string
	Flush  bool
}

type CoreConnectionMsg struct {
	Version  string
	Epoch    uint64
	Err      error
	KeepPage bool
}

type ManagedCoreStartedMsg struct{ Epoch uint64 }

type CoreTargetResolvedMsg struct {
	Request uint64
	Target  service.MihomoTarget
	Config  *service.Config
}

type CoreTargetChangedMsg struct {
	Reset      bool
	Invalidate bool // In-flight requests are stale even when the endpoint is unchanged.
	ConfigPath string
	ProxyURL   string
}

type CoreWSBatchMsg struct {
	Generation  uint64
	Memory      *MemoryWSMsg
	Traffic     *TrafficWSMsg
	Connections *ConnectionsWSMsg
	Logs        []model.LogEntry
	DroppedLogs uint64
}

// CoreHTTPMsg carries the generation captured when the controller read was scheduled.
type CoreHTTPMsg struct {
	Generation uint64
	Payload    any
}

type CoreOwnerChangedMsg struct{ Action string }
