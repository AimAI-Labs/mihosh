package sub

import (
	"errors"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLocalSubscriptionClipboardIsAvailable(t *testing.T) {
	switch runtime.GOOS {
	case "linux", "darwin", "windows":
	default:
		t.Skip("clipboard is supported on Linux, macOS and Windows")
	}
	t.Setenv("SSH_CONNECTION", "")
	require.True(t, sourceClipboardAvailable(), "local clipboard button should be enabled")
}

func TestSubscriptionClipboardRoutes(t *testing.T) {
	for _, tc := range []struct {
		name, platform string
		remote         bool
		failDesktop    bool
		failTerminal   bool
		wantCalls      []string
		wantSuccess    bool
	}{
		{name: "Linux desktop", platform: "linux", wantCalls: []string{"desktop"}, wantSuccess: true},
		{name: "macOS desktop", platform: "darwin", wantCalls: []string{"desktop"}, wantSuccess: true},
		{name: "Linux fallback", platform: "linux", failDesktop: true, wantCalls: []string{"desktop", "terminal"}, wantSuccess: true},
		{name: "macOS fallback", platform: "darwin", failDesktop: true, failTerminal: true, wantCalls: []string{"desktop", "terminal"}},
		{name: "Linux SSH", platform: "linux", remote: true, wantCalls: []string{"terminal"}, wantSuccess: true},
		{name: "macOS SSH", platform: "darwin", remote: true, wantCalls: []string{"terminal"}, wantSuccess: true},
		{name: "Windows retains native clipboard", platform: "windows", remote: true, failDesktop: true, wantCalls: []string{"desktop"}},
		{name: "unsupported", platform: "freebsd"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls []string
			probe := func(name string, fail bool) func(string) error {
				return func(value string) error {
					require.Equal(t, "https://example.test/sub", value)
					calls = append(calls, name)
					if fail {
						return errors.New("clipboard unavailable")
					}
					return nil
				}
			}
			success := copySourceClipboard(" https://example.test/sub ", tc.platform, tc.remote, probe("desktop", tc.failDesktop), probe("terminal", tc.failTerminal))
			require.Equal(t, tc.wantSuccess, success)
			require.Equal(t, tc.wantCalls, calls)
		})
	}
}
