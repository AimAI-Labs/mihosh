package utils

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestParseEditorFromRC(t *testing.T) {
	cases := []struct {
		name, content, want string
	}{
		{"unquoted", "export EDITOR=vim\n", "vim"},
		{"no_export", "EDITOR=nano\n", "nano"},
		{"double_quoted", "export EDITOR=\"vim\"\n", "vim"},
		{"single_quoted_with_args", "export EDITOR='code --wait'\n", "code --wait"},
		{"trailing_comment_unquoted", "export EDITOR=vim # use vim\n", "vim"},
		{"trailing_comment_quoted", "export EDITOR=\"nano\" # comment\n", "nano"},
		{"comment_line_ignored", "# EDITOR=vim\nexport EDITOR=nano\n", "nano"},
		{"last_wins", "export EDITOR=vim\nexport EDITOR=nano\n", "nano"},
		{"my_editor_not_matched", "export MY_EDITOR=vim\n", ""},
		{"no_match", "export PATH=/usr/bin\nalias e=vim\n", ""},
		{"empty_value_skipped", "export EDITOR=vim\nexport EDITOR=''\n", "vim"},
		{"blank_value_skipped", "export EDITOR=vim\nexport EDITOR=' \t'\n", "vim"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ParseEditorFromRC(c.content); got != c.want {
				t.Fatalf("ParseEditorFromRC: got %q, want %q", got, c.want)
			}
		})
	}
}

func TestSplitEditorCommand(t *testing.T) {
	cases := []struct {
		name, in string
		want     []string
	}{
		{"single", "vim", []string{"vim"}},
		{"with_args", "code --wait", []string{"code", "--wait"}},
		{"empty", "  ", nil},
		{"leading_space", "  nano", []string{"nano"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := SplitEditorCommand(c.in)
			if len(got) != len(c.want) {
				t.Fatalf("SplitEditorCommand(%q): got %v, want %v", c.in, got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("SplitEditorCommand(%q)[%d]: got %q, want %q", c.in, i, got[i], c.want[i])
				}
			}
		})
	}
}

func TestResolveEditorSkipsBlankEnvironment(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("VISUAL", " \t")
	t.Setenv("EDITOR", "  nano  ")
	if got := ResolveEditor(); got != "nano" {
		t.Fatalf("ResolveEditor() = %q, want nano", got)
	}
}

func TestResolveEditorPrefersEnvironmentAndMatchingShell(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	for name, contents := range map[string]string{
		".bashrc": "export EDITOR=bash-editor\n",
		".zshrc":  "EDITOR=zsh-editor\nexport EDITOR=\"$VISUAL\"\n",
	} {
		if err := os.WriteFile(filepath.Join(home, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("SHELL", "/bin/zsh")
	t.Setenv("VISUAL", " visual-editor ")
	t.Setenv("EDITOR", "env-editor")
	if got := ResolveEditor(); got != "visual-editor" {
		t.Errorf("ResolveEditor with VISUAL = %q", got)
	}
	t.Setenv("VISUAL", "")
	if got := ResolveEditor(); got != "env-editor" {
		t.Errorf("ResolveEditor with EDITOR = %q", got)
	}
	t.Setenv("EDITOR", "")
	if got := ResolveEditor(); got != "zsh-editor" {
		t.Errorf("ResolveEditor for zsh = %q", got)
	}
	t.Setenv("SHELL", "/usr/bin/bash")
	if got := ResolveEditor(); got != "bash-editor" {
		t.Errorf("ResolveEditor for bash = %q", got)
	}
}

func TestSplitEditorCommandKeepsPathsAndArguments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Editor With Space")
	if err := os.WriteFile(path, nil, 0700); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		command string
		want    []string
	}{
		{path, []string{path}},
		{`"` + path + `" --wait "argument with spaces"`, []string{path, "--wait", "argument with spaces"}},
		{`/opt/Editor\ Folder/editor --wait`, []string{"/opt/Editor Folder/editor", "--wait"}},
		{`"C:\Program Files\Editor\editor.exe" --wait`, []string{`C:\Program Files\Editor\editor.exe`, "--wait"}},
		{`vim -c ""`, []string{"vim", "-c", ""}},
		{`vim "unterminated`, nil},
	} {
		if got := SplitEditorCommand(test.command); !slices.Equal(got, test.want) {
			t.Errorf("SplitEditorCommand(%q) = %q, want %q", test.command, got, test.want)
		}
	}
}
