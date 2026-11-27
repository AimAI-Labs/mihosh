package host

import "testing"

func TestLocalControllerRejectsRemoteAndInvalidTargets(t *testing.T) {
	for _, address := range []string{"https://example.com:9443", "192.0.2.1:9090", "127.0.0.1:0", "127.0.0.1:65536", "unix:///run/mihomo.sock", "bad:port"} {
		ips, _ := localController(address)
		if len(ips) != 0 || LocalCoreExecutable(address) != "" {
			t.Fatalf("remote or invalid address was treated as local: %s", address)
		}
	}
	for _, address := range []string{"127.0.0.1:9090", "http://localhost:9090", "http://[::1]:9090", "0.0.0.0:9090", ":9090", "http://:9090"} {
		ips, port := localController(address)
		if len(ips) == 0 || port != 9090 {
			t.Fatalf("local target was rejected: %s", address)
		}
	}
}
