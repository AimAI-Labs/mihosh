package service

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
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

// FetchMihomoConfig fetches the live config from Mihomo API and supplements with YAML if needed.
// 当 API 不可达时（如端口错误），降级从 YAML 文件读取，让用户仍可查看/编辑配置。
func (s *ConfigService) FetchMihomoConfig(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return s.fetchMihomoConfigFromFile(fmt.Errorf("API client is nil"))
		}
		resp, err := client.GetConfigs()
		if err != nil {
			// API 不可达，降级从 YAML 文件读取
			return s.fetchMihomoConfigFromFile(err)
		}

		mihomoCfg := &model.MihomoConfig{
			ExternalController: resp.ExternalController,
			Secret:             resp.Secret,
			MixedPort:          resp.MixedPort,
			AllowLan:           resp.AllowLan,
			LogLevel:           resp.LogLevel,
		}

		if mihomoCfg.MixedPort == 0 && resp.Port > 0 {
			mihomoCfg.MixedPort = resp.Port
		}

		// Fallback for connection settings if empty (API might omit them)
		endpoint := config.ResolveMihomoEndpoint()
		if mihomoCfg.ExternalController == "" {
			mihomoCfg.ExternalController = endpoint.ExternalController
		}
		if mihomoCfg.Secret == "" {
			mihomoCfg.Secret = endpoint.Secret
		}
		if mihomoCfg.MixedPort == 0 {
			mihomoCfg.MixedPort = endpoint.MixedPort
		}
		return messages.MihomoConfigMsg{Config: mihomoCfg}
	}
}

// fetchMihomoConfigFromFile 从 YAML 文件降级读取 Mihomo 配置。
// 返回的 MihomoConfigMsg 同时携带 Config（文件读取结果）和 Err（原始 API 错误），
// FromFile=true 告诉 UI 层当前为离线模式。
func (s *ConfigService) fetchMihomoConfigFromFile(apiErr error) messages.MihomoConfigMsg {
	endpoint := config.ResolveMihomoEndpoint()
	mihomoCfg := &model.MihomoConfig{
		ExternalController: endpoint.ExternalController,
		Secret:             endpoint.Secret,
		MixedPort:          endpoint.MixedPort,
	}

	// 尝试从 YAML 读取 allow-lan / log-level 等 API-only 字段
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

// SaveMihomoConfigField writes to YAML and reloads the API.
// 连接信息完全归 mihomo 配置文件管理：保存后不再反向同步到 Mihosh 配置，
// 客户端端点的热刷新改由 MihomoConfigSavedMsg 在 update 层驱动。
// 当 YAML 写入成功但 reload 失败时（如修改 external-controller 后旧端口已不可达），
// 仍标记 WriteOK=true 以便 update 层刷新端点。
func (s *ConfigService) SaveMihomoConfigField(client *api.Client, profileSvc *ProfileService, key string, value interface{}) tea.Cmd {
	return func() tea.Msg {
		// 1. Get the current active subscription
		cfg, err := config.Load()
		if err != nil {
			return messages.MihomoConfigSavedMsg{Err: err}
		}

		activeSub := cfg.ActiveSub

		// Path A: Has Active Sub
		if activeSub != "" && profileSvc != nil {
			if err := profile.WriteMergeField(key, value); err != nil {
				return messages.MihomoConfigSavedMsg{Err: fmt.Errorf("write merge.yaml failed: %w", err)}
			}
			
			// Trigger a full merge and core reload
			res, err := profileSvc.Activate(activeSub)
			if err != nil {
				return messages.MihomoConfigSavedMsg{
					Err:      fmt.Errorf("activate after merge failed: %w", err),
					WriteOK:  true, // The file was written to merge.yaml successfully
				}
			}
			
			// If Activate succeeded but merge syntax was bad, log it or bubble it up, but it's generally OK.
			if res.MergeErr != nil {
				// We don't fail the save, but it might mean the merge override was ignored.
			}
			
			return messages.MihomoConfigSavedMsg{WriteOK: true}
		}

		// Path B: No Active Sub, fallback to config.yaml directly
		path, err := config.GetMihomoConfigPath()
		if err != nil {
			return messages.MihomoConfigSavedMsg{Err: err}
		}

		if err := config.WriteMihomoField(path, key, value); err != nil {
			return messages.MihomoConfigSavedMsg{Err: fmt.Errorf("write yaml failed: %w", err)}
		}

		if client != nil {
			if err := client.ReloadConfig(path); err != nil {
				// YAML 已写入，但 reload 失败（端口变更后预期行为）
				return messages.MihomoConfigSavedMsg{
					Err:     fmt.Errorf("reload failed: %w", err),
					WriteOK: true,
				}
			}
		}

		return messages.MihomoConfigSavedMsg{WriteOK: true}
	}
}
