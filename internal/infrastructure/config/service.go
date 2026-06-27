package config

import (
	"fmt"
	"os/exec"
	"runtime"
)

// RestartMihomoService 通过 systemctl 重启 mihomo 服务（仅 Linux）。
func RestartMihomoService() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("systemctl restart 仅支持 Linux")
	}
	return exec.Command("sudo", "systemctl", "restart", "mihomo").Run()
}
