package service

import (
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
)

// LoadFileDiffOptions returns the user's persisted subscription diff options, or defaults when unset.
func LoadFileDiffOptions() profile.FileDiffOptions {
	if cfg, err := config.Load(); err == nil && cfg.FileDiffOptions != nil {
		return *cfg.FileDiffOptions
	}
	return profile.DefaultFileDiffOptions
}

// SaveFileDiffOptions persists the user's subscription diff options to application config.
func SaveFileDiffOptions(opts profile.FileDiffOptions) error {
	return config.Update(func(cfg *config.Config) error {
		v := opts
		cfg.FileDiffOptions = &v
		return nil
	})
}

// DiffRawWithPreview computes a line-level diff between a subscription's raw source and its merged preview.
// When a non-fatal merge warning occurs, both the diff rows and the merge warning error are returned.
func DiffRawWithPreview(uid string, opts profile.FileDiffOptions) ([]profile.FileDiffRow, error) {
	before, err := profile.ReadRawFile(uid)
	if err != nil {
		return nil, err
	}
	generated, err := profile.GenerateSourcePreview(before)
	if err != nil {
		return nil, err
	}
	return profile.DiffFiles(string(before), string(generated.Data), opts), generated.MergeErr
}

// NormalizeMergeDraft normalizes legacy merge content using the active Mihomo config path resolver.
func NormalizeMergeDraft(original []byte, activeUID string) ([]byte, error) {
	return profile.NormalizeLegacyMerge(original, activeUID, config.GetMihomoConfigPath)
}
