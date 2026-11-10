//go:build !linux && !windows

package host

import "net"

func listenerExecutable([]net.IP, int) string { return "" }
