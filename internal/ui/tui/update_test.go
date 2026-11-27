package tui

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestUpdate_GlobalMessages(t *testing.T) {
	endpoint := config.MihomoEndpoint{
		ExternalController: "http://localhost:9090",
		Secret:             "secret",
	}
	client := api.NewClient(endpoint, 2000)

	m := newTestModel(client, "http://www.gstatic.com/generate_204", 2000)

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
	if newModel.notice.Message.Level != messages.NoticeError {
		t.Errorf("Expected error to be set")
	}

	// Test ConfigModeMsg
	newM, _ = newModel.Update(messages.ConfigModeMsg{Mode: "rule"})
	newModel = newM.(Model)
	if newModel.notice.Message.Level != messages.NoticeError {
		t.Errorf("a background mode update should not dismiss an error")
	}
}

func TestHomebrewUpdateDialogShowsManualUpgrade(t *testing.T) {
	i18n.Init()
	const width, height = 140, 30
	background := strings.Repeat(strings.Repeat(" ", width)+"\n", height)
	info := &model.UpdateInfo{Version: "v2.0.1", HomebrewManaged: true}
	rendered := ansi.Strip(overlayUpdateDialog(background, width, height, info, false, nil))
	if !strings.Contains(rendered, "brew upgrade mihosh") || strings.Contains(rendered, i18n.T("update.apply_now")) {
		t.Fatalf("expected Homebrew upgrade instructions without an automatic update button:\n%s", rendered)
	}
}

func TestServiceUpdateLeavesRestartNotice(t *testing.T) {
	i18n.Init()
	const executable = "/tmp/service/bin/mihosh"
	updated, quit, handled := (Model{}).applyCoreEvent(messages.UpdateAppliedMsg{ServiceBinary: executable})
	text := updated.ExitNotice()
	if !strings.Contains(text, executable) || !strings.Contains(text, "setcap") || !strings.Contains(text, "mihosh core service restart") {
		t.Fatalf("expected restart and capability instructions, got %q", text)
	}
	if !handled || quit == nil {
		t.Fatal("expected update to quit the TUI")
	}
	if _, ok := quit().(tea.QuitMsg); !ok {
		t.Fatal("expected a quit message")
	}
}

