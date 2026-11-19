package service

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/stretchr/testify/require"
)

const fakeShellProxy = `#!/bin/sh
printf '%s\n' "$MIHOSH_CONFIG_DIR" >> "$MIHOSH_SHELL_ROOT_LOG"
if [ "$1" = proxy ]; then shift; fi
case "$1" in
on)
  if [ "${MIHOSH_SHELL_FAIL-}" = 1 ]; then
    printf '%s\n' "export HTTP_PROXY='must-not-apply'"
    exit 23
  fi
  port=$(cat "$MIHOSH_SHELL_PORT_FILE") || exit
  for name in HTTP_PROXY HTTPS_PROXY ALL_PROXY http_proxy https_proxy all_proxy; do
    printf "export %s='http://127.0.0.1:%s'\n" "$name" "$port"
  done
  printf '%s\n' "export NO_PROXY='localhost,127.0.0.1,::1'" "export no_proxy='localhost,127.0.0.1,::1'"
  ;;
off)
  printf '%s\n' 'unset HTTP_PROXY HTTPS_PROXY ALL_PROXY NO_PROXY http_proxy https_proxy all_proxy no_proxy'
  ;;
test)
  printf 'arg:%s\n' "$@" >> "$MIHOSH_SHELL_ARGS_FILE"
  ;;
env|activate)
  cat "$MIHOSH_SHELL_ENV_SCRIPT"
  ;;
*) exit 24 ;;
esac
`

type shellProxyFixture struct {
	executable, root, port, roots, args, envScript, dir string
	env                                                 []string
}

func newShellProxyFixture(t *testing.T) shellProxyFixture {
	t.Helper()
	dir := t.TempDir()
	f := shellProxyFixture{
		dir: dir, executable: filepath.Join(dir, "mihosh ' ; $(touch injected); #"),
		root: filepath.Join(dir, "config ' ; $(touch injected); #"),
		port: filepath.Join(dir, "port"), roots: filepath.Join(dir, "roots"),
		args: filepath.Join(dir, "args"), envScript: filepath.Join(dir, "env-script"),
	}
	require.NoError(t, os.WriteFile(f.executable, []byte(fakeShellProxy), 0700))
	require.NoError(t, os.WriteFile(f.port, []byte("7890\n"), 0600))
	f.env = append(os.Environ(), "MIHOSH_SHELL_PORT_FILE="+f.port, "MIHOSH_SHELL_ROOT_LOG="+f.roots, "MIHOSH_SHELL_ARGS_FILE="+f.args, "MIHOSH_SHELL_ENV_SCRIPT="+f.envScript, "MIHOSH_SHELL_FAIL=")
	return f
}

func (f shellProxyFixture) run(t *testing.T, shell, script string) string {
	t.Helper()
	cmd := exec.Command(shell, "-c", script)
	cmd.Dir, cmd.Env = f.dir, f.env
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s: %s", shell, output)
	require.NoFileExists(t, filepath.Join(f.dir, "injected"))
	return string(output)
}

func TestShellProxyHelpersApplyDynamicSettingsAndForwardArguments(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake executable uses a POSIX shebang")
	}
	for _, name := range []string{"bash", "zsh"} {
		t.Run(name, func(t *testing.T) {
			shell, err := exec.LookPath(name)
			if err != nil {
				t.Skip("shell unavailable")
			}
			f := newShellProxyFixture(t)
			script := ShellProxyScript(true, f.executable, f.root) + `
mihosh_proxy_status
printf '%s\n' 7891 > "$MIHOSH_SHELL_PORT_FILE"
mihosh_proxy_on || exit
printf 'changed:%s\n' "$HTTP_PROXY"
mihosh_proxy_ip || exit
mihosh_proxy_test node 'HK space; $(touch injected)' --output json || exit
mihosh_proxy_off || exit
mihosh_proxy_status
export HTTP_PROXY='existing-proxy' MIHOSH_SHELL_FAIL=1
mihosh_proxy_on
printf 'failed:%s:%s\n' "$?" "$HTTP_PROXY"
`
			output := f.run(t, shell, script)
			for _, variable := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
				require.Contains(t, output, variable+"=http://127.0.0.1:7890\n")
				require.Contains(t, output, variable+"=\n")
			}
			for _, variable := range []string{"NO_PROXY", "no_proxy"} {
				require.Contains(t, output, variable+"=localhost,127.0.0.1,::1\n")
				require.Contains(t, output, variable+"=\n")
			}
			require.Contains(t, output, "changed:http://127.0.0.1:7891\n")
			require.Contains(t, output, "failed:23:existing-proxy\n")
			args, err := os.ReadFile(f.args)
			require.NoError(t, err)
			require.Contains(t, string(args), "arg:test\narg:--output\narg:plain\n")
			require.Contains(t, string(args), "arg:test\narg:node\narg:HK space; $(touch injected)\narg:--output\narg:json\n")
			roots, err := os.ReadFile(f.roots)
			require.NoError(t, err)
			for _, actual := range strings.Split(strings.TrimSpace(string(roots)), "\n") {
				require.Equal(t, f.root, actual)
			}
		})
	}
}

