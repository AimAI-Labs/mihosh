package config

import "github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"

// Config 配置结构
type Config struct {
	APIAddress          string `mapstructure:"api_address"`
	Secret              string `mapstructure:"secret"`
	TestURL             string `mapstructure:"test_url"`
	Timeout             int `mapstructure:"timeout"`
	ProxyAddress        string `mapstructure:"proxy_address"`
	Language            string `mapstructure:"language"`
	AutoRefreshInterval int    `mapstructure:"auto_refresh_interval"`
	Theme               string `mapstructure:"theme"`

	// 订阅管理：subs 为订阅元数据列表，active_sub 为当前激活订阅 UID（空=未启用）。
	// raw/merge 内容按 UID 存为独立文件（见 profile 包），不进本结构。
	Subs      []profile.Profile `mapstructure:"subs"`
	ActiveSub string             `mapstructure:"active_sub"`
}

// DefaultConfig 默认配置
var DefaultConfig = Config{
	APIAddress:          "http://127.0.0.1:9090",
	Secret:              "",
	TestURL:             "http://www.gstatic.com/generate_204",
	Timeout:             5000,
	ProxyAddress:        "http://127.0.0.1:7890",
	Language:            "auto",
	AutoRefreshInterval: 5,
	Theme:               "tokyo-night",
}
