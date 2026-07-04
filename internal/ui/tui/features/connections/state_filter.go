package connections

import (
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
)



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
	if s.filterList.FilterValue() == "" {
		return len(conns)
	}
	count := 0
	filter := strings.ToLower(s.filterList.FilterValue())
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
