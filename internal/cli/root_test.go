package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestRootRejectsUnknownCommandsWithoutLaunchingTUI(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	initial := rootCmd.RunE
	calls := 0
	rootCmd.RunE = func(*cobra.Command, []string) error {
		calls++
		return nil
	}
	t.Cleanup(func() {
		rootCmd.RunE = initial
		rootCmd.SetArgs(nil)
	})
	for _, args := range [][]string{{"stauts"}, {"--", "stauts"}} {
		rootCmd.SetArgs(args)
		var stderr bytes.Buffer
		require.Equal(t, exitCodeParameter, executeRootCommand(rootCmd, &stderr), args)
		require.Contains(t, stderr.String(), "stauts")
		require.Zero(t, calls)
	}

	rootCmd.SetArgs([]string{})
	var stderr bytes.Buffer
	require.Equal(t, exitCodeOK, executeRootCommand(rootCmd, &stderr))
	require.Equal(t, 1, calls)
}
