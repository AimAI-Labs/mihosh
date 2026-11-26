package bootstrap

import (
	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
)

// App is the composition root that wires configuration, infrastructure clients,
// and application services for both TUI and CLI entry points.
type App struct {
	Config     *config.Config
	ConfigErr  error
	Target     config.MihomoTarget
	Client     *api.Client
	WSClient   *api.WSClient
	ConfigSvc  *service.ConfigService
	ConnSvc    *service.ConnectionService
	CoreSvc    *service.CoreService
	ProfileSvc *service.ProfileService
	IPResolver *service.IPResolver
}

// NewApp wires a complete runtime container with a freshly resolved Mihomo API client.
func NewApp() *App {
	cfg, err, target := DiscoverTarget()
	return newApp(cfg, err, target, api.NewClient(target.Endpoint, clientTimeout(cfg)))
}

// NewAppWithClient wires a complete runtime container bound to the caller-provided client
// (preserving a nil client for offline or unit-test callers).
func NewAppWithClient(client *api.Client) *App {
	cfg, err, target := DiscoverTarget()
	if client != nil {
		current := client.EndpointSnapshot()
		target.Endpoint.ExternalController, target.Endpoint.Secret = current.ExternalController, current.Secret
	}
	return newApp(cfg, err, target, client)
}

// DiscoverTarget is shared by the initial composition and runtime controller discovery.
func DiscoverTarget() (*config.Config, error, config.MihomoTarget) {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		defaultCfg := config.DefaultConfig
		cfg = &defaultCfg
	}
	return cfg, err, config.ResolveMihomoTarget(cfg)
}

func newApp(cfg *config.Config, cfgErr error, target config.MihomoTarget, client *api.Client) *App {
	configSvc := service.NewConfigService()
	connSvc := service.NewConnectionService(client)
	coreSvc := service.NewCoreService()
	profileSvc := service.NewProfileService(client, coreSvc)
	wsClient := api.NewWSClient(target.Endpoint.ExternalController, target.Endpoint.Secret)
	ipResolver := service.NewIPResolver()

	return &App{
		Config:     cfg,
		ConfigErr:  cfgErr,
		Target:     target,
		Client:     client,
		WSClient:   wsClient,
		ConfigSvc:  configSvc,
		ConnSvc:    connSvc,
		CoreSvc:    coreSvc,
		ProfileSvc: profileSvc,
		IPResolver: ipResolver,
	}
}

// NewClient creates a Mihomo HTTP API client from the resolved endpoint and config timeout.
func NewClient(cfg *config.Config) *api.Client {
	if cfg == nil {
		cfg, _ = config.Load()
	}
	return api.NewClient(config.ResolveMihomoTarget(cfg).Endpoint, clientTimeout(cfg))
}

func clientTimeout(cfg *config.Config) int {
	if cfg != nil && cfg.Timeout > 0 {
		return cfg.Timeout
	}
	return config.DefaultConfig.Timeout
}

// ProxyService returns a ProxyService bound to this container's API client.
func (a *App) ProxyService(testURL string, timeout int) *service.ProxyService {
	return service.NewProxyService(a.Client, testURL, timeout)
}

// Close releases resources owned by the wired services.
func (a *App) Close() {
	if a == nil {
		return
	}
	if a.WSClient != nil {
		a.WSClient.Stop()
	}
	if a.CoreSvc != nil {
		a.CoreSvc.Close()
	}
}
