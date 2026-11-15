package config

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/AimAI-Labs/mihosh/pkg/i18n"
)

func restartMihomoCommand() *exec.Cmd {
	// Never attach tty or prompt for a sudo password under TUI raw mode.
	return exec.Command("sudo", "-n", "systemctl", "restart", "mihomo")
}

// RestartMihomoService 通过非交互 sudo systemctl 重启 mihomo 服务（仅 Linux）。
func RestartMihomoService() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("%s", i18n.T("config.service.err_systemctl_linux"))
	}
	cmd := restartMihomoCommand()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}
