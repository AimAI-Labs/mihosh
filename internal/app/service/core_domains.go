package service

import (
	"context"

	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
)

// CoreRuntime owns process lifecycle and owner coordination. CoreService keeps
// the compatibility facade while exposing this narrower capability to callers.
type CoreRuntime interface {
	Start(context.Context) error
	Stop() error
	StopContext(context.Context) error
	Restart(context.Context) error
	Serve(context.Context, func()) error
	Wait(context.Context) error
	Events() <-chan string
	ReadLogs(uint64) (string, uint64, bool)
	UsesProcessLogs() bool
}

// CoreRelease owns release discovery, installation, import, and upgrade flows.
type CoreRelease interface {
	Release(context.Context, string, func(coreinfra.Progress)) (coreinfra.Release, error)
	Releases(context.Context, int, func(coreinfra.Progress)) (coreinfra.ReleasePage, error)
	RefreshReleases(context.Context, int, func(coreinfra.Progress)) (coreinfra.ReleasePage, error)
	Install(context.Context, coreinfra.Release, func(coreinfra.Progress)) error
	Import(context.Context, string, string, string) error
	CachedRelease() coreinfra.Release
	Upgrade(context.Context, func(coreinfra.Progress)) error
}

// CoreConfig owns the selected core configuration and endpoint settings.
type CoreConfig interface {
	Configure() error
	UseExternal() error
	ConnectExternal(string, string, int) error
	SetPorts(int, int) error
}

// CoreNetwork owns host proxy and TUN integration.
type CoreNetwork interface {
	SaveNetwork(string, []coreinfra.Accelerator, ...int) error
	SetSystemProxy(context.Context, bool) error
	SetTUN(context.Context, bool) error
	GuardNetwork(context.Context) error
	RecoverNetwork(context.Context) error
}

type coreRuntimeDomain struct{ owner *CoreService }
type coreReleaseDomain struct{ owner *CoreService }
type coreConfigDomain struct{ owner *CoreService }
type coreNetworkDomain struct{ owner *CoreService }

var (
	_ CoreRuntime = (*coreRuntimeDomain)(nil)
	_ CoreRelease = (*coreReleaseDomain)(nil)
	_ CoreConfig  = (*coreConfigDomain)(nil)
	_ CoreNetwork = (*coreNetworkDomain)(nil)
)

func (d *coreRuntimeDomain) Start(ctx context.Context) error { return d.owner.startRuntime(ctx) }
func (d *coreRuntimeDomain) Stop() error                     { return d.owner.stopRuntime() }
func (d *coreRuntimeDomain) StopContext(ctx context.Context) error {
	return d.owner.stopRuntimeContext(ctx)
}
func (d *coreRuntimeDomain) Restart(ctx context.Context) error { return d.owner.restartRuntime(ctx) }
func (d *coreRuntimeDomain) Serve(ctx context.Context, ready func()) error {
	return d.owner.serveRuntime(ctx, ready)
}
func (d *coreRuntimeDomain) Wait(ctx context.Context) error { return d.owner.waitRuntime(ctx) }
func (d *coreRuntimeDomain) Events() <-chan string          { return d.owner.runtimeEvents() }
func (d *coreRuntimeDomain) ReadLogs(offset uint64) (string, uint64, bool) {
	return d.owner.readLogs(offset)
}
func (d *coreRuntimeDomain) UsesProcessLogs() bool { return d.owner.usesProcessLogs() }

func (d *coreReleaseDomain) Release(ctx context.Context, tag string, notify func(coreinfra.Progress)) (coreinfra.Release, error) {
	return d.owner.release(ctx, tag, notify)
}
func (d *coreReleaseDomain) Releases(ctx context.Context, page int, notify func(coreinfra.Progress)) (coreinfra.ReleasePage, error) {
	return d.owner.listReleases(ctx, page, notify)
}
func (d *coreReleaseDomain) RefreshReleases(ctx context.Context, page int, notify func(coreinfra.Progress)) (coreinfra.ReleasePage, error) {
	return d.owner.refreshReleases(ctx, page, notify)
}
func (d *coreReleaseDomain) Install(ctx context.Context, release coreinfra.Release, notify func(coreinfra.Progress)) error {
	return d.owner.installRelease(ctx, release, notify)
}
func (d *coreReleaseDomain) Import(ctx context.Context, source, tag, digest string) error {
	return d.owner.importRelease(ctx, source, tag, digest)
}
func (d *coreReleaseDomain) CachedRelease() coreinfra.Release { return d.owner.cachedRelease() }
func (d *coreReleaseDomain) Upgrade(ctx context.Context, notify func(coreinfra.Progress)) error {
	return d.owner.upgrade(ctx, notify)
}

func (d *coreConfigDomain) Configure() error   { return d.owner.configure() }
func (d *coreConfigDomain) UseExternal() error { return d.owner.useExternal() }
func (d *coreConfigDomain) ConnectExternal(address, secret string, mixedPort int) error {
	return d.owner.connectExternal(address, secret, mixedPort)
}
func (d *coreConfigDomain) SetPorts(controllerPort, mixedPort int) error {
	return d.owner.setPorts(controllerPort, mixedPort)
}

