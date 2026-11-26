package service

import (
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/host"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	syncinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/sync"
)

// UI-facing contracts keep presentation packages independent from concrete
// infrastructure adapters. The aliases preserve the existing runtime data
// shapes while the service package remains the only assembly boundary.
type (
	Client          = api.Client
	APIClient       = api.Client
	WSClient        = api.WSClient
	ConnectionsData = api.ConnectionsData
	MemoryData      = api.MemoryData
	TrafficData     = api.TrafficData
	LogData         = api.LogData

	Config           = config.Config
	MihomoCoreConfig = config.CoreConfig
	MihomoEndpoint   = config.MihomoEndpoint
	MihomoTarget     = config.MihomoTarget

	Accelerator = coreinfra.Accelerator
	Asset       = coreinfra.Asset
	Progress    = coreinfra.Progress
	Release     = coreinfra.Release
	ReleasePage = coreinfra.ReleasePage
	Status      = coreinfra.Status

	ChainEdits      = profile.ChainEdits
	FileDiffOptions = profile.FileDiffOptions
	FileDiffRow     = profile.FileDiffRow
	Profile         = profile.Profile
	ProxyChain      = profile.ProxyChain
	Snapshot        = profile.Snapshot
	SnapshotKind    = profile.SnapshotKind
	SourceKind      = profile.SourceKind
	SubSource       = profile.SubSource
)

const (
	SnapshotRaw   = profile.SnapshotRaw
	SnapshotMerge = profile.SnapshotMerge
	SourceLocal   = profile.SourceLocal
	SourceRemote  = profile.SourceRemote

	DiffIgnoreIndent           = profile.DiffIgnoreIndent
	DiffIgnoreWhitespace       = profile.DiffIgnoreWhitespace
	DiffIgnoreFormatCharacters = profile.DiffIgnoreFormatCharacters
	DiffIgnorePunctuation      = profile.DiffIgnorePunctuation
)

var DefaultConfig = config.DefaultConfig

var (
	ErrRawNotFound        = profile.ErrRawNotFound
	ErrChainProxyNotFound = profile.ErrChainProxyNotFound
	ErrChainCycle         = profile.ErrChainCycle
	ErrChainConflict      = profile.ErrChainConflict
)

func Load() (*Config, error) { return config.Load() }

func GetConfigDir() (string, error) { return config.GetConfigDir() }

func GetMihomoConfigPath() (string, error) { return config.GetMihomoConfigPath() }

func ManagedMihomoConfigPath() (string, error) { return config.ManagedMihomoConfigPath() }

func MixedPortToProxyURL(port int) string { return config.MixedPortToProxyURL(port) }

func PreserveSubscriptionUsage() error { return config.PreserveSubscriptionUsage() }

func ServiceName(root string) string { return host.ServiceName(root) }

func RedactDSN(dsn string) string { return syncinfra.RedactDSN(dsn) }

func ReadChainEdits() (ChainEdits, error) { return profile.ReadChainEdits() }

func ValidateProxyChain(chain ProxyChain) error { return profile.ValidateProxyChain(chain) }

func IsChainBuiltInProxy(name string) bool { return profile.IsChainBuiltInProxy(name) }

func ParseSourceKind(value string) SourceKind { return profile.ParseSourceKind(value) }

func FormatUpdateInterval(minutes int) string { return profile.FormatUpdateInterval(minutes) }

func ReadRawFile(uid string) ([]byte, error) { return profile.ReadRawFile(uid) }

func ReadMerge() ([]byte, error) { return profile.ReadMerge() }

func WriteMerge(data []byte) error { return profile.WriteMerge(data) }

func ReadMihomoOverridesFrom(data []byte) (map[string]string, error) {
	return profile.ReadMihomoOverridesFrom(data)
}

func ValidateMihomoOverride(key, value string) error {
	return profile.ValidateMihomoOverride(key, value)
}

func ClearedOverrideKeys(before, after map[string]string) []string {
	return profile.ClearedOverrideKeys(before, after)
}

func GenerateSourcePreview(data []byte) (profile.GeneratedConfig, error) {
	return profile.GenerateSourcePreview(data)
}

func DiffFiles(before, after string, options FileDiffOptions) []FileDiffRow {
	return profile.DiffFiles(before, after, options)
}

func DefaultAccelerators() []Accelerator { return coreinfra.DefaultAccelerators() }

func PinnedDigests() map[string]string { return coreinfra.PinnedDigests() }

func PinnedRelease() Release { return coreinfra.PinnedRelease() }
