package config

import (
	"reflect"
	"testing"
)

func TestRestartMihomoCommandUsesNonInteractiveSudo(t *testing.T) {
	cmd := restartMihomoCommand()
	want := []string{"sudo", "-n", "systemctl", "restart", "mihomo"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("restartMihomoCommand().Args = %v, want %v", cmd.Args, want)
	}
	if cmd.Stdin != nil {
		t.Fatalf("expected Stdin to remain nil to avoid blocking TUI TTY")
	}
}
