package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	Use:   "config",
}

var configShowCmd = &cobra.Command{
	Use:   "show [--output json|table|plain]",
	Example: `  mihosh config show
  mihosh config show --output table
  mihosh config show --output json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		format, err := parseOutputFormat(configShowOutput)
		if err != nil {
			return wrapParameterError(err)
		}

		configSvc := service.NewConfigService()
		cfg, err := configSvc.LoadConfig()
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

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]

		configSvc := service.NewConfigService()
		if err := configSvc.SetConfigValue(key, value); err != nil {
			if isConfigSetValidationError(err) {
				return wrapParameterError(fmt.Errorf(i18n.T("cli.config.set.err_set")+": %w", err))
			}
			return wrapConfigError(fmt.Errorf(i18n.T("cli.config.set.err_set")+": %w", err))
		}

		fmt.Println(i18n.Tf("cli.config.set.success", key, value))
		return nil
	},
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
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
	cfg, err := config.Load()
	if err != nil {
		warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
		fmt.Println(warnStyle.Render(i18n.T("cli.config.edit.reload_warn")))
		return nil
	}

	client := loadClient(cfg)
	if err := client.ReloadConfig(configPath); err != nil {
		warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
		fmt.Println(warnStyle.Render(i18n.Tf("cli.config.edit.reload_fail", err)))
		fmt.Println(warnStyle.Render(i18n.T("cli.config.edit.reload_fail_hint")))
		return nil
	}

	reloadStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	fmt.Println(reloadStyle.Render(i18n.T("cli.config.edit.reload_success")))
	return nil
}

func init() {
	configShowCmd.Flags().StringVar(&configShowOutput, "output", string(outputFormatPlain), "")
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

func detectEditor() string {
	var editors []string
	if runtime.GOOS == "windows" {
		editors = []string{"code.cmd", "code", "notepad.exe", "notepad"}
	} else {
		editors = []string{"code", "vim", "vi", "nano"}
	}

	for _, e := range editors {
		if _, err := exec.LookPath(e); err == nil {
			return e
		}
	}
	return ""
}

func runEditor(editor, path string) error {
	cmd := exec.Command(editor, path)
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
	return config.GetMihomoConfigPath()
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

	editor := preferredEditor
	if editor == "" {
		editor = os.Getenv("VISUAL")
		if editor == "" {
			editor = os.Getenv("EDITOR")
		}
	}
	if editor == "" {
		editor = detectEditorFn()
	}
	if editor == "" {
		return fmt.Errorf("%s", i18n.T("cli.config.edit.err_no_editor"))
	}

	if err := runEditorFn(editor, path); err != nil {
		return fmt.Errorf(i18n.T("cli.config.edit.err_launch_editor")+": %w", err)
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

	msg := strings.TrimSpace(err.Error())
	if strings.Contains(msg, "未找到可用的编辑器") || strings.Contains(msg, "启动编辑器失败") {
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
			TestURL             string `json:"test_url"`
			TimeoutMS           int    `json:"timeout_ms"`
			AutoRefreshInterval int    `json:"auto_refresh_interval"`
			ConfigFile          string `json:"config_file"`
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
		fmt.Fprintln(tw, "KEY\tVALUE")
		fmt.Fprintf(tw, "TEST_URL\t%s\n", cfg.TestURL)
		fmt.Fprintf(tw, "TIMEOUT_MS\t%d\n", cfg.Timeout)
		fmt.Fprintf(tw, "AUTO_REFRESH_INTERVAL\t%d\n", cfg.AutoRefreshInterval)
		fmt.Fprintf(tw, "CONFIG_FILE\t%s\n", configPath)
		mi := buildMihomoShowInfo(endpoint, mihomoPath)
		fmt.Fprintf(tw, "%s\t\n", i18n.T("cli.config.show.label_mihomo_conn_table"))
		fmt.Fprintf(tw, "EXTERNAL_CONTROLLER\t%s\n", mi.ExternalController)
		fmt.Fprintf(tw, "SECRET\t%s\n", mi.Secret)
		fmt.Fprintf(tw, "MIXED_PORT\t%d\n", mi.MixedPort)
		fmt.Fprintf(tw, "PROXY_URL\t%s\n", mi.ProxyURL)
		fmt.Fprintf(tw, "MIHOMO_CONFIG_FILE\t%s\n", mi.ConfigPath)
		return tw.Flush()
	case outputFormatPlain:
		mi := buildMihomoShowInfo(endpoint, mihomoPath)
		fmt.Fprintln(w, i18n.T("cli.config.show.label_current"))
		fmt.Fprintf(w, "  %s: %s\n", i18n.T("cli.config.show.label_test_url"), cfg.TestURL)
		fmt.Fprintf(w, "  %s: %dms\n", i18n.T("cli.config.show.label_timeout"), cfg.Timeout)
		fmt.Fprintf(w, "  %s: %ds\n", i18n.T("cli.config.show.label_auto_refresh"), cfg.AutoRefreshInterval)
		fmt.Fprintf(w, "\n%s: %s\n", i18n.T("cli.config.show.label_config_file"), configPath)
		fmt.Fprintln(w, "\n"+i18n.T("cli.config.show.label_mihomo_conn"))
		fmt.Fprintf(w, "  External Controller: %s\n", mi.ExternalController)
		fmt.Fprintf(w, "  %s: %s\n", i18n.T("cli.config.show.label_secret"), mi.Secret)
		fmt.Fprintf(w, "  Mixed Port: %d\n", mi.MixedPort)
		fmt.Fprintf(w, "  %s: %s\n", i18n.T("cli.config.show.label_proxy_url"), mi.ProxyURL)
		fmt.Fprintf(w, "  %s: %s\n", i18n.T("cli.config.show.label_mihomo_config_file"), mi.ConfigPath)
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
