package api

import "github.com/AimAI-Labs/mihosh/internal/domain/port"

// Compile-time checks guaranteeing *Client satisfies all Mihomo API domain ports.
var (
	_ port.ProxyAPI       = (*Client)(nil)
	_ port.ConnectionAPI  = (*Client)(nil)
	_ port.RuleAPI        = (*Client)(nil)
	_ port.ConfigReloader = (*Client)(nil)
	_ port.RuntimeAPI     = (*Client)(nil)
	_ port.MihomoAPI      = (*Client)(nil)
)
