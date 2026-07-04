package connections

import (
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// handleConnFilterMode 连接过滤输入模式
func (s State) handleConnFilterMode(msg tea.KeyMsg) (State, tea.Cmd) {
	switch {
	case key.Matches(msg, common.Keys.Escape):
		s.connFilterMode = false
		s.connFilter.Blur()
	case key.Matches(msg, common.Keys.Enter):
		s.connFilterMode = false
		s.connFilter.Blur()
		s.selectedConn = 0
		s.connScrollTop = 0
	default:
		var cmd tea.Cmd
		s.connFilter, cmd = s.connFilter.Update(msg)
		return s, cmd
	}
	return s, nil
}

// filteredConnCount 过滤后的连接数量
func (s State) filteredConnCount() int {
	if s.connViewMode == ConnViewTraffic {
		return 0
	}
	var conns []model.Connection
	if s.connViewMode == ConnViewActive {
		if s.Connections == nil {
			return 0
		}
		conns = s.Connections.Connections
	} else {
		conns = s.ClosedConnections()
	}
	if s.connFilter.Value() == "" {
		return len(conns)
	}
	count := 0
	filter := strings.ToLower(s.connFilter.Value())
	for _, conn := range conns {
		if connMatchesFilter(conn, filter) {
			count++
		}
	}
	return count
}

// connMatchesFilter 检查连接是否匹配过滤词
func connMatchesFilter(conn model.Connection, filter string) bool {
	if filter == "" {
		return true
	}
	if strings.Contains(strings.ToLower(conn.Metadata.Host), filter) {
		return true
	}
	if strings.Contains(strings.ToLower(conn.Rule), filter) {
		return true
	}
	if strings.Contains(strings.ToLower(conn.Metadata.DestinationIP), filter) {
		return true
	}
	for _, chain := range conn.Chains {
		if strings.Contains(strings.ToLower(chain), filter) {
			return true
		}
	}
	return false
}
