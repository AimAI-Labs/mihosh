package host

import (
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

// LocalCoreExecutable returns the executable owning the local controller listener.
// Remote addresses, ambiguous owners and inaccessible processes remain unknown.
func LocalCoreExecutable(controller string) string {
	ips, port := localController(controller)
	if len(ips) == 0 {
		return ""
	}
	path := listenerExecutable(ips, port)
	name := strings.ToLower(filepath.Base(path))
	if !filepath.IsAbs(path) || (!strings.Contains(name, "mihomo") && !strings.Contains(name, "clash")) {
		return ""
	}
	return path
}

func localController(controller string) ([]net.IP, int) {
	if !strings.Contains(controller, "://") {
		controller = "http://" + controller
	}
	u, err := url.Parse(controller)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, 0
	}
	portText := u.Port()
	if portText == "" {
		portText = "80"
		if u.Scheme == "https" {
			portText = "443"
		}
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return nil, 0
	}
	if (u.Hostname() == "" && u.Port() != "") || strings.EqualFold(u.Hostname(), "localhost") {
		return []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback}, port
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil {
		return nil, 0
	}
	if ip.IsUnspecified() {
		if ip.To4() != nil {
			ip = net.IPv4(127, 0, 0, 1)
		} else {
			ip = net.IPv6loopback
		}
	}
	if ip.IsLoopback() {
		return []net.IP{ip}, port
	}
	addrs, _ := net.InterfaceAddrs()
	for _, addr := range addrs {
		if network, ok := addr.(*net.IPNet); ok && network.IP.Equal(ip) {
			return []net.IP{ip}, port
		}
	}
	return nil, 0
}
