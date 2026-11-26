package service

import (
	"context"
	"encoding/json"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
)

const ipResolverCmdTimeout = 3 * time.Second

// IPResolver 内网IP解析器（带缓存）
type IPResolver struct {
	mu            sync.Mutex
	cache         map[string]*model.ResolvedIP
	cacheTime     time.Time
	cacheTTL      time.Duration
	dockerChecked bool
	dockerOK      bool
	tsChecked     bool
	tsOK          bool
}

// NewIPResolver 创建IP解析器
func NewIPResolver() *IPResolver {
	return &IPResolver{
		cache:    make(map[string]*model.ResolvedIP),
		cacheTTL: 60 * time.Second,
	}
}

// Resolve 解析IP地址，返回解析结果
func (r *IPResolver) Resolve(ip string) *model.ResolvedIP {
	if !IsPrivateIP(ip) {
		return &model.ResolvedIP{IP: ip, IsPrivate: false}
	}

	r.mu.Lock()
	expired := time.Since(r.cacheTime) >= r.cacheTTL
	dockerChecked, dockerOK := r.dockerChecked, r.dockerOK
	tsChecked, tsOK := r.tsChecked, r.tsOK
	if !expired {
		if resolved, ok := r.cache[ip]; ok {
			r.mu.Unlock()
			return resolved
		}
		r.mu.Unlock()
		return r.guessBySubnet(ip, dockerOK, tsOK)
	}
	r.mu.Unlock()

	// Run external CLI probes outside r.mu with a strict timeout so a hung daemon never deadlocks callers.
	if !dockerChecked {
		_, err := exec.LookPath("docker")
		dockerOK = err == nil
		dockerChecked = true
	}
	if !tsChecked {
		_, err := exec.LookPath("tailscale")
		tsOK = err == nil
		tsChecked = true
	}
	nextCache := make(map[string]*model.ResolvedIP)
	if dockerOK {
		collectDockerIPs(nextCache)
	}
	if tsOK {
		collectTailscaleIPs(nextCache)
	}

	r.mu.Lock()
	r.dockerChecked, r.dockerOK = dockerChecked, dockerOK
	r.tsChecked, r.tsOK = tsChecked, tsOK
	r.cache = nextCache
	r.cacheTime = time.Now()
	resolved, ok := r.cache[ip]
	r.mu.Unlock()
	if ok {
		return resolved
	}
	return r.guessBySubnet(ip, dockerOK, tsOK)
}

// IsPrivateIP 判断是否为内网IP
func IsPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || isCGNAT(ip)
}

func isCGNAT(ip net.IP) bool {
	_, cgnat, _ := net.ParseCIDR("100.64.0.0/10")
	return cgnat != nil && cgnat.Contains(ip)
}

// ── Docker 解析 ──

func collectDockerIPs(cache map[string]*model.ResolvedIP) {
	ctx, cancel := context.WithTimeout(context.Background(), ipResolverCmdTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "docker", "ps", "-q").Output()
	if err != nil {
		return
	}

	ids := strings.Fields(strings.TrimSpace(string(out)))
	if len(ids) == 0 {
		return
	}

	// 一次 inspect 所有容器：Name|Image|IP1,IP2,...
	args := append([]string{"inspect", "--format",
		"{{.Name}}|{{.Config.Image}}|{{range .NetworkSettings.Networks}}{{.IPAddress}},{{end}}"}, ids...)
	inspectCtx, inspectCancel := context.WithTimeout(context.Background(), ipResolverCmdTimeout)
	defer inspectCancel()
	out, err = exec.CommandContext(inspectCtx, "docker", args...).Output()
	if err != nil {
		return
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 3 {
			continue
		}
		name := strings.TrimPrefix(parts[0], "/")
		image := parts[1]
		ips := strings.Split(strings.TrimRight(parts[2], ","), ",")
		for _, ip := range ips {
			ip = strings.TrimSpace(ip)
			if ip != "" {
				cache[ip] = &model.ResolvedIP{
					IP:          ip,
					IsPrivate:   true,
					NetworkType: "docker",
					AppName:     name,
					AppDetail:   image,
				}
			}
		}
	}
}

// ── Tailscale 解析 ──

func collectTailscaleIPs(cache map[string]*model.ResolvedIP) {
	ctx, cancel := context.WithTimeout(context.Background(), ipResolverCmdTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "tailscale", "status", "--json").Output()
	if err != nil {
		return
	}

	var status tailscaleStatus
	if err := json.Unmarshal(out, &status); err != nil {
		return
	}

	// 添加自身
	for _, ip := range status.Self.TailscaleIPs {
		cache[ip] = &model.ResolvedIP{
			IP:          ip,
			IsPrivate:   true,
			NetworkType: "tailscale",
			AppName:     status.Self.HostName,
			AppDetail:   status.Self.OS,
		}
	}

	// 添加 peers
	for _, peer := range status.Peer {
		for _, ip := range peer.TailscaleIPs {
			cache[ip] = &model.ResolvedIP{
				IP:          ip,
				IsPrivate:   true,
				NetworkType: "tailscale",
				AppName:     peer.HostName,
				AppDetail:   peer.OS,
			}
		}
	}
}

// tailscaleStatus tailscale status --json 的部分结构
type tailscaleStatus struct {
	Self struct {
		HostName     string   `json:"HostName"`
		TailscaleIPs []string `json:"TailscaleIPs"`
		OS           string   `json:"OS"`
	} `json:"Self"`
	Peer map[string]struct {
		HostName     string   `json:"HostName"`
		TailscaleIPs []string `json:"TailscaleIPs"`
		OS           string   `json:"OS"`
	} `json:"Peer"`
}

// ── 网段推测（降级） ──

func (r *IPResolver) guessBySubnet(ip string, dockerOK, tsOK bool) *model.ResolvedIP {
	netIP := net.ParseIP(ip)
	if netIP == nil {
		return &model.ResolvedIP{IP: ip, IsPrivate: true, NetworkType: "unknown", AppName: "(unknown)"}
	}

	result := &model.ResolvedIP{IP: ip, IsPrivate: true}

	if netIP.IsLoopback() {
		result.NetworkType = "local"
		result.AppName = "localhost"
		return result
	}

	// Docker: 172.16.0.0/12
	_, docker172, _ := net.ParseCIDR("172.16.0.0/12")
	if docker172 != nil && docker172.Contains(netIP) {
		result.NetworkType = "docker"
		if !dockerOK {
			result.AppName = "(CLI unavailable)"
		} else {
			result.AppName = "(unknown container)"
		}
		return result
	}

	// Tailscale CGNAT: 100.64.0.0/10
	if isCGNAT(netIP) {
		result.NetworkType = "tailscale"
		if !tsOK {
			result.AppName = "(CLI unavailable)"
		} else {
			result.AppName = "(unknown peer)"
		}
		return result
	}

	// 10.0.0.0/8
	_, ten, _ := net.ParseCIDR("10.0.0.0/8")
	if ten != nil && ten.Contains(netIP) {
		result.NetworkType = "lan"
		result.AppName = "(10.x LAN)"
		return result
	}

	// 192.168.0.0/16 或其他
	result.NetworkType = "lan"
	result.AppName = "(LAN)"
	return result
}
