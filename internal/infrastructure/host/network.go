package host

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
)

type Setting struct {
	Kind   uint32 `json:"kind"`
	Data   []byte `json:"data"`
	Exists bool   `json:"exists"`
}
type ProxySettings map[string]Setting
type TUN struct {
	Device string `json:"device"`
	Table  int    `json:"table"`
	Rule   int    `json:"rule"`
	Index  int    `json:"index"`
}
type Network interface {
	ReadProxy(context.Context) (ProxySettings, error)
	ProxyTarget(int) ProxySettings
	WriteProxy(context.Context, ProxySettings) error
	CheckTUN(context.Context, string, TUN) error
	CaptureTUN(context.Context, TUN) (TUN, error)
	CleanupTUN(context.Context, TUN) error
}

func TUNIdentity(root string) TUN {
	digest := sha256.Sum256([]byte(root))
	n := binary.LittleEndian.Uint32(digest[:4])
	return TUN{Device: fmt.Sprintf("mhs%x", digest[:4]), Table: 100000 + int(n%100000), Rule: 10000 + int(n%1000)*12}
}
func captureTUN(t TUN) (TUN, error) {
	i, err := net.InterfaceByName(t.Device)
	if err != nil {
		return t, fmt.Errorf("managed TUN interface %s is not ready", t.Device)
	}
	t.Index = i.Index
	return t, nil
}
func GuardCommand(executable, root string, service bool) *exec.Cmd {
	// A setsid process still belongs to systemd's service cgroup. Start the guard
	// in its own transient user unit so manager cleanup cannot kill both owners.
	if service && runtime.GOOS == "linux" && os.Getenv("INVOCATION_ID") != "" {
		args := []string{"--user", "--unit=" + ServiceName(root) + "-guard", "--collect", "--property=Type=exec", "--property=TimeoutStopSec=30"}
		for _, key := range []string{"DBUS_SESSION_BUS_ADDRESS", "XDG_RUNTIME_DIR", "DISPLAY", "WAYLAND_DISPLAY"} {
			if value := os.Getenv(key); value != "" {
				args = append(args, "--setenv="+key+"="+value)
			}
		}
		args = append(args, "--", executable, "core", "network-guard", "--home", root)
		return exec.Command("systemd-run", args...)
	}
	cmd := exec.Command(executable, "core", "network-guard", "--home", root)
	configureGuard(cmd)
	return cmd
}
