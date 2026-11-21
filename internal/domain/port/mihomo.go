package port

import (
	"net/http"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
)

// ProxyAPI covers proxy group inspection, selection, delay testing, and proxied HTTP transport.
type ProxyAPI interface {
	GetGroups() (map[string]model.Group, []string, error)
	GetProxies() (map[string]model.Proxy, error)
	GetProxy(name string) (*model.Proxy, error)
	SelectProxy(group, proxy string) error
	TestProxyDelay(name, testURL string, timeout int) (int, error)
	TestGroupDelay(group, testURL string, timeout int) error
	NewHTTPClientWithProxy(proxyAddr string) (*http.Client, error)
}

// ConnectionAPI covers active connection inspection and termination.
type ConnectionAPI interface {
	GetConnections() (*model.ConnectionsResponse, error)
	CloseConnection(id string) error
	CloseAllConnections() error
}

// RuleAPI covers runtime rule inspection.
type RuleAPI interface {
	GetRules() (*model.RulesResponse, error)
}

// ConfigReloader covers live configuration reload and runtime patching.
type ConfigReloader interface {
	ReloadConfig(configPath string) error
	ReloadConfigPayload(payload []byte) error
	PatchConfig(patch map[string]any) error
}

// RuntimeAPI covers runtime config inspection, DNS, cache, and core maintenance API calls.
type RuntimeAPI interface {
	GetConfigs() (*model.ConfigsResponse, error)
	UpdateConfig(reqBody model.UpdateConfigRequest) error
	GetMemory() (*model.MemoryResponse, error)
	QueryDNS(name, recordType string) (*model.DNSQueryResponse, error)
	FlushDNSCache() error
	FlushFakeIP() error
	UpgradeCore(channel string) error
	RestartCore() error
	ReloadConfigsAPI() error
	UpdateGeoData() error
}

// MihomoAPI is the full controller API port implemented by infrastructure/api.Client.
type MihomoAPI interface {
	ProxyAPI
	ConnectionAPI
	RuleAPI
	ConfigReloader
	RuntimeAPI
}
