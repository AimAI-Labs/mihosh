package service

import (
	"errors"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
)

// ErrRuleEditTarget indicates the target rule was not found in the effective subscription/config.
var ErrRuleEditTarget = profile.ErrRuleEditTarget

// ExtractProxyPolicies reads selectable policy groups and proxy nodes from the Mihomo config file.
func ExtractProxyPolicies(configPath string) (groups, nodes []string, err error) {
	return config.ExtractProxyPolicies(configPath)
}

// RuleNoResolveFlags parses no-resolve flags from the active Mihomo config (or generated active subscription).
func RuleNoResolveFlags() map[string]bool {
	path, err := config.GetMihomoConfigPath()
	if err == nil {
		return config.ParseRulesNoResolve(path)
	}
	if errors.Is(err, config.ErrExternalConfig) {
		if cfg, loadErr := config.Load(); loadErr == nil && cfg.ActiveSub != "" {
			if generated, generateErr := profile.GenerateForUID(cfg.ActiveSub, ""); generateErr == nil {
				return config.ParseRulesNoResolveYAML(generated.Data)
			}
		}
	}
	return nil
}
