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
	Endpoint   config.MihomoEndpoint
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
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		defaultCfg := config.DefaultConfig
		cfg = &defaultCfg
	}
	endpoint := config.ResolveMihomoEndpoint()
	return newApp(cfg, err, endpoint, api.NewClient(endpoint, cfg.Timeout))
}

// NewAppWithClient wires a complete runtime container bound to the caller-provided client
// (preserving a nil client for offline or unit-test callers).
func NewAppWithClient(client *api.Client) *App {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		defaultCfg := config.DefaultConfig
		cfg = &defaultCfg
	}
	endpoint := config.ResolveMihomoEndpoint()
	return newApp(cfg, err, endpoint, client)
}

func newApp(cfg *config.Config, cfgErr error, endpoint config.MihomoEndpoint, client *api.Client) *App {
	configSvc := service.NewConfigService()
	connSvc := service.NewConnectionService(client)
	coreSvc := service.NewCoreService()
	profileSvc := service.NewProfileService(client, coreSvc)
	wsClient := api.NewWSClient(endpoint.ExternalController, endpoint.Secret)
	ipResolver := service.NewIPResolver()

	return &App{
		Config:     cfg,
		ConfigErr:  cfgErr,
		Endpoint:   endpoint,
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
	timeout := config.DefaultConfig.Timeout
	if cfg != nil && cfg.Timeout > 0 {
		timeout = cfg.Timeout
	}
	return api.NewClient(config.ResolveMihomoEndpoint(), timeout)
}

// ProxyService returns a ProxyService bound to this container's API client.
func (a *App) ProxyService(testURL string, timeout int) *service.ProxyService {
	return service.NewProxyService(a.Client, testURL, timeout)
}

// Close releases resources owned by the wired services.
func (a *App) Close() {
	if a != nil && a.CoreSvc != nil {
		a.CoreSvc.Close()
	}
}
