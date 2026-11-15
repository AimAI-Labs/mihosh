package service

import (
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
	command := shellProxyCommand(executable, root)
	var script strings.Builder
	for _, action := range []string{"on", "off"} {
		fmt.Fprintf(&script, `mihosh_proxy_%s() {
  local _mihosh_proxy_exports
  _mihosh_proxy_exports=$(%s %s) || return
  eval "$_mihosh_proxy_exports"
}
`, action, command, action)
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

// SaveShellProxy writes a stable loader; source always reads the latest setting and port.
func SaveShellProxy(enabled bool) (string, error) {
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
	loader := "_mihosh_proxy_script=$(" + shellProxyCommand(executable, root) + " env) || return\neval \"$_mihosh_proxy_script\" || {\n  unset _mihosh_proxy_script\n  return 1\n}\nunset _mihosh_proxy_script\n"
	if err := privateWrite(path, []byte(loader)); err != nil {
		return "", err
	}
	if err := config.Update(func(cfg *config.Config) error {
		cfg.ShellProxy = enabled
		return nil
	}); err != nil {
		return "", err
	}
	return path, nil
}
