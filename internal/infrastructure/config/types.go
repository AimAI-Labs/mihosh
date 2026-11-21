package config

import (
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
)

// CoreConfig 保存内核托管、下载网络和显式外部 API 连接设置。
// 托管模式的控制器凭据与代理端口仍以 runtime/config.yaml 为准。
type CoreConfig struct {
	Runtime                string             `mapstructure:"runtime" yaml:"runtime"`
	SystemProxy            bool               `mapstructure:"system_proxy" yaml:"system_proxy"`
	TUN                    bool               `mapstructure:"tun" yaml:"tun"`
	DownloadTimeoutSeconds int                `mapstructure:"download_timeout_seconds" yaml:"download_timeout_seconds"`
	ExternalController     string             `mapstructure:"external_controller" yaml:"external_controller"`
	ExternalSecret         string             `mapstructure:"external_secret" yaml:"external_secret"`
	ExternalMixedPort      int                `mapstructure:"external_mixed_port" yaml:"external_mixed_port"`
	Mode                   string             `mapstructure:"mode" yaml:"mode"`
	Version                string             `mapstructure:"version" yaml:"version"`
	PendingVersion         string             `mapstructure:"pending_version" yaml:"pending_version"`
	PreviousVersion        string             `mapstructure:"previous_version" yaml:"previous_version"`
	Proxy                  string             `mapstructure:"proxy" yaml:"proxy"`
	Accelerators           []core.Accelerator `mapstructure:"accelerators" yaml:"accelerators"`
}

// SyncConfig 保存 PostgreSQL 云端同步设置。
// DSN 中的敏感凭据绝不会写入导出压缩包或云端同步快照。
type SyncConfig struct {
	DSN        string `mapstructure:"dsn" yaml:"dsn,omitempty"`
	ProfileKey string `mapstructure:"profile_key" yaml:"profile_key,omitempty"`
	LastSyncAt string `mapstructure:"last_sync_at" yaml:"last_sync_at,omitempty"`
}

// Config 保存应用偏好、订阅元数据与内核管理设置。
type Config struct {
	Core                CoreConfig               `mapstructure:"core"`
	Sync                SyncConfig               `mapstructure:"sync"`
	TestURL             string                   `mapstructure:"test_url"`
	Timeout             int                      `mapstructure:"timeout"`
	Language            string                   `mapstructure:"language"`
	AutoRefreshInterval int                      `mapstructure:"auto_refresh_interval"`
	Theme               string                   `mapstructure:"theme"`
	BorderStyle         string                   `mapstructure:"border_style"`
	ShellProxy          bool                     `mapstructure:"shell_proxy"`
	FileDiffOptions     *profile.FileDiffOptions `mapstructure:"file_diff_options"`

	// 订阅管理：subs 为订阅元数据列表，active_sub 为当前激活订阅 UID（空=未启用）。
	// raw/merge 内容按 UID 存为独立文件（见 profile 包），不进本结构。
	Subs      []profile.Profile `mapstructure:"subs"`
	ActiveSub string            `mapstructure:"active_sub"`

	subscriptionUsageNeedsBackup bool
}

// DefaultConfig 默认配置
var DefaultConfig = Config{
	TestURL:             "http://www.gstatic.com/generate_204",
	Timeout:             5000,
	Language:            "auto",
	AutoRefreshInterval: 5,
	Theme:               "tokyo-night",
	BorderStyle:         "rounded",
}
