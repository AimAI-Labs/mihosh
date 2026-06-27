package service

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
)

// ConfigService 配置管理服务
type ConfigService struct{}

// NewConfigService 创建配置服务
func NewConfigService() *ConfigService {
	return &ConfigService{}
}

// LoadConfig 加载配置
func (s *ConfigService) LoadConfig() (*config.Config, error) {
	return config.Load()
}

// SaveConfig 保存配置
func (s *ConfigService) SaveConfig(cfg *config.Config) error {
	return config.Save(cfg)
}

// SetConfigValue 设置单个配置项。
// 连接信息（external-controller/secret/mixed-port）已迁移至 mihomo 配置文件，
// 不再通过本方法管理；请用 TUI 的 Mihomo 标签页或 config edit 编辑 mihomo 配置。
func (s *ConfigService) SetConfigValue(key, value string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	switch key {
	case "test_url", "test-url":
		cfg.TestURL = value
	case "timeout":
		var timeout int
		if _, err := fmt.Sscanf(value, "%d", &timeout); err != nil {
			return fmt.Errorf("timeout 必须是数字: %v", err)
		}
		cfg.Timeout = timeout
	case "language":
		if value != "auto" && value != "zh-CN" && value != "en-US" {
			return fmt.Errorf("language 必须是 auto, zh-CN 或 en-US")
		}
		cfg.Language = value
	case "auto_refresh_interval", "auto-refresh-interval":
		var interval int
		if _, err := fmt.Sscanf(value, "%d", &interval); err != nil {
			return fmt.Errorf("auto_refresh_interval 必须是数字: %v", err)
		}
		if interval < 0 {
			return fmt.Errorf("auto_refresh_interval 不能小于 0")
		}
		cfg.AutoRefreshInterval = interval
	case "theme":
		if !theme.IsValid(value) {
			return fmt.Errorf("theme 必须是: %s", strings.Join(theme.Names(), ", "))
		}
		cfg.Theme = value
	default:
		return fmt.Errorf("未知的配置项: %s (可用: test_url, timeout, language, auto_refresh_interval, theme)", key)
	}

	return config.Save(cfg)
}

// FetchMihomoConfig 从 mihomo 配置文件（YAML）读取 5 个托管字段。
// 路径通过 GetMihomoConfigPath() 解析（Linux 下优先用 systemctl -d 参数）。
// 始终从磁盘读取，不依赖 API（API 响应可能含运行时变化，文件才是真实来源）。
func (s *ConfigService) FetchMihomoConfig(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		return s.fetchMihomoConfigFromFile(nil)
	}
}

// fetchMihomoConfigFromFile 从 YAML 文件读取 Mihomo 配置的 5 个托管字段。
// 始终从磁盘读取，确保展示的是配置文件实际内容（而非 API 运行时状态）。
func (s *ConfigService) fetchMihomoConfigFromFile(apiErr error) messages.MihomoConfigMsg {
	endpoint := config.ResolveMihomoEndpoint()
	mihomoCfg := &model.MihomoConfig{
		ExternalController: endpoint.ExternalController,
		Secret:             endpoint.Secret,
		MixedPort:          endpoint.MixedPort,
	}

	// 从 YAML 读取 allow-lan / log-level（ResolveMihomoEndpoint 未解析的字段）
	if path, err := config.GetMihomoConfigPath(); err == nil {
		if data, err := config.ReadMihomoYAML(path); err == nil {
			if v, ok := data["allow-lan"].(bool); ok {
				mihomoCfg.AllowLan = v
			}
			if v, ok := data["log-level"].(string); ok {
				mihomoCfg.LogLevel = v
			}
		}
	}

	return messages.MihomoConfigMsg{
		Config:   mihomoCfg,
		Err:      apiErr,
		FromFile: true,
	}
}

// SaveMihomoConfigField 将单个字段写入 mihomo 物理配置文件并使其生效。
// 流程：写 YAML -> systemctl restart（确保所有变更生效，含端口等不可热重载字段）。
// 非 Linux 或 systemctl 不可用时降级为 API 热重载。
func (s *ConfigService) SaveMihomoConfigField(client *api.Client, key string, value interface{}) tea.Cmd {
	return func() tea.Msg {
		path, err := config.GetMihomoConfigPath()
		if err != nil {
			return messages.MihomoConfigSavedMsg{Err: err}
		}

		if err := config.WriteMihomoField(path, key, value); err != nil {
			return messages.MihomoConfigSavedMsg{Err: fmt.Errorf("write yaml failed: %w", err)}
		}

		// 优先 systemctl restart：确保端口等不可热重载字段也能生效
		if err := config.RestartMihomoService(); err == nil {
			return messages.MihomoConfigSavedMsg{WriteOK: true}
		}

		// 降级：API 热重载（非 Linux 或 systemctl 不可用时）
		if client != nil {
			if err := client.ReloadConfig(path); err != nil {
				return messages.MihomoConfigSavedMsg{
					Err:     fmt.Errorf("systemctl restart and API reload both failed: %w", err),
					WriteOK: true,
				}
			}
		}

		return messages.MihomoConfigSavedMsg{WriteOK: true}
	}
}
