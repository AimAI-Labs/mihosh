package config

// Config 配置结构
type Config struct {
	APIAddress          string `mapstructure:"api_address"`
	Secret              string `mapstructure:"secret"`
	TestURL             string `mapstructure:"test_url"`
	Timeout             int    `mapstructure:"timeout"`
	ProxyAddress        string `mapstructure:"proxy_address"`
	Language            string `mapstructure:"language"`
	AutoRefreshInterval int    `mapstructure:"auto_refresh_interval"`
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
}
