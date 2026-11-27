package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use: "config",
}

var configShowCmd = &cobra.Command{
	Use:  "show [--output json|table|plain]",
	Args: cobra.NoArgs,
	Example: `  mihosh config show
  mihosh config show --output table
  mihosh config show --output json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		format, err := parseOutputFormat(configShowOutput)
		if err != nil {
			return wrapParameterError(err)
		}

		cfg, err := loadConfigOrDefault()
		if err != nil {
			return wrapConfigError(fmt.Errorf(i18n.T("cli.config.show.err_load")+": %w", err))
		}

		configDir, _ := config.GetConfigDir()
		configPath := filepath.Join(configDir, "config.yaml")

		if err := renderConfigShow(os.Stdout, cfg, configPath, format); err != nil {
			return fmt.Errorf(i18n.T("cli.config.show.err_render")+": %w", err)
		}
		return nil
	},
}

var configSetCmd = newConfigSetCommand()

func newConfigSetCommand() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:  "set <key> <value>",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseOutputFormat(output)
			if err != nil {
				return wrapParameterError(err)
			}
			key := args[0]
			value := args[1]

			configSvc := service.NewConfigService()
			if err := configSvc.SetConfigValue(key, value); err != nil {
				if isConfigSetValidationError(err) {
					return wrapParameterError(fmt.Errorf(i18n.T("cli.config.set.err_set")+": %w", err))
				}
				return wrapConfigError(fmt.Errorf(i18n.T("cli.config.set.err_set")+": %w", err))
			}

			return writeFieldResult(cmd.OutOrStdout(), format, i18n.Tf("cli.config.set.success", key, value),
				outputField{"key", key}, outputField{"value", value})
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "plain", "plain|table|json")
	return cmd
}

var configEditCmd = &cobra.Command{
	Use:  "edit",
	Args: cobra.NoArgs,
	Example: `  mihosh config edit
  mihosh config edit --editor vim`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 启动选择器 TUI
		p := tea.NewProgram(initialSelectorModel())
		m, err := p.Run()
		if err != nil {
			return fmt.Errorf(i18n.T("cli.config.edit.err_selector")+": %w", err)
		}

		sel := m.(selectorModel)
		switch sel.choice {
		case choiceMihosh:
			return runMihoshConfigEdit()
		case choiceMihomo:
			return runMihomoConfigEdit()
		default:
			return nil
		}
	},
}

func runMihoshConfigEdit() error {
	configDir, err := config.GetConfigDir()
	if err != nil {
		return wrapConfigError(fmt.Errorf(i18n.T("cli.config.edit.err_get_dir")+": %w", err))
	}

	// 按优先级查找配置文件
	extensions := []string{".yaml", ".yml", ".json", ".toml"}
	var configPath string
	for _, ext := range extensions {
		path := filepath.Join(configDir, "config"+ext)
		if _, err := os.Stat(path); err == nil {
			configPath = path
			break
		}
	}

	// 如果都不存在，默认使用 config.yaml
	if configPath == "" {
		configPath = filepath.Join(configDir, "config.yaml")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return wrapConfigError(fmt.Errorf(i18n.T("cli.config.edit.err_not_found"), configPath))
		}
	}

	if err := editConfigFileWithEditor(configPath, configEditEditor); err != nil {
		return classifyConfigEditError(err)
	}
	return nil
}

func runMihomoConfigEdit() error {
	path, err := resolveAutoMihomoConfigTarget()
	if err != nil {
		return wrapConfigError(err)
	}

	// 记录编辑前的文件修改时间
	beforeInfo, err := os.Stat(path)
	if err != nil {
		return wrapConfigError(fmt.Errorf(i18n.T("cli.config.edit.err_stat_config")+": %w", err))
	}
	beforeModTime := beforeInfo.ModTime()

	if err := editConfigFileWithEditor(path, configEditEditor); err != nil {
		return classifyConfigEditError(err)
	}

	// 检测文件是否被修改
	afterInfo, err := os.Stat(path)
	if err != nil {
		return nil // 编辑成功但无法检测变更，跳过重载
	}
	if !afterInfo.ModTime().After(beforeModTime) {
		return nil // 文件未修改，无需重载
	}

	// 文件已修改，自动重载 mihomo 核心
	return reloadMihomoCore(path)
}

func reloadMihomoCore(configPath string) error {
	cfg, err := loadConfigOrDefault()
	if err != nil {
		warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
		fmt.Println(warnStyle.Render(i18n.T("cli.config.edit.reload_warn")))
		return nil
	}

	client := loadClient(cfg)
	if result := service.NewConfigService().ReloadMihomoConfig(client, configPath); result.Err != nil {
		warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
		fmt.Println(warnStyle.Render(i18n.Tf("cli.config.edit.reload_fail", result.Err)))
		fmt.Println(warnStyle.Render(i18n.T("cli.config.edit.reload_fail_hint")))
		return nil
	}

	reloadStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	fmt.Println(reloadStyle.Render(i18n.T("cli.config.edit.reload_success")))
	return nil
}

func init() {
	configShowCmd.Flags().StringVarP(&configShowOutput, "output", "o", string(outputFormatPlain), "")
	configEditCmd.Flags().StringVar(&configEditEditor, "editor", "", "")
	configEditCmd.Flags().StringVar(&configEditPath, "path", "", "")
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configEditCmd)
}

var configShowOutput string
var configEditEditor string
var configEditPath string
var runEditorFn = runEditor
var detectEditorFn = detectEditor

func checkConfigFileSize(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() > 1024*1024 {
		return fmt.Errorf(i18n.T("cli.config.edit.err_file_too_large"), info.Size())
	}
	return nil
}

var (
	errEditorNotFound = errors.New("editor not found")
	errEditorLaunch   = errors.New("editor launch failed")
)

func detectEditor() string {
	return utils.ResolveEditor()
}

func runEditor(editor, path string) error {
	parts := utils.SplitEditorCommand(editor)
	if len(parts) == 0 {
		return errEditorNotFound
	}
	args := append(parts[1:], path)
	cmd := exec.Command(parts[0], args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func validateConfigFile(path string) error {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		// 使用 viper 验证
		v := viper.New()
		v.SetConfigFile(path)
		v.SetConfigType("yaml")
		return v.ReadInConfig()
	case ".json":
		v := viper.New()
		v.SetConfigFile(path)
		v.SetConfigType("json")
		return v.ReadInConfig()
	case ".toml":
		v := viper.New()
		v.SetConfigFile(path)
		v.SetConfigType("toml")
		return v.ReadInConfig()
	default:
		return fmt.Errorf(i18n.T("cli.config.edit.unsupported_format"), ext)
	}
}

func resolveAutoMihomoConfigTarget() (string, error) {
	if strings.TrimSpace(configEditPath) != "" {
		return resolveMihomoConfigTarget(configEditPath)
	}
	return config.GetMihomoConfigPathForWrite()
}

func resolveMihomoConfigTarget(input string) (string, error) {
	path := filepath.Clean(strings.TrimSpace(input))
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf(i18n.T("cli.config.edit.err_path_not_found"), path)
	}

	if info.IsDir() {
		for _, name := range []string{"config.yaml", "config.yml"} {
			configPath := filepath.Join(path, name)
			if _, err := os.Stat(configPath); err == nil {
				return configPath, nil
			}
		}
		return "", fmt.Errorf(i18n.T("cli.config.edit.err_no_default"), path)
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".yaml" && ext != ".yml" {
		return "", fmt.Errorf(i18n.T("cli.config.edit.unsupported_mihomo"), ext)
	}

	return path, nil
}

func editConfigFileWithEditor(path, preferredEditor string) error {
	if err := checkConfigFileSize(path); err != nil {
		return err
	}

	editor := strings.TrimSpace(preferredEditor)
	if editor == "" {
		editor = detectEditorFn()
	}
	if editor == "" {
		return fmt.Errorf("%w: %s", errEditorNotFound, i18n.T("cli.config.edit.err_no_editor"))
	}

	if err := runEditorFn(editor, path); err != nil {
		return fmt.Errorf("%w: %s: %w", errEditorLaunch, i18n.T("cli.config.edit.err_launch_editor"), err)
	}
	if err := validateConfigFile(path); err != nil {
		return fmt.Errorf(i18n.T("cli.config.edit.err_syntax")+": %w", err)
	}

	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	fmt.Println(successStyle.Render(i18n.T("cli.config.edit.success")))
	return nil
}

func classifyConfigEditError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, errEditorNotFound) || errors.Is(err, errEditorLaunch) {
		return wrapGeneralError(err)
	}
	return wrapConfigError(err)
}

func wrapGeneralError(err error) error {
	if err == nil {
		return nil
	}
	return &commandError{kind: commandErrorGeneral, err: err}
}

func isConfigSetValidationError(err error) bool {
	if errors.Is(err, service.ErrInvalidConfigValue) {
		return true
	}
	msg := strings.TrimSpace(err.Error())
	return strings.Contains(msg, "未知的配置项:") ||
		strings.Contains(msg, "timeout 必须是数字:") ||
		strings.Contains(msg, "auto_refresh_interval 必须是数字:") ||
		strings.Contains(msg, "auto_refresh_interval 不能小于 0")
}

func renderConfigShow(w io.Writer, cfg *config.Config, configPath string, format outputFormat) error {
	// 连接信息（external-controller/secret/mixed-port）由 mihomo 配置文件自动发现，
	// 此处作为只读小节展示，值来自 ResolveMihomoEndpoint + GetMihomoConfigPath。
	endpoint := config.ResolveMihomoEndpoint()
	mihomoPath, _ := config.GetMihomoConfigPath()

	switch format {
	case outputFormatJSON:
		payload := struct {
			TestURL             string         `json:"test_url"`
			TimeoutMS           int            `json:"timeout_ms"`
			AutoRefreshInterval int            `json:"auto_refresh_interval"`
			ConfigFile          string         `json:"config_file"`
			Mihomo              mihomoShowInfo `json:"mihomo"`
		}{
			TestURL:             cfg.TestURL,
			TimeoutMS:           cfg.Timeout,
			AutoRefreshInterval: cfg.AutoRefreshInterval,
			ConfigFile:          configPath,
			Mihomo:              buildMihomoShowInfo(endpoint, mihomoPath),
		}
		return writeJSON(w, payload)
	case outputFormatTable:
		tw := newTabWriter(w)
		if _, err := fmt.Fprintln(tw, "KEY\tVALUE"); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(tw, "TEST_URL\t%s\n", cfg.TestURL); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(tw, "TIMEOUT_MS\t%d\n", cfg.Timeout); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(tw, "AUTO_REFRESH_INTERVAL\t%d\n", cfg.AutoRefreshInterval); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(tw, "CONFIG_FILE\t%s\n", configPath); err != nil {
			return err
		}
		mi := buildMihomoShowInfo(endpoint, mihomoPath)
		if _, err := fmt.Fprintf(tw, "%s\t\n", i18n.T("cli.config.show.label_mihomo_conn_table")); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(tw, "EXTERNAL_CONTROLLER\t%s\n", mi.ExternalController); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(tw, "SECRET\t%s\n", mi.Secret); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(tw, "MIXED_PORT\t%d\n", mi.MixedPort); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(tw, "PROXY_URL\t%s\n", mi.ProxyURL); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(tw, "MIHOMO_CONFIG_FILE\t%s\n", mi.ConfigPath); err != nil {
			return err
		}
		return tw.Flush()
	case outputFormatPlain:
		mi := buildMihomoShowInfo(endpoint, mihomoPath)
		if _, err := fmt.Fprintln(w, i18n.T("cli.config.show.label_current")); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  %s: %s\n", i18n.T("cli.config.show.label_test_url"), cfg.TestURL); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  %s: %dms\n", i18n.T("cli.config.show.label_timeout"), cfg.Timeout); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  %s: %ds\n", i18n.T("cli.config.show.label_auto_refresh"), cfg.AutoRefreshInterval); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "\n%s: %s\n", i18n.T("cli.config.show.label_config_file"), configPath); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "\n"+i18n.T("cli.config.show.label_mihomo_conn")); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  External Controller: %s\n", mi.ExternalController); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  %s: %s\n", i18n.T("cli.config.show.label_secret"), mi.Secret); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  Mixed Port: %d\n", mi.MixedPort); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  %s: %s\n", i18n.T("cli.config.show.label_proxy_url"), mi.ProxyURL); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  %s: %s\n", i18n.T("cli.config.show.label_mihomo_config_file"), mi.ConfigPath); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf(i18n.T("cli.config.show.unsupported_format"), format)
	}
}

// mihomoShowInfo config show 中只读展示的 Mihomo 连接信息。
type mihomoShowInfo struct {
	ExternalController string `json:"external_controller"`
	Secret             string `json:"secret"`
	MixedPort          int    `json:"mixed_port"`
	ProxyURL           string `json:"proxy_url"`
	ConfigPath         string `json:"config_path,omitempty"`
}

// buildMihomoShowInfo 由解析出的 endpoint + 配置文件路径构造展示信息（secret 掩码）。
func buildMihomoShowInfo(endpoint config.MihomoEndpoint, mihomoPath string) mihomoShowInfo {
	return mihomoShowInfo{
		ExternalController: endpoint.ExternalController,
		Secret:             utils.MaskSecret(endpoint.Secret),
		MixedPort:          endpoint.MixedPort,
		ProxyURL:           config.MixedPortToProxyURL(endpoint.MixedPort),
		ConfigPath:         mihomoPath,
	}
}
