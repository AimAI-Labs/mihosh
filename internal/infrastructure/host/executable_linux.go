package host

import (
	"bufio"
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func listenerExecutable(ips []net.IP, port int) string {
	return procListenerExecutable("/proc", ips, port)
}

func procListenerExecutable(root string, ips []net.IP, port int) string {
	sockets := make(map[string]bool)
	for _, table := range []string{"tcp", "tcp6"} {
		f, err := os.Open(filepath.Join(root, "net", table))
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) < 10 || fields[3] != "0A" {
				continue
			}
			address, portText, ok := strings.Cut(fields[1], ":")
			number, err := strconv.ParseUint(portText, 16, 16)
			if !ok || err != nil || int(number) != port {
				continue
			}
			ip := procIP(address)
			for _, target := range ips {
				if ip != nil && (ip.Equal(target) || ip.IsUnspecified()) {
					sockets["socket:["+fields[9]+"]"] = true
				}
			}
		}
		f.Close()
	}
	if len(sockets) == 0 {
		return ""
	}
	processes, _ := os.ReadDir(root)
	path := ""
	for _, process := range processes {
		if _, err := strconv.Atoi(process.Name()); err != nil || !process.IsDir() {
			continue
		}
		dir := filepath.Join(root, process.Name())
		fds, _ := os.ReadDir(filepath.Join(dir, "fd"))
		for _, fd := range fds {
			link, _ := os.Readlink(filepath.Join(dir, "fd", fd.Name()))
			if !sockets[link] {
				continue
			}
			candidate, err := os.Readlink(filepath.Join(dir, "exe"))
			if err != nil || (path != "" && path != candidate) {
				return ""
			}
			path = candidate
			break
		}
	}
	return path
}

func procIP(address string) net.IP {
	if len(address) != 8 && len(address) != 32 {
		return nil
	}
	ip := make(net.IP, len(address)/2)
	for i := 0; i < len(address); i += 8 {
		word, err := strconv.ParseUint(address[i:i+8], 16, 32)
		if err != nil {
			return nil
		}
		binary.NativeEndian.PutUint32(ip[i/2:], uint32(word))
	}
	return ip
}
