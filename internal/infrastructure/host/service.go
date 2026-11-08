package host

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os/exec"
	"time"
)

type ServiceState struct{ Installed, Running, Enabled bool }
type Services interface {
	Install(context.Context, string) error
	Action(context.Context, string) error
	Status(context.Context) (ServiceState, error)
}
type Runner func(context.Context, string, ...string) ([]byte, error)

func Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	// OS commands must never include credentials or subscription data in errors.
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w", name, err)
	}
	return out, nil
}
func ServiceName(root string) string {
	return fmt.Sprintf("mihosh-core-%x", sha256.Sum256([]byte(root)))[:24]
}
