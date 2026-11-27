//go:build !linux

package host

import (
	"context"
	"errors"
	"os/exec"
)

func NewNetwork() Network { return unsupported{} }
func (unsupported) ReadProxy(context.Context) (ProxySettings, error) {
	return nil, errors.New("unsupported system proxy platform")
}
func (unsupported) ProxyTarget(int) ProxySettings { return nil }
func (unsupported) WriteProxy(context.Context, ProxySettings) error {
	return errors.New("unsupported system proxy platform")
}
func (unsupported) CheckTUN(context.Context, string, TUN) error {
	return errors.New("unsupported TUN platform")
}
func (unsupported) CaptureTUN(context.Context, TUN) (TUN, error) {
	return TUN{}, errors.New("unsupported TUN platform")
}
func (unsupported) CleanupTUN(context.Context, TUN) error {
	return errors.New("unsupported TUN platform")
}
func configureGuard(*exec.Cmd)      {}
func ConfigureTUNProcess(*exec.Cmd) {}
