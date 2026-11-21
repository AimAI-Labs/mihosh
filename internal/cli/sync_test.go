package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/stretchr/testify/require"
)

func TestSyncCLIExportAndImportFormats(t *testing.T) {
	i18n.Init()
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	archivePath := filepath.Join(t.TempDir(), "cli-sync.tar.gz")

	t.Setenv("MIHOSH_CONFIG_DIR", srcDir)
	cfg := config.DefaultConfig
	cfg.Theme = "dracula"
	cfg.Subs = []profile.Profile{{
		UID:  "s1",
		Name: "Sub1",
		Source: profile.SubSource{
			Kind: profile.SourceRemote,
			URL:  "https://sub.example.com/sub?token=cli-secret",
		},
	}}
	require.NoError(t, config.Save(&cfg))
	require.NoError(t, profile.WriteMerge([]byte("secret: cli-ctrl-secret\nlog-level: info\n")))

	// Export with --output json
	cmd := newSyncCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"export", archivePath, "--output", "json"})
	require.NoError(t, cmd.Execute())

	var expRes service.SyncExportResult
	require.NoError(t, json.Unmarshal(out.Bytes(), &expRes))
	require.Equal(t, archivePath, expRes.Path)
	require.True(t, expRes.Redacted)
	require.Equal(t, 1, expRes.SubsCount)

	// Import onto destination with --output table
	t.Setenv("MIHOSH_CONFIG_DIR", dstDir)
	cmd2 := newSyncCommand()
	var out2 bytes.Buffer
	cmd2.SetOut(&out2)
	cmd2.SetArgs([]string{"import", archivePath, "--output", "table"})
	require.NoError(t, cmd2.Execute())
	require.Contains(t, out2.String(), "FIELD")
	require.Contains(t, out2.String(), archivePath)

	loaded, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, "dracula", loaded.Theme)
	require.NotContains(t, loaded.Subs[0].Source.URL, "cli-secret")

	// Import with --output plain
	cmd3 := newSyncCommand()
	var out3 bytes.Buffer
	cmd3.SetOut(&out3)
	cmd3.SetArgs([]string{"import", archivePath, "--output", "plain"})
	require.NoError(t, cmd3.Execute())
	require.Contains(t, out3.String(), archivePath)
}
