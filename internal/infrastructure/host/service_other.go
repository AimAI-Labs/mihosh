//go:build !linux

package host

import (
	"context"
	"errors"
)

type unsupported struct{}

func NewServices(string) Services { return unsupported{} }
func (unsupported) Install(context.Context, string) error {
	return errors.New("unsupported service platform")
}
func (unsupported) Action(context.Context, string) error {
	return errors.New("unsupported service platform")
}
func (unsupported) Status(context.Context) (ServiceState, error) {
	return ServiceState{}, errors.New("unsupported service platform")
}
func RunService(context.Context, string, func(context.Context, func()) error) error {
	return errors.New("unsupported service platform")
}
