package sub

import (
	"os/exec"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/stretchr/testify/require"
)

func TestMergeEditorValidatesBeforeReplacingOriginal(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh is unavailable")
	}
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	original := []byte("allow-lan: true\nmode: rule\n")
	require.NoError(t, profile.WriteMerge(original))
	editor := mergeEditor{command: []string{sh, "-c", `printf '%s\n' 'prepend-rules: []' > "$1"`, "editor"}}
	require.ErrorContains(t, editor.Run(), "prepend-rules")
	saved, err := profile.ReadMerge()
	require.NoError(t, err)
	require.Equal(t, original, saved)

	editor.command[2] = `printf '%s\n' 'mode: global' > "$1"`
	require.NoError(t, editor.Run())
	require.Equal(t, []string{"allow-lan"}, editor.cleared)
	saved, err = profile.ReadMerge()
	require.NoError(t, err)
	require.Equal(t, "mode: global\n", string(saved))
}
