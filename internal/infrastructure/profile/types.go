// Package profile 实现订阅 (Subscription) 管理子系统。
//
// 设计参照 Clash Verge 的全局扩展覆写模型（src-tauri/src/enhance/merge.rs）：
// 每个 Profile = 一个原始配置来源（远程 URL 或本地文件）+ 一个 merge 覆写 YAML。
// 激活某个 Profile 时：读 raw → 叠加 merge → 生成最终配置 → 原子写入 mihomo
// 配置文件 → 热重载核心。
//
// 本包仅负责「数据结构与无副作用操作」，网络拉取（fetcher）、生成与重载
// （generate）、磁盘读写（store）拆分到同包不同文件，便于独立测试。
package profile

import (
	"fmt"
	"strings"
	"time"
)

// SourceKind 订阅来源类型。
//
// 序列化约定：经 mapstructure 序列化为整数存入 mihosh 的 config.yaml；
// 解析时按整数还原，非法值回退为 SourceRemote。
type SourceKind int

const (
	// SourceRemote 远程订阅 URL（http(s)://）。
	SourceRemote SourceKind = iota
	// SourceLocal 本地配置文件路径。
	SourceLocal
)

// String 返回来源的可读名称（用于 UI 显示与日志）。
func (k SourceKind) String() string {
	switch k {
	case SourceLocal:
		return "local"
	default:
		return "remote"
	}
}

// ParseSourceKind 根据字符串推断来源类型：http(s):// → Remote，其余 → Local。
// 用于添加订阅表单：用户填写单个字段，自动判断类型。
func ParseSourceKind(s string) SourceKind {
	trimmed := strings.TrimSpace(s)
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return SourceRemote
	}
	return SourceLocal
}

// SubSource 描述订阅的原始配置来源与拉取选项。
// Kind 决定使用 URL 还是 Path 字段。
type SubSource struct {
	Kind               SourceKind `mapstructure:"kind"                 yaml:"kind"`
	URL                string     `mapstructure:"url"                  yaml:"url"`                            // Kind==SourceRemote 时使用
	Path               string     `mapstructure:"path"                 yaml:"path"`                           // Kind==SourceLocal 时使用
	UseProxy           bool       `mapstructure:"use_proxy"            yaml:"use_proxy,omitempty"`            // 通过当前 Mihomo 代理拉取远程订阅
	UserAgent          string     `mapstructure:"user_agent"           yaml:"user_agent,omitempty"`           // 自定义 User-Agent（空则使用默认值）
	InsecureSkipVerify bool       `mapstructure:"insecure_skip_verify" yaml:"insecure_skip_verify,omitempty"` // 跳过 TLS 证书校验
	UpdateInterval     int        `mapstructure:"update_interval"      yaml:"update_interval,omitempty"`      // 自动更新周期（分钟），0=关闭
}

// SameOrigin reports whether two SubSources point to the exact same remote URL
// or local file path, ignoring fetch transport options.
func (s SubSource) SameOrigin(other SubSource) bool {
	return s.Kind == other.Kind &&
		strings.TrimSpace(s.URL) == strings.TrimSpace(other.URL) &&
		strings.TrimSpace(s.Path) == strings.TrimSpace(other.Path)
}

// Display 返回来源的可显示字符串（remote 显示 URL，local 显示 Path）。
func (s SubSource) Display() string {
	switch s.Kind {
	case SourceLocal:
		return s.Path
	default:
		return s.URL
	}
}

// Profile 单个订阅的元数据。
//
// raw.yaml（订阅原始配置快照）与 merge.yaml（用户覆写）按 UID 存为独立文件，
// 不进入本结构（避免 config.yaml 膨胀）。
type Profile struct {
	UID       string             `mapstructure:"uid"        yaml:"uid"`        // 短 ID，用作 profiles/<uid>/ 目录名
	Name      string             `mapstructure:"name"       yaml:"name"`       // 显示名
	Source    SubSource          `mapstructure:"source"     yaml:"source"`     // 原始配置来源
	UpdatedAt int64              `mapstructure:"updated_at" yaml:"updated_at"` // 最近成功拉取的 unix 时间戳（秒），0=从未更新
	Usage     *SubscriptionUsage `mapstructure:"usage" yaml:"usage,omitempty"`
}

// IsDueForUpdate reports whether a remote subscription has an enabled auto-update
// schedule whose interval has elapsed (or has never been updated).
func (p Profile) IsDueForUpdate(now time.Time) bool {
	if p.Source.Kind != SourceRemote || p.Source.UpdateInterval <= 0 {
		return false
	}
	if p.UpdatedAt <= 0 {
		return true
	}
	interval := time.Duration(p.Source.UpdateInterval) * time.Minute
	return now.Sub(time.Unix(p.UpdatedAt, 0)) >= interval
}

// FormatUpdateInterval formats an update interval in minutes as a compact label (e.g. 30m, 12h, 7d).
func FormatUpdateInterval(minutes int) string {
	if minutes <= 0 {
		return ""
	}
	if minutes%(24*60) == 0 {
		return fmt.Sprintf("%dd", minutes/(24*60))
	}
	if minutes%60 == 0 {
		return fmt.Sprintf("%dh", minutes/60)
	}
	return fmt.Sprintf("%dm", minutes)
}

type SubscriptionUsage struct {
	UsedBytes     int64  `mapstructure:"used_bytes" yaml:"used_bytes"`
	TotalBytes    int64  `mapstructure:"total_bytes" yaml:"total_bytes"`
	ExpiresAt     int64  `mapstructure:"expires_at" yaml:"expires_at"`
	UploadBytes   *int64 `mapstructure:"upload_bytes" yaml:"upload_bytes,omitempty"`
	DownloadBytes *int64 `mapstructure:"download_bytes" yaml:"download_bytes,omitempty"`
}

// 错误哨兵。集中定义，便于调用方用 errors.Is 判别。
// （具体错误在 store/fetcher/generate 中返回时包装。）
