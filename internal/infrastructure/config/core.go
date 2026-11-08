package config

import (
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"path/filepath"
)

func ManagedMihomoConfigPath() (string, error) {
	root, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "runtime", "config.yaml"), nil
}

type errExternalConfig struct{}

func (errExternalConfig) Error() string { return i18n.T("core.external_no_file") }

var ErrExternalConfig = errExternalConfig{}
