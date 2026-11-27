package host

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type DesktopNetwork struct{ Run Runner }

func NewNetwork() Network { return &DesktopNetwork{} }

func (n *DesktopNetwork) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if n.Run != nil {
		return n.Run(ctx, name, args...)
	}
	return Run(ctx, name, args...)
}

func (n *DesktopNetwork) runPrivileged(ctx context.Context, name string, args ...string) ([]byte, error) {
	if n.Run != nil {
		return n.Run(ctx, name, args...)
	}
	return runWithTUNCapabilities(ctx, name, args...)
}

func runWithTUNCapabilities(ctx context.Context, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	if os.Geteuid() != 0 && hasEffectiveCap(unix.CAP_NET_ADMIN) {
		ConfigureTUNProcess(cmd)
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w", name, err)
	}
	return out, nil
}

func currentCapabilities() ([2]unix.CapUserData, bool) {
	var data [2]unix.CapUserData
	header := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	if unix.Capget(&header, &data[0]) != nil {
		return data, false
	}
	return data, true
}

func hasEffectiveCap(capability uint) bool {
	data, ok := currentCapabilities()
	if !ok {
		return false
	}
	return data[capability/32].Effective&(1<<(capability%32)) != 0
}

func hasPermittedCap(capability uint) bool {
	data, ok := currentCapabilities()
	if !ok {
		return false
	}
	return data[capability/32].Permitted&(1<<(capability%32)) != 0
}

var desktopKeys = []string{"http.host", "http.port", "http.use-authentication", "https.host", "https.port", "socks.host", "socks.port", "ignore-hosts", "mode"}

