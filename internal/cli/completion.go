package cli

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion <bash|zsh|fish|powershell>",
	Short: "生成 shell 补全脚本",
	Long: `生成指定 shell 的命令补全脚本。

安装说明
--------

Bash（Linux）:
  mihosh completion bash | sudo tee /etc/bash_completion.d/mihosh > /dev/null
  source /etc/bash_completion.d/mihosh

Bash（macOS，需要 bash-completion@2）:
  brew install bash-completion@2
  mihosh completion bash > "$(brew --prefix)/etc/bash_completion.d/mihosh"
  # 在 ~/.bash_profile 中确保已加载 bash_completion

Zsh:
  mihosh completion zsh > "${fpath[1]}/_mihosh"
  # 若 fpath 不包含用户目录，可使用：
  mkdir -p ~/.zsh/completions
  mihosh completion zsh > ~/.zsh/completions/_mihosh
  # 在 ~/.zshrc 中添加：
  #   fpath=(~/.zsh/completions $fpath)
  #   autoload -Uz compinit && compinit

Oh My Zsh:
  mihosh completion zsh > ~/.oh-my-zsh/completions/_mihosh
  exec zsh

Fish:
  mihosh completion fish | source
  # 永久安装：
  mihosh completion fish > ~/.config/fish/completions/mihosh.fish

PowerShell:
  mihosh completion powershell | Out-String | Invoke-Expression
  # 永久安装：
  mihosh completion powershell >> $PROFILE`,
	Example: `  mihosh completion bash
  mihosh completion zsh
  mihosh completion fish
  mihosh completion powershell`,
	ValidArgs:          []string{"bash", "zsh", "fish", "powershell"},
	Args:               cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	DisableFlagParsing: false,
	RunE: func(cmd *cobra.Command, args []string) error {
		root := cmd.Root()
		switch args[0] {
		case "bash":
			return root.GenBashCompletion(os.Stdout)
		case "zsh":
			return root.GenZshCompletion(os.Stdout)
		case "fish":
			return root.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return root.GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
