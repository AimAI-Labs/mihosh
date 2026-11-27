package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/fsutil"
)

func quoteShellProxy(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func ShellProxySourceCommand(path string) string { return "source " + quoteShellProxy(path) }

func shellProxyCommand(executable, root string) string {
	return "MIHOSH_CONFIG_DIR=" + quoteShellProxy(root) + " " + quoteShellProxy(executable)
}

func stableShellProxyExecutable(executable string) string {
	if !isHomebrewUpdateTarget(executable) {
		return executable
	}
	running, err := os.Stat(executable)
	if err != nil {
		return executable
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if !filepath.IsAbs(dir) {
			continue
		}
		candidate := filepath.Join(dir, "mihosh")
		if isHomebrewUpdateTarget(candidate) {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && os.SameFile(running, info) {
			return candidate
		}
	}
	return executable
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
	executable = stableShellProxyExecutable(executable)
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	path := filepath.Join(root, "proxy.sh")
	activate := shellProxyCommand(executable, root) + " activate " + shell
	guard := "[ -x " + quoteShellProxy(executable) + " ]"
	loader := "if " + guard + "; then\n_mihosh_proxy_script=$(" + activate + ") || return\neval \"$_mihosh_proxy_script\" || {\n  unset _mihosh_proxy_script\n  return 1\n}\nunset _mihosh_proxy_script\nfi\n"
	prevProxy, prevProxyExisted, err := readOptionalFile(path)
	if err != nil {
		return "", err
	}
	if err := privateWrite(path, []byte(loader)); err != nil {
		return "", err
	}
	activation := "if " + guard + "; then eval \"$(" + activate + ")\"; fi"
	rollbackStartup, err := installShellProxyStartup(startup, activation)
	if err != nil {
		return "", errors.Join(err, restoreMergeFile(path, prevProxyExisted, prevProxy))
	}
	if err := config.Update(func(cfg *config.Config) error {
		cfg.ShellProxy = enabled
		return nil
	}); err != nil {
		return "", errors.Join(err, rollbackStartup(), restoreMergeFile(path, prevProxyExisted, prevProxy))
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

const (
	shellProxyBlockBegin = "# >>> mihosh proxy >>>"
	shellProxyBlockEnd   = "# <<< mihosh proxy <<<"
)

func installShellProxyStartup(startup, activation string) (func() error, error) {
	// Follow dotfile symlinks so an atomic replacement keeps the user's dotfile layout.
	target, err := resolveDotfileTarget(startup)
	if err != nil {
		return nil, err
	}
	startup = target
	data, existed, err := readOptionalFile(startup)
	if err != nil {
		return nil, err
	}
	text := string(data)
	newline := "\n"
	if strings.Contains(text, "\r\n") {
		newline = "\r\n"
	}
	block := strings.Join([]string{shellProxyBlockBegin, activation, shellProxyBlockEnd}, newline)
	start, finish := strings.Index(text, shellProxyBlockBegin), strings.Index(text, shellProxyBlockEnd)
	if start >= 0 || finish >= 0 {
		if start < 0 || finish < start || strings.Count(text, shellProxyBlockBegin) != 1 || strings.Count(text, shellProxyBlockEnd) != 1 {
			return nil, errors.New("invalid Mihosh proxy block in shell startup file")
		}
		text = text[:start] + block + text[finish+len(shellProxyBlockEnd):]
	} else {
		if len(text) > 0 && !strings.HasSuffix(text, "\n") {
			text += newline
		}
		text += block + newline
	}
	if text == string(data) {
		return func() error { return nil }, nil
	}
	mode := os.FileMode(0600)
	if existed {
		info, err := os.Stat(startup)
		if err != nil {
			return nil, err
		}
		mode = info.Mode().Perm()
	}
	opts := fsutil.WriteOptions{Perm: mode}
	if err := fsutil.WriteFileAtomic(startup, []byte(text), opts); err != nil {
		return nil, err
	}
	return func() error {
		if !existed {
			return restoreMergeFile(startup, false, nil)
		}
		return fsutil.WriteFileAtomic(startup, data, opts)
	}, nil
}

func resolveDotfileTarget(path string) (string, error) {
	info, err := os.Lstat(path)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return filepath.EvalSymlinks(path)
	}
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return path, nil
}

func shellProxyStartupCandidates(home string) []string {
	var raw []string
	if home != "" {
		raw = append(raw, filepath.Join(home, ".bashrc"), filepath.Join(home, ".zshrc"))
	}
	if dir := os.Getenv("ZDOTDIR"); dir != "" {
		raw = append(raw, filepath.Join(dir, ".zshrc"))
	}
	seen := make(map[string]bool, len(raw))
	var out []string
	for _, candidate := range raw {
		cleaned := filepath.Clean(candidate)
		key := cleaned
		if target, err := resolveDotfileTarget(cleaned); err == nil {
			key = filepath.Clean(target)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, cleaned)
	}
	return out
}

// FindShellProxyStartupFiles returns bash/zsh startup files that contain a Mihosh proxy block.
func FindShellProxyStartupFiles(home string) []string {
	var found []string
	for _, candidate := range shellProxyStartupCandidates(home) {
		target, err := resolveDotfileTarget(candidate)
		if err != nil {
			continue
		}
		data, existed, err := readOptionalFile(target)
		if err != nil || !existed {
			continue
		}
		text := string(data)
		if strings.Contains(text, shellProxyBlockBegin) || strings.Contains(text, shellProxyBlockEnd) {
			found = append(found, candidate)
		}
	}
	return found
}

// RemoveShellProxyStartupInHome removes the managed Mihosh proxy block from bash/zsh startup files.
func RemoveShellProxyStartupInHome(home string) ([]string, error) {
	var cleaned []string
	var errs []error
	for _, candidate := range shellProxyStartupCandidates(home) {
		removed, err := removeShellProxyStartup(candidate)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", candidate, err))
			continue
		}
		if removed {
			cleaned = append(cleaned, candidate)
		}
	}
	return cleaned, errors.Join(errs...)
}

func removeShellProxyStartup(startup string) (bool, error) {
	target, err := resolveDotfileTarget(startup)
	if err != nil {
		return false, err
	}
	data, existed, err := readOptionalFile(target)
	if err != nil || !existed {
		return false, err
	}
	text := string(data)
	start, finish := strings.Index(text, shellProxyBlockBegin), strings.Index(text, shellProxyBlockEnd)
	if start < 0 && finish < 0 {
		return false, nil
	}
	if start < 0 || finish < start || strings.Count(text, shellProxyBlockBegin) != 1 || strings.Count(text, shellProxyBlockEnd) != 1 {
		return false, errors.New("invalid Mihosh proxy block in shell startup file")
	}
	after := finish + len(shellProxyBlockEnd)
	switch {
	case strings.HasPrefix(text[after:], "\r\n"):
		after += 2
	case strings.HasPrefix(text[after:], "\n"):
		after++
	}
	updated := text[:start] + text[after:]
	info, err := os.Stat(target)
	if err != nil {
		return false, err
	}
	if err := fsutil.WriteFileAtomic(target, []byte(updated), fsutil.WriteOptions{Perm: info.Mode().Perm()}); err != nil {
		return false, err
	}
	return true, nil
}
