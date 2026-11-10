package host

import (
	"context"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func listenerExecutable(ips []net.IP, port int) string {
	addresses := []string{"'0.0.0.0'", "'::'"}
	for _, ip := range ips {
		addresses = append(addresses, "'"+ip.String()+"'")
	}
	// Only parsed IP literals and an integer enter the PowerShell command.
	script := "$ErrorActionPreference='Stop'; [Console]::OutputEncoding=[System.Text.Encoding]::UTF8; " +
		"$addresses=@(" + strings.Join(addresses, ",") + "); " +
		"$paths=@(Get-NetTCPConnection -State Listen -LocalPort " + strconv.Itoa(port) +
		" | Where-Object { $_.LocalAddress -in $addresses } | ForEach-Object { (Get-Process -Id $_.OwningProcess).Path } | Sort-Object -Unique); " +
		"if ($paths.Count -eq 1) { [Console]::Write($paths[0]) }"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(string(output), "\ufeff"))
}
