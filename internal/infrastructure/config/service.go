package config

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/AimAI-Labs/mihosh/pkg/i18n"
)

// RestartMihomoService 通过 systemctl 重启 mihomo 服务（仅 Linux）。
func RestartMihomoService() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("%s", i18n.T("config.service.err_systemctl_linux"))
	}
	return exec.Command("sudo", "systemctl", "restart", "mihomo").Run()
}
