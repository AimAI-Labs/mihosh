package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
)

func quoteShellProxy(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func ShellProxySourceCommand(path string) string { return "source " + quoteShellProxy(path) }

func shellProxyCommand(executable, root string) string {
	return "MIHOSH_CONFIG_DIR=" + quoteShellProxy(root) + " " + quoteShellProxy(executable)
}

// ShellProxyScript defines bash/zsh helpers and applies the saved preference to its caller.
func ShellProxyScript(enabled bool, executable, root string) string {
	return shellProxyHelpers(enabled, shellProxyCommand(executable, root), "")
}

func ShellActivationScript(shell string, enabled bool, executable, root string) (string, error) {
	if shell != "bash" && shell != "zsh" {
		return "", errors.New("shell activation requires Bash or Zsh")
	}
	command := "command " + quoteShellProxy(executable)
	proxyCommand := "MIHOSH_CONFIG_DIR=" + quoteShellProxy(root) + " " + command
	wrapper := fmt.Sprintf(`mihosh() {
  if [ "$#" -eq 2 ] && [ "$1" = proxy ] && { [ "$2" = on ] || [ "$2" = off ]; }; then
    local _mihosh_exports
    _mihosh_exports=$(%s proxy "$2") || return
    eval "$_mihosh_exports"
  elif [ "${1-}" = proxy ]; then
    %s "$@"
  else
    %s "$@"
  fi
}
`, proxyCommand, proxyCommand, command)
	return wrapper + shellProxyHelpers(enabled, proxyCommand, "proxy "), nil
}

func shellProxyHelpers(enabled bool, command, prefix string) string {
	var script strings.Builder
	for _, action := range []string{"on", "off"} {
		fmt.Fprintf(&script, `mihosh_proxy_%s() {
  local _mihosh_proxy_exports
  _mihosh_proxy_exports=$(%s %s%s) || return
  eval "$_mihosh_proxy_exports"
}
`, action, command, prefix, action)
	}
	script.WriteString("mihosh_proxy_status() {\n")
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "all_proxy", "no_proxy"} {
		fmt.Fprintf(&script, "  printf '%s=%%s\\n' \"${%s-}\"\n", name, name)
	}
	fmt.Fprintf(&script, `}
mihosh_proxy_ip() {
  %s test --output plain
}
mihosh_proxy_test() {
  %s test "$@"
}
`, command, command)
	action := "off"
	if enabled {
		action = "on"
	}
	fmt.Fprintf(&script, "mihosh_proxy_%s\n", action)
	return script.String()
}

// SaveShellProxy installs a dynamic loader in the shell startup file and returns that file.
func SaveShellProxy(enabled bool) (string, error) {
	shell := filepath.Base(os.Getenv("SHELL"))
	startup, err := shellProxyStartupFile(shell)
	if err != nil {
		return "", err
	}
	root, err := config.GetConfigDir()
	if err != nil {
		return "", err
	}
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	path := filepath.Join(root, "proxy.sh")
	activate := shellProxyCommand(executable, root) + " activate " + shell
	loader := "_mihosh_proxy_script=$(" + activate + ") || return\neval \"$_mihosh_proxy_script\" || {\n  unset _mihosh_proxy_script\n  return 1\n}\nunset _mihosh_proxy_script\n"
	prevProxy, prevProxyExisted, err := readOptionalFile(path)
	if err != nil {
		return "", err
	}
	prevStartup, prevStartupExisted, err := readOptionalFile(startup)
	if err != nil {
		return "", err
	}
	if err := privateWrite(path, []byte(loader)); err != nil {
		return "", err
	}
	if err := installShellProxyStartup(startup, "eval \"$("+activate+")\""); err != nil {
		_ = restoreMergeFile(path, prevProxyExisted, prevProxy)
		return "", err
	}
	if err := config.Update(func(cfg *config.Config) error {
		cfg.ShellProxy = enabled
		return nil
	}); err != nil {
		_ = restoreMergeFile(startup, prevStartupExisted, prevStartup)
		_ = restoreMergeFile(path, prevProxyExisted, prevProxy)
		return "", err
	}
	return startup, nil
}

func shellProxyStartupFile(shell string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch shell {
	case "bash":
		return filepath.Join(home, ".bashrc"), nil
	case "zsh":
		if dir := os.Getenv("ZDOTDIR"); dir != "" {
			home = dir
		}
		return filepath.Join(home, ".zshrc"), nil
	default:
		return "", errors.New("shell proxy startup requires Bash or Zsh (SHELL)")
	}
}

func installShellProxyStartup(startup, activation string) error {
	// Follow dotfile symlinks so an atomic replacement keeps the user's dotfile layout.
	if info, err := os.Lstat(startup); err == nil && info.Mode()&os.ModeSymlink != 0 {
		target, err := filepath.EvalSymlinks(startup)
		if err != nil {
			return err
		}
		startup = target
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	data, err := os.ReadFile(startup)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	const begin, end = "# >>> mihosh proxy >>>", "# <<< mihosh proxy <<<"
	text := string(data)
	newline := "\n"
	if strings.Contains(text, "\r\n") {
		newline = "\r\n"
	}
	block := strings.Join([]string{begin, activation, end}, newline)
	start, finish := strings.Index(text, begin), strings.Index(text, end)
	if start >= 0 || finish >= 0 {
		if start < 0 || finish < start || strings.Count(text, begin) != 1 || strings.Count(text, end) != 1 {
			return errors.New("invalid Mihosh proxy block in shell startup file")
		}
		text = text[:start] + block + text[finish+len(end):]
	} else {
		if len(text) > 0 && !strings.HasSuffix(text, "\n") {
			text += newline
		}
		text += block + newline
	}
	if text == string(data) {
		return nil
	}
	mode := os.FileMode(0600)
	if info, err := os.Stat(startup); err == nil {
		mode = info.Mode().Perm()
	}
	if err := privateWrite(startup, []byte(text)); err != nil {
		return err
	}
	return os.Chmod(startup, mode)
}