func TestUpdate_MouseMessages(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	client := api.NewClient(config.MihomoEndpoint{}, 2000)
	m := newTestModel(client, "http://www.gstatic.com/generate_204", 2000)
	m.currentPage = layout.PageNodes // Navigation starts here, independently of first-run routing.
	m.width = 100
	m.height = 50

	// Help popup intercepts mouse clicks
	m.showHelp = true
	msg := tea.MouseMsg{X: 10, Y: 10, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
	newM, _ := m.Update(msg)
	newModel := newM.(Model)
	if newModel.showHelp {
		t.Errorf("Expected left click to dismiss help popup")
	}

	// Error popup intercepts
	newModel.notice.DetailsOpen = true
	newM, _ = newModel.Update(msg)
	newModel = newM.(Model)
	if newModel.notice.DetailsOpen {
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
	msg = tea.MouseMsg{X: 10, Y: 0, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
	newM, _ = newModel.Update(msg)
	newModel = newM.(Model)
	if newModel.currentPage != layout.PageNodes {
		t.Errorf("Expected top nav click on first tab to select PageNodes")
	}
}

func TestUpdate_KeyMessages(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	client := api.NewClient(config.MihomoEndpoint{}, 2000)
	m := newTestModel(client, "", 2000)
	m.currentPage = layout.PageNodes // Navigation starts here, independently of first-run routing.

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
	m := newTestModel(client, "", 2000)
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

	// A successful rule edit reports the persisted and applied result.
	newM, _ = m.Update(messages.RuleEditAppliedMsg{NoticeKey: "rules.added_toast", Applied: true})
	m = newM.(Model)
	if m.notice.Message.Text == "" {
		t.Errorf("Expected notice to be set for rule added")
	}

	// Test SubAddDoneMsg
	newM, _ = m.Update(messages.SubAddDoneMsg{UID: "sub1"})
	m = newM.(Model)
	if m.notice.Message.Text == "" {
		t.Errorf("Expected notice to be set for sub add done")
	}

	// Test NoticeMsg
	newM, _ = m.Update(messages.NoticeMsg{Text: "test notice", Level: messages.NoticeSuccess})
	m = newM.(Model)
	if m.notice.Message.Text != "test notice" {
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

func TestModelCompactFootprintAndBoundedRingBuffers(t *testing.T) {
	if sz := unsafe.Sizeof(Model{}); sz >= 64<<10 {
		t.Fatalf("Model value size = %d bytes, want < %d bytes", sz, 64<<10)
	}

	m := newTestModel(nil, "", 2000)
	defer m.Close()

	// Logs ring buffer caps at common.LogsCap and retains newest entry.
	totalLogs := common.LogsCap + 25
	for i := 0; i < totalLogs; i++ {
		next, _ := m.Update(messages.LogsWSMsg{
			LogType: "info",
			Payload: fmt.Sprintf("entry-%d", i),
		})
		m = next.(Model)
	}
	logs := m.logsState.Logs()
	if len(logs) != common.LogsCap {
		t.Fatalf("len(Logs()) = %d, want %d", len(logs), common.LogsCap)
	}
	if logs[0].Payload != fmt.Sprintf("entry-%d", totalLogs-1) {
		t.Fatalf("newest log = %q, want entry-%d", logs[0].Payload, totalLogs-1)
	}

	// Closed connections ring buffer caps at common.ClosedConnCap.
	totalConns := common.ClosedConnCap + 25
	opened := make([]api.ConnectionData, totalConns)
	for i := range opened {
		opened[i] = api.ConnectionData{ID: fmt.Sprintf("conn-%d", i)}
	}
	next, _ := m.Update(messages.ConnectionsWSMsg{
		Data: api.ConnectionsData{Connections: opened},
	})
	m = next.(Model)
	next, _ = m.Update(messages.ConnectionsWSMsg{
		Data: api.ConnectionsData{Connections: nil},
	})
	m = next.(Model)
	if closed := m.connsState.ClosedConnections(); len(closed) != common.ClosedConnCap {
		t.Fatalf("len(ClosedConnections()) = %d, want %d", len(closed), common.ClosedConnCap)
	}
}

func TestUpdate_InlineHelpMenuClick(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	i18n.Init()
	i18n.SetLanguageOverride("zh-CN")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })

	client := api.NewClient(config.MihomoEndpoint{}, 2000)
	m := newTestModel(client, "", 2000)
	defer m.Close()
	m.width = 120
	m.height = 30
	m.currentPage = layout.PageRules

	findLabelX := func(view, label string) (int, int) {
		t.Helper()
		lines := strings.Split(ansi.Strip(view), "\n")
		page := m.mainPageLayout()
		for y := page.noticeRow() + 1; y < page.statusRow(); y++ {
			if at := strings.Index(lines[y], label); at >= 0 {
				return lipgloss.Width(lines[y][:at]) + 1, y
			}
		}
		t.Fatalf("label %q not found in bottom hint rows", label)
		return 0, 0
	}

	// Click "/ 搜索" on Rules page -> enters filter mode.
	x, y := findLabelX(m.View(), "/ "+i18n.T("help.rules.hint_search"))
	next, _ := m.Update(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m = next.(Model)
	if !m.rulesState.FilterMode() {
		t.Fatal("clicking bottom-right search menu item did not enter filter mode")
	}

	// While in filter mode (InputCapture active), click "Esc 取消" -> exits filter mode.
	x, y = findLabelX(m.View(), "Esc "+i18n.T("help.rules_search.cancel"))
	next, _ = m.Update(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m = next.(Model)
	if m.rulesState.FilterMode() {
		t.Fatal("clicking bottom-right Esc menu item did not exit filter mode")
	}

	// Click "? 帮助" on Settings page -> opens help popup.
	m.currentPage = layout.PageSettings
	x, y = findLabelX(m.View(), "? "+i18n.T("core.inline.help"))
	next, _ = m.Update(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m = next.(Model)
	if !m.showHelp {
		t.Fatal("clicking bottom-right help menu item did not open help popup")
	}
}

func TestUpdate_TrafficTestAllMenuClick(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	i18n.Init()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Millisecond)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	m := newTestModel(api.NewClient(config.MihomoEndpoint{}, 1000), "", 1000)
	defer func() {
		m.connsState.CancelTest()
		m.Close()
	}()
	m.width, m.height, m.currentPage = 80, 26, layout.PageConnections
	sites := []model.SiteTest{
		{Name: "Site A", URL: server.URL + "/a"},
		{Name: "Site B", URL: server.URL + "/b"},
	}
	m.connsState = connections.NewState("", sites, nil, 1000, nil)

	label := "S " + i18n.T("help.conns.hint_test_all")
	page := m.mainPageLayout()
	rows := strings.Split(ansi.Strip(m.View()), "\n")
	y, at := page.noticeRow()+1, -1
	for ; y < page.statusRow(); y++ {
		if at = strings.Index(rows[y], label); at >= 0 {
			break
		}
	}
	if at < 0 {
		t.Fatalf("batch test menu item %q is missing", label)
	}
	next, cmd := m.Update(tea.MouseMsg{
		X: lipgloss.Width(rows[y][:at]), Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
	})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("clicking the batch test menu item did not launch a site test")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) != len(sites) {
		t.Fatalf("batch launched %d commands for %d sites", len(batch), len(sites))
	}
	for _, probe := range batch {
		next, _ = m.Update(probe())
		m = next.(Model)
	}
	width, height := m.getPageSize()
	for _, site := range m.connsState.ToPageState(m.chartData, width, height).SiteTests {
		if site.Testing || site.Delay <= 0 || site.Error != "" {
			t.Errorf("site %q did not receive a successful latency result: %+v", site.Name, site)
		}
	}
}

func TestUpdateDownloadModalAndFailureStayVisible(t *testing.T) {
	const page = layout.PageCore
	click := tea.MouseMsg{X: 10, Y: 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
	for _, visible := range []bool{true, false} {
		t.Run(fmt.Sprintf("modal_visible_%t", visible), func(t *testing.T) {
			m := Model{currentPage: page, showUpdateDialog: visible, isUpdating: true}
			if visible {
				next, cmd := m.Update(click)
				m = next.(Model)
				if !m.showUpdateDialog || !m.isUpdating || m.currentPage != page || cmd != nil {
					t.Fatal("download modal failed to intercept a left click")
				}
			}
			err := context.DeadlineExceeded
			next, _ := m.Update(messages.UpdateAppliedMsg{Err: err})
			m = next.(Model)
			if m.isUpdating || m.updateError != err {
				t.Fatal("download failure did not finish the operation")
			}
			if visible {
				if !m.showUpdateDialog || m.notice.Message.Text != "" {
					t.Fatal("visible modal should keep and display its own error")
				}
				next, _ := m.Update(click)
				if next.(Model).showUpdateDialog || next.(Model).updateError != nil {
					t.Fatal("finished modal should dismiss and clear the error")
				}
			} else {
				notice := m.notice.Message
				if notice.Level != messages.NoticeError || notice.Text == "" || notice.Detail != err.Error() {
					t.Fatal("update failure with hidden modal was not published")
				}
			}
		})
	}
}

func TestUpdateAppliedNoticeUsesTheSelectedLanguage(t *testing.T) {
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("auto") })
	for _, tc := range []struct{ lang, want string }{
		{"zh-CN", "更新已完成"},
		{"en-US", "Update applied"},
	} {
		i18n.SetLanguageOverride(tc.lang)
		m, _, _ := (Model{}).applyCoreEvent(messages.UpdateAppliedMsg{})
		if m.notice.Message.Level != messages.NoticeSuccess || !strings.Contains(m.notice.Message.Text, tc.want) {
			t.Errorf("%s: unexpected update notice %q", tc.lang, m.notice.Message.Text)
		}
	}
}
