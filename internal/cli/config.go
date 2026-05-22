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
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "配置管理",
}

var configInitCmd = &cobra.Command{
	Use:     "init",
	Short:   "初始化配置",
	Example: `  mihosh config init`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configSvc := service.NewConfigService()
		if err := configSvc.InitConfig(); err != nil {
			return wrapConfigError(fmt.Errorf("初始化配置失败: %w", err))
		}
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show [--output json|table|plain]",
	Short: "显示当前配置（支持多种输出格式）",
	Long: `显示当前配置内容。

可通过 --output 选择输出格式：
  plain  人类可读文本（默认）
  table  表格输出
  json   结构化 JSON 输出`,
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
			return wrapConfigError(fmt.Errorf("加载配置失败: %w", err))
		}

		configDir, _ := config.GetConfigDir()
		configPath := filepath.Join(configDir, "config.yaml")

		if err := renderConfigShow(os.Stdout, cfg, configPath, format); err != nil {
			return fmt.Errorf("渲染输出失败: %w", err)
		}
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "设置配置项",
	Long: `设置配置项

可用的配置项:
  api-address  - mihosh API 地址 (例如: http://127.0.0.1:9090)
  secret       - API 密钥
  test-url     - 测速 URL (例如: http://www.gstatic.com/generate_204)
  timeout      - 超时时间，单位毫秒 (例如: 5000)
  proxy-address - HTTP 代理地址 (例如: http://127.0.0.1:7890)
  auto-refresh-interval - TUI 自动刷新间隔，单位秒，0 表示关闭

示例:
  mihosh config set api-address http://127.0.0.1:9090
  mihosh config set secret your-secret-here
  mihosh config set test-url http://www.google.com/generate_204
  mihosh config set timeout 3000
  mihosh config set proxy-address http://127.0.0.1:7890
  mihosh config set auto-refresh-interval 5`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]

		configSvc := service.NewConfigService()
		if err := configSvc.SetConfigValue(key, value); err != nil {
			if isConfigSetValidationError(err) {
				return wrapParameterError(fmt.Errorf("设置配置失败: %w", err))
			}
			return wrapConfigError(fmt.Errorf("设置配置失败: %w", err))
		}

		fmt.Printf("✓ 已设置 %s = %s\n", key, value)
		return nil
	},
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "在编辑器中打开配置文件进行编辑",
	Long: `在编辑器中直接编辑配置文件。

默认会检测环境变量 VISUAL 或 EDITOR，如果未设置，将尝试常用的编辑器。
修改完成后会验证配置文件的语法。

示例:
  mihosh config edit
  mihosh config edit --editor vim`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 启动选择器 TUI
		p := tea.NewProgram(initialSelectorModel())
		m, err := p.Run()
		if err != nil {
			return fmt.Errorf("选择器运行失败: %w", err)
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
		return wrapConfigError(fmt.Errorf("获取配置目录失败: %w", err))
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
			return wrapConfigError(fmt.Errorf("配置文件不存在: %s\n可运行 `mihosh config init` 重新初始化配置。", configPath))
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
		return wrapConfigError(fmt.Errorf("无法读取配置文件信息: %w", err))
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
		fmt.Println(warnStyle.Render("⚠ 无法加载 mihosh 配置，跳过自动重载核心。请手动重启 mihomo。"))
		return nil
	}

	client := api.NewClient(cfg)
	if err := client.ReloadConfig(configPath); err != nil {
		warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
		fmt.Println(warnStyle.Render(fmt.Sprintf("⚠ 自动重载核心失败: %v", err)))
		fmt.Println(warnStyle.Render("  请手动重启 mihomo 使配置生效。"))
		return nil
	}

	reloadStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	fmt.Println(reloadStyle.Render("✓ 已自动重载 mihomo 核心，新配置已生效。"))
	return nil
}

