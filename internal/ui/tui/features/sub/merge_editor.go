package sub

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	tea "github.com/charmbracelet/bubbletea"
)

func (s State) openMergeExternalEditor() (State, tea.Cmd) {
	editor := utils.ResolveEditor()
	if editor == "" {
		return s, func() tea.Msg {
			return messages.MergeEditFinishedMsg{Err: errors.New("未找到可用的编辑器，请设置 EDITOR 变量")}
		}
	}
	cmd := &mergeEditor{command: utils.SplitEditorCommand(editor), activeUID: s.activeUID}
	return s, tea.Exec(cmd, func(err error) tea.Msg {
		return messages.MergeEditFinishedMsg{Err: err, ClearedOverrides: cmd.cleared}
	})
}

type mergeEditor struct {
	command        []string
	activeUID      string
	cleared        []string
	stdin          io.Reader
	stdout, stderr io.Writer
}

func (e *mergeEditor) SetStdin(r io.Reader)  { e.stdin = r }
func (e *mergeEditor) SetStdout(w io.Writer) { e.stdout = w }
func (e *mergeEditor) SetStderr(w io.Writer) { e.stderr = w }

func (e *mergeEditor) Run() error {
	original, err := profile.ReadMerge()
	if err != nil {
		return err
	}
	draft, draftErr := profile.NormalizeLegacyMerge(original, e.activeUID, config.GetMihomoConfigPath)
	if draftErr != nil {
		// Open the original so the user can repair invalid YAML or convert a legacy file manually.
		draft = original
	}
	dir, err := os.MkdirTemp("", "mihosh-merge-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	path := filepath.Join(dir, "merge.yaml")
	if err := os.WriteFile(path, draft, 0600); err != nil {
		return err
	}
	args := append(append([]string{}, e.command[1:]...), path)
	cmd := exec.Command(e.command[0], args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = e.stdin, e.stdout, e.stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if bytes.Equal(original, updated) {
		return nil
	}
	current, err := profile.ReadMerge()
	if err != nil {
		return err
	}
	if !bytes.Equal(current, original) {
		return errors.New("merge.yaml changed while the editor was open")
	}
	before, _ := profile.ReadMihomoOverridesFrom(original)
	after, err := profile.ReadMihomoOverridesFrom(updated)
	if err != nil {
		return err
	}
	if err := profile.WriteMerge(updated); err != nil {
		return err
	}
	e.cleared = profile.ClearedOverrideKeys(before, after)
	return nil
}