func (d *coreNetworkDomain) SaveNetwork(proxy string, accelerators []coreinfra.Accelerator, timeoutSeconds ...int) error {
	return d.owner.saveNetwork(proxy, accelerators, timeoutSeconds...)
}
func (d *coreNetworkDomain) SetSystemProxy(ctx context.Context, enabled bool) error {
	return d.owner.setSystemProxy(ctx, enabled)
}
func (d *coreNetworkDomain) SetTUN(ctx context.Context, enabled bool) error {
	return d.owner.setTUN(ctx, enabled)
}
func (d *coreNetworkDomain) GuardNetwork(ctx context.Context) error {
	return d.owner.guardNetwork(ctx)
}
func (d *coreNetworkDomain) RecoverNetwork(ctx context.Context) error {
	return d.owner.recoverNetwork(ctx)
}

func (s *CoreService) Start(ctx context.Context) error { return s.runtimeDomain.Start(ctx) }
func (s *CoreService) Stop() error                     { return s.runtimeDomain.Stop() }
func (s *CoreService) StopContext(ctx context.Context) error {
	return s.runtimeDomain.StopContext(ctx)
}
func (s *CoreService) Restart(ctx context.Context) error { return s.runtimeDomain.Restart(ctx) }
func (s *CoreService) Serve(ctx context.Context, ready func()) error {
	return s.runtimeDomain.Serve(ctx, ready)
}
func (s *CoreService) Wait(ctx context.Context) error { return s.runtimeDomain.Wait(ctx) }
func (s *CoreService) Events() <-chan string          { return s.runtimeDomain.Events() }
func (s *CoreService) ReadLogs(offset uint64) (string, uint64, bool) {
	return s.runtimeDomain.ReadLogs(offset)
}
func (s *CoreService) UsesProcessLogs() bool { return s.runtimeDomain.UsesProcessLogs() }

func (s *CoreService) Release(ctx context.Context, tag string, notify func(coreinfra.Progress)) (coreinfra.Release, error) {
	return s.releaseDomain.Release(ctx, tag, notify)
}
func (s *CoreService) Releases(ctx context.Context, page int, notify func(coreinfra.Progress)) (coreinfra.ReleasePage, error) {
	return s.releaseDomain.Releases(ctx, page, notify)
}
func (s *CoreService) RefreshReleases(ctx context.Context, page int, notify func(coreinfra.Progress)) (coreinfra.ReleasePage, error) {
	return s.releaseDomain.RefreshReleases(ctx, page, notify)
}
func (s *CoreService) Install(ctx context.Context, release coreinfra.Release, notify func(coreinfra.Progress)) error {
	return s.releaseDomain.Install(ctx, release, notify)
}
func (s *CoreService) Import(ctx context.Context, source, tag, digest string) error {
	return s.releaseDomain.Import(ctx, source, tag, digest)
}
func (s *CoreService) CachedRelease() coreinfra.Release { return s.releaseDomain.CachedRelease() }
func (s *CoreService) Upgrade(ctx context.Context, notify func(coreinfra.Progress)) error {
	return s.releaseDomain.Upgrade(ctx, notify)
}

func (s *CoreService) Configure() error { return s.configDomain.Configure() }
func (s *CoreService) SaveNetwork(proxy string, accelerators []coreinfra.Accelerator, timeoutSeconds ...int) error {
	return s.networkDomain.SaveNetwork(proxy, accelerators, timeoutSeconds...)
}
func (s *CoreService) UseExternal() error { return s.configDomain.UseExternal() }
func (s *CoreService) ConnectExternal(address, secret string, mixedPort int) error {
	return s.configDomain.ConnectExternal(address, secret, mixedPort)
}
func (s *CoreService) SetPorts(controllerPort, mixedPort int) error {
	return s.configDomain.SetPorts(controllerPort, mixedPort)
}
func (s *CoreService) SetSystemProxy(ctx context.Context, enabled bool) error {
	return s.networkDomain.SetSystemProxy(ctx, enabled)
}
func (s *CoreService) SetTUN(ctx context.Context, enabled bool) error {
	return s.networkDomain.SetTUN(ctx, enabled)
}
func (s *CoreService) GuardNetwork(ctx context.Context) error {
	return s.networkDomain.GuardNetwork(ctx)
}
func (s *CoreService) RecoverNetwork(ctx context.Context) error {
	return s.networkDomain.RecoverNetwork(ctx)
}

func (s *CoreService) Runtime() CoreRuntime        { return s.runtimeDomain }
func (s *CoreService) ReleaseManager() CoreRelease { return s.releaseDomain }
func (s *CoreService) Configuration() CoreConfig   { return s.configDomain }
func (s *CoreService) Network() CoreNetwork        { return s.networkDomain }
