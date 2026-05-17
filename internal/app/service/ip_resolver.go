package service

import (
	"encoding/json"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
)

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
	defer r.mu.Unlock()

	// 缓存过期则刷新
	if time.Since(r.cacheTime) >= r.cacheTTL {
		r.refreshCache()
	}

	// 查缓存
	if resolved, ok := r.cache[ip]; ok {
		return resolved
	}

	// 缓存中未找到，按网段推测
	return r.guessBySubnet(ip)
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

func (r *IPResolver) refreshCache() {
	r.cache = make(map[string]*model.ResolvedIP)
	r.cacheTime = time.Now()
	r.refreshDocker()
	r.refreshTailscale()
}

// ── Docker 解析 ──

func (r *IPResolver) refreshDocker() {
	if !r.dockerChecked {
		_, err := exec.LookPath("docker")
		r.dockerOK = err == nil
		r.dockerChecked = true
	}
	if !r.dockerOK {
		return
	}

	out, err := exec.Command("docker", "ps", "-q").Output()
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
	out, err = exec.Command("docker", args...).Output()
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
				r.cache[ip] = &model.ResolvedIP{
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

func (r *IPResolver) refreshTailscale() {
	if !r.tsChecked {
		_, err := exec.LookPath("tailscale")
		r.tsOK = err == nil
		r.tsChecked = true
	}
	if !r.tsOK {
		return
	}

	out, err := exec.Command("tailscale", "status", "--json").Output()
	if err != nil {
		return
	}

	var status tailscaleStatus
	if err := json.Unmarshal(out, &status); err != nil {
		return
	}

	// 添加自身
	for _, ip := range status.Self.TailscaleIPs {
		r.cache[ip] = &model.ResolvedIP{
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
			r.cache[ip] = &model.ResolvedIP{
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

func (r *IPResolver) guessBySubnet(ip string) *model.ResolvedIP {
	netIP := net.ParseIP(ip)
	if netIP == nil {
		return &model.ResolvedIP{IP: ip, IsPrivate: true, NetworkType: "unknown", AppName: "(未知来源)"}
	}

	result := &model.ResolvedIP{IP: ip, IsPrivate: true}

	if netIP.IsLoopback() {
		result.NetworkType = "local"
		result.AppName = "本机"
		return result
	}

	// Docker: 172.16.0.0/12
	_, docker172, _ := net.ParseCIDR("172.16.0.0/12")
	if docker172 != nil && docker172.Contains(netIP) {
		result.NetworkType = "docker"
		if !r.dockerOK {
			result.AppName = "(CLI未安装)"
		} else {
			result.AppName = "(未知容器)"
		}
		return result
	}

	// Tailscale CGNAT: 100.64.0.0/10
	if isCGNAT(netIP) {
		result.NetworkType = "tailscale"
		if !r.tsOK {
			result.AppName = "(CLI未安装)"
		} else {
			result.AppName = "(未知设备)"
		}
		return result
	}

	// 10.0.0.0/8
	_, ten, _ := net.ParseCIDR("10.0.0.0/8")
	if ten != nil && ten.Contains(netIP) {
		result.NetworkType = "lan"
		result.AppName = "(10.x 内网设备)"
		return result
	}

	// 192.168.0.0/16 或其他
	result.NetworkType = "lan"
	result.AppName = "(局域网设备)"
	return result
}
