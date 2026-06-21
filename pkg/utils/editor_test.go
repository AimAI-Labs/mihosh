package utils

import "testing"

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
		want      []string
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
