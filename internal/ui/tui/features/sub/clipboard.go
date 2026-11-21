package sub

import (
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
	return runtime.GOOS == "windows" || runtime.GOOS == "linux" && os.Getenv("SSH_CONNECTION") != ""
}

func copySourceCmd(value string) tea.Cmd {
	return func() tea.Msg {
		value := strings.TrimSpace(value)
		if value == "" || !sourceClipboardAvailable() {
			return messages.SubSourceCopiedMsg{}
		}
		if runtime.GOOS == "windows" {
			return messages.SubSourceCopiedMsg{Success: clipboard.WriteAll(value) == nil}
		}
		terminal, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
		if err != nil {
			return messages.SubSourceCopiedMsg{}
		}
		_, writeErr := io.WriteString(terminal, ansi.SetSystemClipboard(value))
		closeErr := terminal.Close()
		return messages.SubSourceCopiedMsg{Success: writeErr == nil && closeErr == nil}
	}
}
