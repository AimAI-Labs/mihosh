package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func readProcessArguments(pid int) ([]string, string) {
	root := filepath.Join("/proc", strconv.Itoa(pid))
	raw, err := os.ReadFile(filepath.Join(root, "cmdline"))
	if err != nil || len(raw) == 0 {
		return nil, ""
	}
	cwd, _ := os.Readlink(filepath.Join(root, "cwd"))
	return strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00"), cwd
}
