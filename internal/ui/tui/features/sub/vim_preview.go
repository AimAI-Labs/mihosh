package sub

import (
	_ "embed"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
)

//go:embed onedark-pro.vim
var previewVimrc []byte

type vimPreview struct {
	uid            string
	activated      bool
	stdin          io.Reader
	stdout, stderr io.Writer
}

func (v *vimPreview) SetStdin(r io.Reader)  { v.stdin = r }
func (v *vimPreview) SetStdout(w io.Writer) { v.stdout = w }
func (v *vimPreview) SetStderr(w io.Writer) { v.stderr = w }

func (v *vimPreview) Run() error {
	vim, err := exec.LookPath("vim")
	if err != nil {
		return errors.New(i18n.T("sub.file.vim_missing"))
	}
	data, err := profile.ReadRawFile(v.uid)
	if err != nil {
		return err
	}
	name := "source.yaml"
	var mergeWarning error
	if v.activated {
		generated, err := profile.GenerateSourcePreview(data)
		if err != nil {
			return err
		}
		data, mergeWarning = generated.Data, generated.MergeErr
		name = "activated.yaml"
	}
	dir, err := os.MkdirTemp("", "mihosh-preview-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	vimrc := filepath.Join(dir, "onedark-pro.vim")
	if err := os.WriteFile(vimrc, previewVimrc, 0600); err != nil {
		return err
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}
	cmd := exec.Command(vim, "-N", "-u", vimrc, "--noplugin", "-i", "NONE", "-n", "-M", "-R", "--", path)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = v.stdin, v.stdout, v.stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return mergeWarning
}