func TestSaveShellProxyPersistsPreferenceAndPrivateDynamicLoader(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("SHELL", "/bin/bash")
	root := filepath.Join(t.TempDir(), "configuration")
	t.Setenv("MIHOSH_CONFIG_DIR", root)
	startup, err := SaveShellProxy(true)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(home, ".bashrc"), startup)
	path := filepath.Join(root, "proxy.sh")
	rc, err := os.ReadFile(startup)
	require.NoError(t, err)
	require.Contains(t, string(rc), "eval \"$(MIHOSH_CONFIG_DIR=")
	require.Contains(t, string(rc), " activate bash)\"")
	cfg, err := config.Load()
	require.NoError(t, err)
	require.True(t, cfg.ShellProxy)
	loader, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(loader), " activate bash) || return\n")
	require.NotContains(t, string(loader), "export ")
	require.NotContains(t, string(loader), "http://")
	info, err := os.Stat(path)
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		require.Equal(t, os.FileMode(0600), info.Mode().Perm())
		rootInfo, err := os.Stat(root)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0700), rootInfo.Mode().Perm())
	}
	_, err = SaveShellProxy(false)
	require.NoError(t, err)
	cfg, err = config.Load()
	require.NoError(t, err)
	require.False(t, cfg.ShellProxy)
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, loader, after, "loader must read the setting at source time")
	after, err = os.ReadFile(startup)
	require.NoError(t, err)
	require.Equal(t, rc, after, "saving again must not duplicate the startup block")
	if runtime.GOOS == "windows" {
		return
	}
	shell, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash unavailable for source check")
	}
	f := newShellProxyFixture(t)
	f.root = root
	executable, err := os.Executable()
	require.NoError(t, err)
	boundLoader := strings.ReplaceAll(string(loader), quoteShellProxy(executable), quoteShellProxy(f.executable))
	copyPath := filepath.Join(f.dir, "proxy.sh")
	require.NoError(t, os.WriteFile(copyPath, []byte(boundLoader), 0600))
	for _, enabled := range []bool{false, true} {
		require.NoError(t, os.WriteFile(f.envScript, []byte(ShellProxyScript(enabled, f.executable, root)), 0600))
		output := f.run(t, shell, ". "+quoteShellProxy(copyPath)+"\nmihosh_proxy_status\n")
		want := "HTTP_PROXY=\n"
		if enabled {
			want = "HTTP_PROXY=http://127.0.0.1:7890\n"
		}
		require.Contains(t, output, want)
	}
	require.NoError(t, os.WriteFile(f.envScript, []byte(ShellProxyScript(true, f.executable, root)), 0600))
	f.env = append(f.env, "MIHOSH_SHELL_FAIL=1")
	output := f.run(t, shell, ". "+quoteShellProxy(copyPath)+"\nprintf 'source-failed:%s\\n' \"$?\"\n")
	require.Contains(t, output, "source-failed:1\n")
}

