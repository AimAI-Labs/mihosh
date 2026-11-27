package config

import "golang.org/x/sys/unix"

func readProcessArguments(pid int) ([]string, string) {
	raw, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil {
		return nil, ""
	}
	return parseDarwinProcessArguments(raw), ""
}
