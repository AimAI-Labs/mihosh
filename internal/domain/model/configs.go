package model

// MihomoConfig 表示 mihomo 内核的运行时配置
type MihomoConfig struct {
	ExternalController string
	Secret             string
	MixedPort          int
	AllowLan           bool
	LogLevel           string
}

// ConfigsResponse 配置信息响应
type ConfigsResponse struct {
	Mode               string `json:"mode"`
	ExternalController string `json:"external-controller"`
	Secret             string `json:"secret"`
	MixedPort          int    `json:"mixed-port"`
	Port               int    `json:"port"`
	AllowLan           bool   `json:"allow-lan"`
	LogLevel           string `json:"log-level"`
}

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	Mode string `json:"mode,omitempty"`
}