func TestActivationWrapsProxyCommandsWithoutChangingOtherArguments(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake executable uses a POSIX shebang")
	}
	for _, name := range []string{"bash", "zsh"} {
		t.Run(name, func(t *testing.T) {
			shell, err := exec.LookPath(name)
			if err != nil {
				t.Skip("shell unavailable")
			}
			f := newShellProxyFixture(t)
			script, err := ShellActivationScript(name, false, f.executable, f.root)
			require.NoError(t, err)
			output := f.run(t, shell, script+`
mihosh proxy on || exit
printf 'enabled:%s\n' "$HTTP_PROXY"
mihosh proxy test --output json || exit
mihosh proxy off || exit
printf 'disabled:%s\n' "${HTTP_PROXY-}"
export HTTP_PROXY=existing MIHOSH_SHELL_FAIL=1
mihosh proxy on
printf 'failed:%s:%s\n' "$?" "$HTTP_PROXY"
`)
			require.Contains(t, output, "enabled:http://127.0.0.1:7890\n")
			require.Contains(t, output, "disabled:\n")
			require.Contains(t, output, "failed:23:existing\n")
			args, err := os.ReadFile(f.args)
			require.NoError(t, err)
			require.Equal(t, "arg:test\narg:--output\narg:json\n", string(args))
		})
	}
}

func TestShellProxyStartupPreservesDotfilesAndUpdatesLoader(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		t.Run(strings.ReplaceAll(newline, "\n", "LF"), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ".zshrc")
			original := "export MY_SETTING=keep" + newline
			require.NoError(t, os.WriteFile(path, []byte(original), 0640))
			require.NoError(t, installShellProxyStartup(path, ShellProxySourceCommand("/old/proxy.sh")))
			activation := "eval \"$(" + quoteShellProxy("/new ' $(touch injected)/mihosh") + " activate zsh)\""
			require.NoError(t, installShellProxyStartup(path, activation))
			data, err := os.ReadFile(path)
			require.NoError(t, err)
			require.True(t, strings.HasPrefix(string(data), original))
			require.Equal(t, 1, strings.Count(string(data), "# >>> mihosh proxy >>>"))
			require.NotContains(t, string(data), "/old/proxy.sh")
			require.Contains(t, string(data), activation)
			info, err := os.Stat(path)
			require.NoError(t, err)
			if runtime.GOOS != "windows" {
				require.Equal(t, os.FileMode(0640), info.Mode().Perm())
			}
		})
	}
	if runtime.GOOS != "windows" {
		dir := t.TempDir()
		target, link := filepath.Join(dir, "dotfile"), filepath.Join(dir, ".bashrc")
		require.NoError(t, os.WriteFile(target, []byte("# keep\n"), 0600))
		require.NoError(t, os.Symlink(target, link))
		require.NoError(t, installShellProxyStartup(link, ShellProxySourceCommand("/proxy.sh")))
		actual, err := os.Readlink(link)
		require.NoError(t, err)
		require.Equal(t, target, actual)
		data, err := os.ReadFile(target)
		require.NoError(t, err)
		require.Contains(t, string(data), "# keep\n")
		require.Contains(t, string(data), "source '/proxy.sh'")
	}
}

func TestShellProxyStartupRejectsInvalidFilesAndUnsupportedShell(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".bashrc")
	for _, data := range []string{"# >>> mihosh proxy >>>\nkeep", "# <<< mihosh proxy <<<\nkeep"} {
		require.NoError(t, os.WriteFile(path, []byte(data), 0600))
		require.Error(t, installShellProxyStartup(path, "/proxy.sh"))
		after, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, data, string(after))
	}
	t.Setenv("SHELL", "/bin/fish")
	t.Setenv("MIHOSH_CONFIG_DIR", filepath.Join(t.TempDir(), "unused"))
	_, err := SaveShellProxy(true)
	require.ErrorContains(t, err, "Bash or Zsh")
	_, err = config.Load()
	require.ErrorIs(t, err, config.ErrConfigNotFound)
}

func TestShellProxyZshStartupUsesZdotdir(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")
	t.Setenv("ZDOTDIR", t.TempDir())
	path, err := shellProxyStartupFile("zsh")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(os.Getenv("ZDOTDIR"), ".zshrc"), path)
}
