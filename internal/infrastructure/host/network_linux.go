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

	"golang.org/x/sys/unix"
)

type DesktopNetwork struct{ Run Runner }

func NewNetwork() Network { return &DesktopNetwork{Run: Run} }

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
		value, err := n.Run(ctx, "gsettings", "get", schema, name)
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
		if _, err := n.Run(ctx, "gsettings", "set", schema, name, string(settings[key].Data)); err != nil {
			if errors.Is(err, exec.ErrNotFound) {
				return errors.New("GNOME system proxy requires gsettings (GNOME desktop); on headless or non-GNOME Linux, use terminal proxy (mihosh proxy on/off) or TUN instead")
			}
			return err
		}
	}
	return nil
}
func (n *DesktopNetwork) rules(ctx context.Context, family string) ([]map[string]any, error) {
	out, err := n.Run(ctx, "ip", family, "-json", "rule", "show")
	if err != nil {
		return nil, err
	}
	var rules []map[string]any
	err = json.Unmarshal(out, &rules)
	return rules, err
}
func (n *DesktopNetwork) CheckTUN(ctx context.Context, binaryPath string, t TUN) error {
	if _, err := os.Stat("/dev/net/tun"); err != nil {
		return errors.New("TUN requires /dev/net/tun")
	}
	var data unix.CapUserData
	header := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_1}
	privileged := unix.Capget(&header, &data) == nil && data.Effective&(1<<unix.CAP_NET_ADMIN) != 0
	if !privileged {
		return fmt.Errorf("TUN requires CAP_NET_ADMIN on the Mihosh coordinator and recovery guard; run elevated or grant cap_net_admin,cap_net_raw+ep to the installed Mihosh executable (selected kernel: %s)", binaryPath)
	}
	if _, err := net.InterfaceByName(t.Device); err == nil {
		return errors.New("managed TUN interface already exists; recover it before enabling TUN")
	}
	for _, family := range []string{"-4", "-6"} {
		rules, err := n.rules(ctx, family)
		if err != nil {
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
	for _, family := range []string{"-4", "-6"} {
		rules, err := n.rules(ctx, family)
		if err != nil {
			return err
		}
		for _, rule := range rules {
			priority, _ := rule["priority"].(float64)
			if int(priority) >= t.Rule && int(priority) <= t.Rule+10 {
				if _, err = n.Run(ctx, "ip", family, "rule", "del", "priority", strconv.Itoa(int(priority))); err != nil {
					return err
				}
			}
		}
	}
	if _, err := net.InterfaceByName(t.Device); err == nil {
		// Interface-scoped resolver state is reverted before deleting our own device.
		if _, err := exec.LookPath("resolvectl"); err == nil {
			if _, err = n.Run(ctx, "resolvectl", "revert", t.Device); err != nil {
				return err
			}
		}
		_, err = n.Run(ctx, "ip", "link", "delete", "dev", t.Device)
		return err
	}
	return nil
}
func configureGuard(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	var data unix.CapUserData
	header := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_1}
	if unix.Capget(&header, &data) == nil && data.Effective&(1<<unix.CAP_NET_ADMIN) != 0 {
		ConfigureTUNProcess(cmd)
	}
}
func ConfigureTUNProcess(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.AmbientCaps = []uintptr{unix.CAP_NET_ADMIN}
	var data unix.CapUserData
	header := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_1}
	if unix.Capget(&header, &data) == nil && data.Permitted&(1<<unix.CAP_NET_RAW) != 0 {
		cmd.SysProcAttr.AmbientCaps = append(cmd.SysProcAttr.AmbientCaps, unix.CAP_NET_RAW)
	}
}
func GracefulCancel(cmd *exec.Cmd) {
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
}
