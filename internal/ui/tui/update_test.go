package tui

import (
	"context"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdate_GlobalMessages(t *testing.T) {
	endpoint := config.MihomoEndpoint{
		ExternalController: "http://localhost:9090",
		Secret:             "secret",
	}
	client := api.NewClient(endpoint, 2000)

	m := NewModel(client, "http://www.gstatic.com/generate_204", 2000)

	// Test WindowSizeMsg
	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	newM, _ := m.Update(msg)
	newModel := newM.(Model)
	if newModel.width != 100 || newModel.height != 50 {
		t.Errorf("Expected width 100 and height 50, got %d, %d", newModel.width, newModel.height)
	}

	// Test UpdateCheckedMsg
	info := &model.UpdateInfo{Version: "v1.2.0", DownloadURL: "http://example.com"}
	newM, _ = newModel.Update(messages.UpdateCheckedMsg{Info: info})
	newModel = newM.(Model)
	if !newModel.hasUpdate || newModel.updateInfo == nil {
		t.Errorf("Expected hasUpdate true, got %v", newModel.hasUpdate)
	}

	// Test ErrMsg
	errMsg := messages.ErrMsg{Err: context.DeadlineExceeded}
	newM, _ = newModel.Update(errMsg)
	newModel = newM.(Model)
	if newModel.err == nil {
		t.Errorf("Expected error to be set")
	}

	// Test ConfigModeMsg
	newM, _ = newModel.Update(messages.ConfigModeMsg{Mode: "rule"})
	newModel = newM.(Model)
	if newModel.noticeTicks != 0 {
		t.Errorf("Notice ticks should be 0 or maintained based on logic")
	}
}

func TestUpdate_MouseMessages(t *testing.T) {
	client := api.NewClient(config.MihomoEndpoint{}, 2000)
	m := NewModel(client, "http://www.gstatic.com/generate_204", 2000)
	m.width = 100
	m.height = 50

	// Help popup intercepts mouse clicks
	m.showHelp = true
	msg := tea.MouseMsg{X: 10, Y: 10, Type: tea.MouseLeft, Action: tea.MouseActionPress}
	newM, _ := m.Update(msg)
	newModel := newM.(Model)
	if newModel.showHelp {
		t.Errorf("Expected left click to dismiss help popup")
	}

	// Error popup intercepts
	newModel.showErrorPopup = true
	newM, _ = newModel.Update(msg)
	newModel = newM.(Model)
	if newModel.showErrorPopup {
		t.Errorf("Expected left click to dismiss error popup")
	}

	// Update dialog intercepts
	newModel.showUpdateDialog = true
	newM, _ = newModel.Update(msg)
	newModel = newM.(Model)
	if newModel.showUpdateDialog {
		t.Errorf("Expected left click to dismiss update dialog")
	}

	// Click on top nav
	msg = tea.MouseMsg{X: 10, Y: 0, Type: tea.MouseLeft, Action: tea.MouseActionPress}
	newM, _ = newModel.Update(msg)
	newModel = newM.(Model)
	if newModel.currentPage != layout.PageNodes {
		t.Errorf("Expected top nav click on first tab to select PageNodes")
	}
}

func TestUpdate_KeyMessages(t *testing.T) {
	client := api.NewClient(config.MihomoEndpoint{}, 2000)
	m := NewModel(client, "", 2000)

	// Help popup intercepts keys
	m.showHelp = true
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	newM, cmd := m.Update(msg)
	newModel := newM.(Model)
	if newModel.showHelp {
		t.Errorf("Expected 'q' to dismiss help")
	}
	if cmd != nil {
		t.Errorf("Expected nil command when dismissing help")
	}

	// Global help key
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}
	newM, _ = newModel.Update(msg)
	newModel = newM.(Model)
	if !newModel.showHelp {
		t.Errorf("Expected '?' to show help")
	}

	// Next page
	newModel.showHelp = false
	msg = tea.KeyMsg{Type: tea.KeyTab}
	newM, _ = newModel.Update(msg)
	newModel = newM.(Model)
	if newModel.currentPage != layout.PageConnections {
		t.Errorf("Expected Tab to switch to next page, got %v", newModel.currentPage)
	}
}

func TestUpdate_DataMessages(t *testing.T) {
	client := api.NewClient(config.MihomoEndpoint{}, 2000)
	m := NewModel(client, "", 2000)
	var newM tea.Model

	// Test GroupsMsg
	groups := map[string]model.Group{"Proxy": {Name: "Proxy", Type: "Selector"}}
	newM, _ = m.Update(messages.GroupsMsg{Groups: groups, OrderedNames: []string{"Proxy"}})
	m = newM.(Model)

	// Test ProxiesMsg
	proxies := map[string]model.Proxy{"A": {Name: "A"}}
	newM, _ = m.Update(messages.ProxiesMsg(proxies))
	m = newM.(Model)

	// Test ConnectionsWSMsg
	connMsg := messages.ConnectionsWSMsg{Data: api.ConnectionsData{
		DownloadTotal: 100,
		Connections:   []api.ConnectionData{{ID: "1"}},
	}}
	newM, _ = m.Update(connMsg)
	m = newM.(Model)

	// Test LogsWSMsg
	logMsg := messages.LogsWSMsg{LogType: "info", Payload: "test log"}
	newM, _ = m.Update(logMsg)
	m = newM.(Model)

	// Test RuleAddedMsg
	newM, _ = m.Update(messages.RuleAddedMsg{})
	m = newM.(Model)
	if m.notice == "" {
		t.Errorf("Expected notice to be set for rule added")
	}

	// Test SubAddDoneMsg
	newM, _ = m.Update(messages.SubAddDoneMsg{UID: "sub1"})
	m = newM.(Model)
	if m.notice == "" {
		t.Errorf("Expected notice to be set for sub add done")
	}

	// Test NoticeMsg
	newM, _ = m.Update(messages.NoticeMsg{Text: "test notice"})
	m = newM.(Model)
	if m.notice != "test notice" {
		t.Errorf("Expected notice to be updated")
	}

	// Test AutoRefreshTickMsg
	m.autoRefreshRemaining = 1
	newM, _ = m.Update(messages.AutoRefreshTickMsg(time.Now()))
	m = newM.(Model)
	if m.autoRefreshRemaining != 5 { // Default auto refresh is 5 from DefaultConfig
		t.Errorf("Expected auto refresh remaining to reset to default interval, got %v", m.autoRefreshRemaining)
	}
}
