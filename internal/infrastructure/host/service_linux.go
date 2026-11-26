package host

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const serviceMarker = "# Managed by Mihosh\n"

type UserService struct {
	Root, UnitDir string
	Run           Runner
}

func NewServices(root string) Services {
	dir, err := os.UserConfigDir()
	unitDir := ""
	if err == nil && filepath.IsAbs(dir) {
		unitDir = filepath.Join(dir, "systemd", "user")
	}
	return &UserService{Root: root, UnitDir: unitDir, Run: Run}
}
func (s *UserService) unit() string { return ServiceName(s.Root) + ".service" }

func (s *UserService) validateUnitDir() error {
	if s.UnitDir == "" || !filepath.IsAbs(s.UnitDir) {
		return errors.New("systemd user unit directory is unavailable")
	}
	return nil
}

// systemd specifiers and environment substitutions are escaped independently of quoting.
func unitQuote(value string) (string, error) {
	if strings.ContainsAny(value, "\x00\r\n") {
		return "", errors.New("invalid service path")
	}
	value = strings.ReplaceAll(strings.ReplaceAll(value, "%", "%%"), "$", "$$")
	return strconv.Quote(value), nil
}
func (s *UserService) owned() error {
	if err := s.validateUnitDir(); err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(s.UnitDir, s.unit()))
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("background service is not installed; install it first: %w", err)
		}
		return err
	}
	if !strings.HasPrefix(string(data), serviceMarker) {
		return errors.New("service unit is not owned by Mihosh")
	}
	return nil
}
func (s *UserService) Install(ctx context.Context, executable string) error {
	if err := s.validateUnitDir(); err != nil {
		return err
	}
	path := filepath.Join(s.UnitDir, s.unit())
	if _, err := os.Stat(path); err == nil {
		if err = s.owned(); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	exe, err := unitQuote(executable)
	if err != nil {
		return err
	}
	root, err := unitQuote(s.Root)
	if err != nil {
		return err
	}
	data := serviceMarker + fmt.Sprintf("[Unit]\nDescription=Mihosh managed core\nStartLimitIntervalSec=60\nStartLimitBurst=3\n[Service]\nType=notify\nNotifyAccess=main\nExecStart=%s core service run --home %s\nRestart=on-failure\nRestartSec=3\nKillMode=mixed\nTimeoutStartSec=60\nTimeoutStopSec=45\nUMask=0077\n[Install]\nWantedBy=default.target\n", exe, root)
	if err = os.MkdirAll(s.UnitDir, 0700); err != nil {
		return err
	}
	if err = os.WriteFile(path, []byte(data), 0600); err != nil {
		return err
	}
	_, err = s.Run(ctx, "systemctl", "--user", "daemon-reload")
	return err
}
func (s *UserService) Action(ctx context.Context, action string) error {
	if err := s.owned(); err != nil {
		return err
	}
	switch action {
	case "start", "stop", "restart", "enable", "disable":
		_, err := s.Run(ctx, "systemctl", "--user", action, s.unit())
		return err
	case "boot":
		if _, err := s.Run(ctx, "loginctl", "enable-linger", strconv.Itoa(os.Getuid())); err != nil {
			return fmt.Errorf("enable user lingering for boot startup: %w", err)
		}
		_, err := s.Run(ctx, "systemctl", "--user", "enable", s.unit())
		return err
	case "uninstall":
		if _, err := s.Run(ctx, "systemctl", "--user", "stop", s.unit()); err != nil {
			return err
		}
		if _, err := s.Run(ctx, "systemctl", "--user", "disable", s.unit()); err != nil {
			return err
		}
		if err := os.Remove(filepath.Join(s.UnitDir, s.unit())); err != nil {
			return err
		}
		_, err := s.Run(ctx, "systemctl", "--user", "daemon-reload")
		return err
	}
	return errors.New("unsupported service action")
}
func (s *UserService) Status(ctx context.Context) (ServiceState, error) {
	if err := s.owned(); errors.Is(err, os.ErrNotExist) {
		return ServiceState{}, nil
	} else if err != nil {
		return ServiceState{}, err
	}
	out, err := s.Run(ctx, "systemctl", "--user", "show", s.unit(), "--property=ActiveState,UnitFileState")
	state := ServiceState{Installed: true}
	if err != nil {
		return state, err
	}
	for _, line := range strings.Split(string(out), "\n") {
		if line == "ActiveState=active" {
			state.Running = true
		}
		if line == "UnitFileState=enabled" {
			state.Enabled = true
		}
	}
	return state, nil
}
func RunService(ctx context.Context, _ string, run func(context.Context, func()) error) error {
	return run(ctx, func() {
		address := os.Getenv("NOTIFY_SOCKET")
		if address == "" {
			return
		}
		if address[0] == '@' {
			address = "\x00" + address[1:]
		}
		conn, err := net.Dial("unixgram", address)
		if err == nil {
			defer func() { _ = conn.Close() }()
			_, _ = conn.Write([]byte("READY=1"))
		}
	})
}
