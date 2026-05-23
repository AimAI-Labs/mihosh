package cli

import (
	"bytes"
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeSystemdUnit(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "default", in: "", want: "mihomo.service"},
		{name: "bare name", in: "mihomo", want: "mihomo.service"},
		{name: "service suffix", in: "mihomo.service", want: "mihomo.service"},
		{name: "trim spaces", in: " mihomo ", want: "mihomo.service"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeSystemdUnit(tt.in))
		})
	}
}

func TestServiceStatusRunsSystemctlNoPager(t *testing.T) {
	calls := captureServiceCommand(t, "service", "--unit", "clash-meta", "status")

	require.Len(t, calls, 1)
	assert.Equal(t, "systemctl", calls[0].name)
	assert.Equal(t, []string{"status", "clash-meta.service", "--no-pager"}, calls[0].args)
}

func TestServiceMutatingCommandsUseSudoSystemctl(t *testing.T) {
	for _, action := range []string{"start", "stop", "restart", "enable", "disable"} {
		t.Run(action, func(t *testing.T) {
			calls := captureServiceCommand(t, "service", action)

			require.Len(t, calls, 1)
			assert.Equal(t, "sudo", calls[0].name)
			assert.Equal(t, []string{"systemctl", action, "mihomo.service"}, calls[0].args)
		})
	}
}

func TestServiceLogsRunsJournalctl(t *testing.T) {
	calls := captureServiceCommand(t, "service", "logs", "--lines", "50")

	require.Len(t, calls, 1)
	assert.Equal(t, "journalctl", calls[0].name)
	assert.Equal(t, []string{"-u", "mihomo.service", "-n", "50", "--no-pager"}, calls[0].args)
}

func TestServiceLogsFollowRunsJournalctlFollow(t *testing.T) {
	calls := captureServiceCommand(t, "service", "logs", "--follow", "--unit", "mihomo")

	require.Len(t, calls, 1)
	assert.Equal(t, "journalctl", calls[0].name)
	assert.Equal(t, []string{"-u", "mihomo.service", "-n", "200", "-f"}, calls[0].args)
}

func TestServiceLogsRejectsInvalidLines(t *testing.T) {
	oldPlatform := isLinuxSystemdFn
	defer func() { isLinuxSystemdFn = oldPlatform }()
	isLinuxSystemdFn = func() bool { return true }

	cmd := newTestRootWithService()
	cmd.SetArgs([]string{"service", "logs", "--lines", "0"})

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--lines 必须大于 0")
}

func TestServiceCommandFailureIsWrapped(t *testing.T) {
	oldRunner := runSystemCommandFn
	oldPlatform := isLinuxSystemdFn
	defer func() {
		runSystemCommandFn = oldRunner
		isLinuxSystemdFn = oldPlatform
	}()
	isLinuxSystemdFn = func() bool { return true }
	runSystemCommandFn = func(name string, args ...string) error {
		return errors.New("boom")
	}

	cmd := newTestRootWithService()
	cmd.SetArgs([]string{"service", "status"})

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "执行 systemctl 失败")
}

func captureServiceCommand(t *testing.T, args ...string) []systemCommandCall {
	t.Helper()

	var calls []systemCommandCall
	oldRunner := runSystemCommandFn
	oldPlatform := isLinuxSystemdFn
	defer func() {
		runSystemCommandFn = oldRunner
		isLinuxSystemdFn = oldPlatform
	}()
	isLinuxSystemdFn = func() bool { return true }
	runSystemCommandFn = func(name string, args ...string) error {
		calls = append(calls, systemCommandCall{name: name, args: append([]string(nil), args...)})
		return nil
	}

	cmd := newTestRootWithService()
	cmd.SetArgs(args)

	require.NoError(t, cmd.Execute())
	return calls
}

func newTestRootWithService() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "mihosh",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.AddCommand(newServiceCommand())
	return cmd
}

type systemCommandCall struct {
	name string
	args []string
}
