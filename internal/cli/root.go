package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/AimAI-Labs/mihosh/internal/bootstrap"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "mihosh",
	Version:       model.Version,
	SilenceErrors: true,
	SilenceUsage:  true,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return wrapParameterError(cobra.NoArgs(cmd, args))
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// 默认行为：启动TUI界面。
		// 配置缺失（含首次启动）直接用默认值继续，不在启动期写文件；
		// 连接信息由 mihomo 配置文件自动发现。
		app := bootstrap.NewApp()
		defer app.Close()
		if app.ConfigErr == nil {
			i18n.SetLanguageOverride(app.Config.Language)
		}

		m := tui.NewModel(app)

		p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
		finalModel, err := p.Run()
		if fm, ok := finalModel.(tui.Model); ok {
			fm.Close()
			if text := fm.ExitNotice(); text != "" {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), text)
			}
		} else {
			m.Close()
		}
		if err != nil {
			return fmt.Errorf(i18n.T("cli.root.err_start")+": %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(newCoreCommand())
	rootCmd.AddCommand(newSetupCommand())
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(selectCmd)
	rootCmd.AddCommand(testCmd)
	rootCmd.AddCommand(testGroupCmd)
	rootCmd.AddCommand(connectionsCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(modeCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(serviceCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(proxyOnCmd)
	rootCmd.AddCommand(proxyOffCmd)
	rootCmd.AddCommand(newEnvCommand())
	rootCmd.AddCommand(newActivateCommand(), newProxyCommand())
	rootCmd.AddCommand(newDNSCommand())
	rootCmd.AddCommand(newSyncCommand())
}

// Execute 执行命令
func Execute() {
	os.Exit(executeRootCommand(rootCmd, os.Stderr))
}

func executeRootCommand(root *cobra.Command, stderr io.Writer) int {
	i18n.Init()
	if cfg, err := config.Load(); err == nil && cfg != nil && cfg.Language != "" {
		i18n.SetLanguageOverride(cfg.Language)
	}
	localizeCommands()
	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintln(stderr, renderCommandError(err))
		return exitCodeForError(err)
	}
	return exitCodeOK
}

func loadConfigOrDefault() (*config.Config, error) {
	cfg, err := config.Load()
	if errors.Is(err, config.ErrConfigNotFound) {
		defaults := config.DefaultConfig
		return &defaults, nil
	}
	return cfg, err
}

// loadClient 统一构造 API Client：从 mihomo 配置文件解析连接信息，
// 超时取自 Mihosh 配置（加载失败则回退默认）。供所有 CLI 子命令复用，
// 避免各处重复 api.NewClient + endpoint 解析。
func loadClient(cfg *config.Config) *api.Client {
	return bootstrap.NewClient(cfg)
}

func localizeCommands() {
	if activate, _, err := rootCmd.Find([]string{"activate"}); err == nil {
		activate.Short = i18n.T("cli.activate.short")
	}
	if proxy, _, err := rootCmd.Find([]string{"proxy"}); err == nil {
		proxy.Short = i18n.T("cli.proxy.short")
		for _, action := range proxy.Commands() {
			action.Short = i18n.T("cli.proxy." + action.Name() + ".short")
		}
	}
	if env, _, err := rootCmd.Find([]string{"env"}); err == nil {
		env.Short, env.Long = i18n.T("cli.env.short"), i18n.T("cli.env.long")
	}
	if core, _, err := rootCmd.Find([]string{"core"}); err == nil {
		core.Short = i18n.T("cli.core.short")
		for _, child := range core.Commands() {
			child.Short = i18n.T("core.action." + child.Name())
			if child.Name() == "network-recover" {
				child.Short = i18n.T("core.action.recover")
			}
			for _, nested := range child.Commands() {
				prefix := "core.network."
				if child.Name() == "service" {
					prefix = "core.service."
				}
				nested.Short = i18n.T(prefix + nested.Name())

			}
			for _, flagName := range []string{"version", "file", "sha256"} {
				if flag := child.Flags().Lookup(flagName); flag != nil {
					flag.Usage = i18n.T("cli.core." + flagName)
				}
			}
		}
	}
	if setup, _, err := rootCmd.Find([]string{"setup"}); err == nil {
		setup.Short = i18n.T("cli.setup.short")
	}
	if dns, _, err := rootCmd.Find([]string{"dns"}); err == nil && dns != nil {
		dns.Short = i18n.T("cli.dns.short")
		if q, _, qErr := dns.Find([]string{"query"}); qErr == nil && q != nil {
			q.Short = i18n.T("cli.dns.query.short")
			q.Long = i18n.T("cli.dns.query.long")
			if f := q.Flags().Lookup("type"); f != nil {
				f.Usage = i18n.T("cli.dns.query.flag_type")
			}
			if f := q.Flags().Lookup("output"); f != nil {
				f.Usage = i18n.T("cli.dns.query.flag_output")
			}
		}
	}
	if syncCmd, _, err := rootCmd.Find([]string{"sync"}); err == nil && syncCmd != nil {
		syncCmd.Short = i18n.T("cli.sync.short")
		for _, subName := range []string{"export", "import", "push", "pull", "status"} {
			if sub, _, subErr := syncCmd.Find([]string{subName}); subErr == nil && sub != nil {
				sub.Short = i18n.T("cli.sync." + subName + ".short")
				if f := sub.Flags().Lookup("output"); f != nil {
					f.Usage = i18n.T("cli.sync.flag_output")
				}
				if f := sub.Flags().Lookup("dsn"); f != nil {
					f.Usage = i18n.T("cli.sync.flag_dsn")
				}
				if f := sub.Flags().Lookup("key"); f != nil {
					f.Usage = i18n.T("cli.sync.flag_key")
				}
				if f := sub.Flags().Lookup("no-redact"); f != nil {
					f.Usage = i18n.T("cli.sync.flag_no_redact")
				}
			}
		}
	}
	rootCmd.Short = i18n.T("cli.root.short")
	rootCmd.Long = i18n.T("cli.root.long")

	statusCmd.Short = i18n.T("cli.status.short")
	statusCmd.Long = i18n.T("cli.status.long")

	configCmd.Short = i18n.T("cli.config.short")
	configShowCmd.Short = i18n.T("cli.config.show.short")
	configShowCmd.Long = i18n.T("cli.config.show.long")
	configSetCmd.Short = i18n.T("cli.config.set.short")
	configSetCmd.Long = i18n.T("cli.config.set.long")
	configEditCmd.Short = i18n.T("cli.config.edit.short")
	configEditCmd.Long = i18n.T("cli.config.edit.long")

	connectionsCmd.Short = i18n.T("cli.connections.short")
	connectionsCmd.Long = i18n.T("cli.connections.long")

	listCmd.Short = i18n.T("cli.list.short")
	listCmd.Long = i18n.T("cli.list.long")

	modeCmd.Short = i18n.T("cli.mode.short")
	modeCmd.Long = i18n.T("cli.mode.long")

	selectCmd.Short = i18n.T("cli.select.short")
	selectCmd.Long = i18n.T("cli.select.long")

	testCmd.Use = i18n.T("cli.test.use")
	testCmd.Short = i18n.T("cli.test.short")
	testCmd.Long = i18n.T("cli.test.long")
	testGroupCmd.Short = i18n.T("cli.test_group.short")

	versionCmd.Short = i18n.T("cli.version.short")
	versionCmd.Long = i18n.T("cli.version.long")

	// cobra generated completion command
	if completionCmd, _, err := rootCmd.Find([]string{"completion"}); err == nil && completionCmd != nil {
		completionCmd.Short = i18n.T("cli.completion.short")
		completionCmd.Long = i18n.T("cli.completion.long")
	}

	serviceCmd.Short = i18n.T("cli.service.short")
	serviceCmd.Long = i18n.T("cli.service.long")
	if c, _, err := serviceCmd.Find([]string{"logs"}); err == nil && c != nil {
		c.Short = i18n.T("cli.service.logs.short")
		if f := c.Flags().Lookup("lines"); f != nil {
			f.Usage = i18n.T("cli.service.logs.flag_lines")
		}
		if f := c.Flags().Lookup("follow"); f != nil {
			f.Usage = i18n.T("cli.service.logs.flag_follow")
		}
	}
	for _, action := range []string{"start", "stop", "restart", "enable", "disable", "status"} {
		if c, _, err := serviceCmd.Find([]string{action}); err == nil && c != nil {
			c.Short = i18n.T("cli.service.action." + action)
		}
	}

	doctorCmd.Short = i18n.T("cli.doctor.short")
	doctorCmd.Long = i18n.T("cli.doctor.long")

	proxyOnCmd.Short = i18n.T("cli.proxy.on.short")
	proxyOnCmd.Long = i18n.T("cli.proxy.on.long")
	proxyOffCmd.Short = i18n.T("cli.proxy.off.short")
	proxyOffCmd.Long = i18n.T("cli.proxy.off.long")

	// Set flag descriptions
	if f := statusCmd.Flags().Lookup("output"); f != nil {
		f.Usage = i18n.T("cli.status.flag_output")
	}
	if f := configShowCmd.Flags().Lookup("output"); f != nil {
		f.Usage = i18n.T("cli.config.show.flag_output")
	}
	if f := configEditCmd.Flags().Lookup("editor"); f != nil {
		f.Usage = i18n.T("cli.config.edit.flag_editor")
	}
	if f := configEditCmd.Flags().Lookup("path"); f != nil {
		f.Usage = i18n.T("cli.config.edit.flag_path")
	}
	if f := connectionsCmd.Flags().Lookup("output"); f != nil {
		f.Usage = i18n.T("cli.connections.flag_output")
	}
	if f := listCmd.Flags().Lookup("output"); f != nil {
		f.Usage = i18n.T("cli.list.flag_output")
	}
	if f := testCmd.Flags().Lookup("output"); f != nil {
		f.Usage = i18n.T("cli.test.flag_output")
	}
	if f := versionCmd.Flags().Lookup("output"); f != nil {
		f.Usage = i18n.T("cli.version.flag_output")
	}
	if f := serviceCmd.PersistentFlags().Lookup("unit"); f != nil {
		f.Usage = i18n.T("cli.service.flag_unit")
	}
	if f := doctorCmd.Flags().Lookup("output"); f != nil {
		f.Usage = i18n.T("cli.doctor.flag_output")
	}
}
