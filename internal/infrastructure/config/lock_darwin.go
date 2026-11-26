package config

import coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"

func lockAppConfigFile(path string) (func(), error) {
	return coreinfra.LockRuntime(path)
}
