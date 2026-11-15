package host

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserServiceUsesStablePathsEscapesSpecifiersAndLimitsRestarts(t *testing.T) {
	root := filepath.Join(t.TempDir(), "profile % data $name")
	var calls [][]string
	s := &UserService{Root: root, UnitDir: t.TempDir(), Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		return nil, nil
	}}
	require.NoError(t, s.Install(context.Background(), filepath.Join(root, "bin", "mihosh")))
	data, err := os.ReadFile(filepath.Join(s.UnitDir, s.unit()))
	require.NoError(t, err)
	for _, expected := range []string{"Type=notify", "NotifyAccess=main", "Restart=on-failure", "StartLimitBurst=3", "KillMode=mixed", "--home", "%%", "$$", "UMask=0077"} {
		require.Contains(t, string(data), expected)
	}
	require.Equal(t, []string{"systemctl", "--user", "daemon-reload"}, calls[0])
	require.NoError(t, s.Action(context.Background(), "boot"))
	require.Equal(t, "loginctl", calls[1][0])
	require.Equal(t, "enable-linger", calls[1][1])
	require.Equal(t, []string{"systemctl", "--user", "enable", s.unit()}, calls[2])
}
func TestUserServiceNeverChangesUnownedUnit(t *testing.T) {
	s := &UserService{Root: t.TempDir(), UnitDir: t.TempDir(), Run: func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("must not run OS commands for unowned unit")
		return nil, nil
	}}
	path := filepath.Join(s.UnitDir, s.unit())
	require.NoError(t, os.WriteFile(path, []byte("[Service]\nExecStart=unrelated\n"), 0600))
	require.Error(t, s.Install(context.Background(), "/tmp/mihosh"))
	require.Error(t, s.Action(context.Background(), "uninstall"))
	data, _ := os.ReadFile(path)
	require.Contains(t, string(data), "unrelated")
}
func TestUserServiceStatusAndUninstallFailurePreserveUnit(t *testing.T) {
	s := &UserService{Root: t.TempDir(), UnitDir: t.TempDir()}
	path := filepath.Join(s.UnitDir, s.unit())
	require.NoError(t, os.WriteFile(path, []byte(serviceMarker), 0600))
	s.Run = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), "show") {
			return []byte("ActiveState=active\nUnitFileState=enabled\n"), nil
		}
		return nil, errors.New("OS rejected action")
	}
	state, err := s.Status(context.Background())
	require.NoError(t, err)
	require.Equal(t, ServiceState{Installed: true, Running: true, Enabled: true}, state)
	require.Error(t, s.Action(context.Background(), "uninstall"))
	_, err = os.Stat(path)
	require.NoError(t, err)
}

func TestServiceNetworkGuardUsesSeparateSystemdUnit(t *testing.T) {
	t.Setenv("INVOCATION_ID", "isolated-native-systemd-service")
	root := t.TempDir()
	cmd := GuardCommand("/test/mihosh", root, true)
	require.Equal(t, "systemd-run", filepath.Base(cmd.Path))
	require.Contains(t, cmd.Args, "--unit="+ServiceName(root)+"-guard")
	require.Contains(t, cmd.Args, "--collect")
	cmd = GuardCommand("/test/mihosh", root, false)
	require.Equal(t, "/test/mihosh", cmd.Path)
}

func TestUserServiceMissingUnitReportsFriendlyNotInstalledError(t *testing.T) {
	s := &UserService{Root: t.TempDir(), UnitDir: t.TempDir(), Run: func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("must not invoke systemctl for missing unit")
		return nil, nil
	}}
	state, err := s.Status(context.Background())
	require.NoError(t, err)
	require.False(t, state.Installed)
	err = s.Action(context.Background(), "start")
	require.ErrorContains(t, err, "not installed")
	require.ErrorIs(t, err, os.ErrNotExist)
}
