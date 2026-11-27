package sub

import (
	"errors"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func sourceClipboardAvailable() bool {
	return supportsSourceClipboard(runtime.GOOS)
}

func supportsSourceClipboard(platform string) bool {
	switch platform {
	case "windows", "linux", "darwin":
		return true
	default:
		return false
	}
}

func copySourceCmd(value string) tea.Cmd {
	return func() tea.Msg {
		success := copySourceClipboard(value, runtime.GOOS, os.Getenv("SSH_CONNECTION") != "", clipboard.WriteAll, writeTerminalClipboard)
		return messages.SubSourceCopiedMsg{Success: success}
	}
}

func copySourceClipboard(value, platform string, remote bool, writeDesktop, writeTerminal func(string) error) bool {
	value = strings.TrimSpace(value)
	if value == "" || !supportsSourceClipboard(platform) {
		return false
	}
	if platform == "windows" {
		return writeDesktop(value) == nil
	}
	if !remote && writeDesktop(value) == nil {
		return true
	}
	return writeTerminal(value) == nil
}

func writeTerminalClipboard(value string) error {
	terminal, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	_, writeErr := io.WriteString(terminal, ansi.SetSystemClipboard(value))
	return errors.Join(writeErr, terminal.Close())
}