func init() {
	configShowCmd.Flags().StringVar(&configShowOutput, "output", string(outputFormatPlain), "输出格式: json|table|plain")
	configEditCmd.Flags().StringVar(&configEditEditor, "editor", "", "指定编辑器 (例如: code, vim, nano)")
	configEditCmd.Flags().StringVar(&configEditPath, "path", "", "指定 Mihomo 配置文件或目录路径")
	configCmd.AddCommand(configInitCmd)
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
		return fmt.Errorf("配置文件过大 (%d 字节)，为了安全起见拒绝打开", info.Size())
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
		return fmt.Errorf("不支持的文件格式: %s", ext)
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
		return "", fmt.Errorf("指定的 mihomo 配置路径不存在: %s", path)
	}

	if info.IsDir() {
		for _, name := range []string{"config.yaml", "config.yml"} {
			configPath := filepath.Join(path, name)
			if _, err := os.Stat(configPath); err == nil {
				return configPath, nil
			}
		}
		return "", fmt.Errorf("目录 %s 下未找到默认配置文件，请确认 `config.yaml` 或 `config.yml` 存在", path)
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".yaml" && ext != ".yml" {
		return "", fmt.Errorf("不支持的 mihomo 配置文件格式: %s", ext)
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
		return fmt.Errorf("未找到可用的编辑器，请使用 --editor 指定或设置 EDITOR 环境变量")
	}

	if err := runEditorFn(editor, path); err != nil {
		return fmt.Errorf("启动编辑器失败: %w", err)
	}
	if err := validateConfigFile(path); err != nil {
		return fmt.Errorf("配置文件语法错误: %w", err)
	}

	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	fmt.Println(successStyle.Render("✓ 配置已成功更新并验证。"))
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
	switch format {
	case outputFormatJSON:
		payload := struct {
			APIAddress          string `json:"api_address"`
			Secret              string `json:"secret"`
			TestURL             string `json:"test_url"`
			TimeoutMS           int    `json:"timeout_ms"`
			ProxyAddress        string `json:"proxy_address"`
			AutoRefreshInterval int    `json:"auto_refresh_interval"`
			ConfigFile          string `json:"config_file"`
		}{
			APIAddress:          cfg.APIAddress,
			Secret:              utils.MaskSecret(cfg.Secret),
			TestURL:             cfg.TestURL,
			TimeoutMS:           cfg.Timeout,
			ProxyAddress:        cfg.ProxyAddress,
			AutoRefreshInterval: cfg.AutoRefreshInterval,
			ConfigFile:          configPath,
		}
		return writeJSON(w, payload)
	case outputFormatTable:
		tw := newTabWriter(w)
		fmt.Fprintln(tw, "KEY\tVALUE")
		fmt.Fprintf(tw, "API_ADDRESS\t%s\n", cfg.APIAddress)
		fmt.Fprintf(tw, "SECRET\t%s\n", utils.MaskSecret(cfg.Secret))
		fmt.Fprintf(tw, "TEST_URL\t%s\n", cfg.TestURL)
		fmt.Fprintf(tw, "TIMEOUT_MS\t%d\n", cfg.Timeout)
		fmt.Fprintf(tw, "PROXY_ADDRESS\t%s\n", cfg.ProxyAddress)
		fmt.Fprintf(tw, "AUTO_REFRESH_INTERVAL\t%d\n", cfg.AutoRefreshInterval)
		fmt.Fprintf(tw, "CONFIG_FILE\t%s\n", configPath)
		return tw.Flush()
	case outputFormatPlain:
		fmt.Fprintln(w, "当前配置:")
		fmt.Fprintf(w, "  API 地址: %s\n", cfg.APIAddress)
		fmt.Fprintf(w, "  密钥:     %s\n", utils.MaskSecret(cfg.Secret))
		fmt.Fprintf(w, "  测速 URL: %s\n", cfg.TestURL)
		fmt.Fprintf(w, "  超时:     %dms\n", cfg.Timeout)
		fmt.Fprintf(w, "  代理地址: %s\n", cfg.ProxyAddress)
		fmt.Fprintf(w, "  自动刷新: %ds\n", cfg.AutoRefreshInterval)
		fmt.Fprintf(w, "\n配置文件位置: %s\n", configPath)
		return nil
	default:
		return fmt.Errorf("不支持的输出格式: %s", format)
	}
}