func settingKey(key string) (string, string) {
	schema := "org.gnome.system.proxy"
	if group, name, ok := strings.Cut(key, "."); ok {
		return schema + "." + group, name
	}
	return schema, key
}
func (n *DesktopNetwork) ReadProxy(ctx context.Context) (ProxySettings, error) {
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
		info, err := os.Stat(filepath.Join(runtimeDir, "bus"))
		if runtimeDir == "" || err != nil || info.Mode()&os.ModeSocket == 0 {
			return nil, errors.New("GNOME system proxy requires a desktop D-Bus session; terminal on/off remains available")
		}
	}
	settings := ProxySettings{}
	for _, key := range desktopKeys {
		schema, name := settingKey(key)
		value, err := n.run(ctx, "gsettings", "get", schema, name)
		if err != nil {
			if errors.Is(err, exec.ErrNotFound) {
				return nil, errors.New("GNOME system proxy requires gsettings (GNOME desktop); on headless or non-GNOME Linux, use terminal proxy (mihosh proxy on/off) or TUN instead")
			}
			return nil, fmt.Errorf("GNOME proxy settings unavailable: %w", err)
		}
		settings[key] = Setting{Data: []byte(strings.TrimSpace(string(value))), Exists: true}
	}
	return settings, nil
}
func (*DesktopNetwork) ProxyTarget(port int) ProxySettings {
	result := ProxySettings{}
	for _, key := range desktopKeys {
		value := "'127.0.0.1'"
		if strings.HasSuffix(key, ".port") {
			value = strconv.Itoa(port)
		}
		if key == "mode" {
			value = "'manual'"
		}
		if key == "ignore-hosts" {
			value = "['localhost', '127.0.0.1', '::1']"
		}
		if key == "http.use-authentication" {
			value = "false"
		}
		result[key] = Setting{Data: []byte(value), Exists: true}
	}
	return result
}
func (n *DesktopNetwork) WriteProxy(ctx context.Context, settings ProxySettings) error {
	keys := make([]string, 0, len(settings))
	for key := range settings {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	// Switch mode only after all endpoint values are ready.
	if _, ok := settings["mode"]; ok {
		for i, key := range keys {
			if key == "mode" {
				keys = append(append(keys[:i:i], keys[i+1:]...), key)
				break
			}
		}
	}
	for _, key := range keys {
		schema, name := settingKey(key)
		if _, err := n.run(ctx, "gsettings", "set", schema, name, string(settings[key].Data)); err != nil {
			if errors.Is(err, exec.ErrNotFound) {
				return errors.New("GNOME system proxy requires gsettings (GNOME desktop); on headless or non-GNOME Linux, use terminal proxy (mihosh proxy on/off) or TUN instead")
			}
			return err
		}
	}
	return nil
}
func (n *DesktopNetwork) rules(ctx context.Context, family string) ([]map[string]any, error) {
	out, err := n.run(ctx, "ip", family, "-json", "rule", "show")
	if err != nil {
		return nil, err
	}
	var rules []map[string]any
	err = json.Unmarshal(out, &rules)
	return rules, err
}
func (n *DesktopNetwork) CheckTUN(ctx context.Context, _ string, t TUN) error {
	if _, err := os.Stat("/dev/net/tun"); err != nil {
		return errors.New("TUN requires /dev/net/tun")
	}
	if !hasEffectiveCap(unix.CAP_NET_ADMIN) {
		exe, err := os.Executable()
		if err != nil || exe == "" {
			exe = "mihosh"
		} else if resolved, err := filepath.EvalSymlinks(exe); err == nil && resolved != "" {
			exe = resolved
		}
		return fmt.Errorf("TUN requires CAP_NET_ADMIN on Mihosh (needed to spawn mihomo with TUN and clean up routes). Run with sudo or grant capabilities: sudo setcap cap_net_admin,cap_net_raw+ep %s", exe)
	}
	if _, err := net.InterfaceByName(t.Device); err == nil {
		return errors.New("managed TUN interface already exists; recover it before enabling TUN")
	}
	for _, family := range []string{"-4", "-6"} {
		rules, err := n.rules(ctx, family)
		if err != nil {
			if family == "-6" && isIPv6Unsupported(err) {
				continue
			}
			return err
		}
		for _, rule := range rules {
			priority, _ := rule["priority"].(float64)
			if int(priority) >= t.Rule && int(priority) <= t.Rule+10 || fmt.Sprint(rule["table"]) == strconv.Itoa(t.Table) {
				return errors.New("managed TUN routing table or rule range is already in use")
			}
		}
	}
	return nil
}
func (*DesktopNetwork) CaptureTUN(_ context.Context, t TUN) (TUN, error) { return captureTUN(t) }
func (n *DesktopNetwork) CleanupTUN(ctx context.Context, t TUN) error {
	if i, err := net.InterfaceByName(t.Device); err == nil && t.Index != 0 && i.Index != t.Index {
		return errors.New("TUN interface identity changed; refusing to remove unrelated routes")
	}
	var errs []error
	for _, family := range []string{"-4", "-6"} {
		rules, err := n.rules(ctx, family)
		if err != nil {
			if family == "-6" && isIPv6Unsupported(err) {
				continue
			}
			errs = append(errs, err)
			continue
		}
		for _, rule := range rules {
			priority, _ := rule["priority"].(float64)
			if int(priority) >= t.Rule && int(priority) <= t.Rule+10 {
				if _, err = n.runPrivileged(ctx, "ip", family, "rule", "del", "priority", strconv.Itoa(int(priority))); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}
	if _, err := net.InterfaceByName(t.Device); err == nil {
		// Interface-scoped resolver state is best-effort reverted; deleting the link below
		// drops interface-scoped state and must not be blocked when systemd-resolved is inactive.
		if _, err := exec.LookPath("resolvectl"); err == nil {
			_, _ = n.run(ctx, "resolvectl", "revert", t.Device)
		}
		if _, err = n.runPrivileged(ctx, "ip", "link", "delete", "dev", t.Device); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func isIPv6Unsupported(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		msg += "\n" + string(exitError.Stderr)
	}
	msg = strings.ToLower(msg)
	return strings.Contains(msg, "address family not supported") || strings.Contains(msg, "eafnosupport")
}
func configureGuard(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if hasEffectiveCap(unix.CAP_NET_ADMIN) {
		ConfigureTUNProcess(cmd)
	}
}
func ConfigureTUNProcess(cmd *exec.Cmd) {
	if os.Geteuid() == 0 || !hasPermittedCap(unix.CAP_NET_ADMIN) {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.AmbientCaps = []uintptr{unix.CAP_NET_ADMIN}
	if hasPermittedCap(unix.CAP_NET_RAW) {
		cmd.SysProcAttr.AmbientCaps = append(cmd.SysProcAttr.AmbientCaps, unix.CAP_NET_RAW)
	}
}
