package connections

import (
	"sort"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
)

// ConnSortField defines the sorting dimension for active and historical connections.
type ConnSortField int

const (
	ConnSortDefault ConnSortField = iota
	ConnSortSpeed
	ConnSortTraffic
	ConnSortDuration
	ConnSortStart
	connSortFieldCount
)

// ConnFilterField defines which connection attribute text queries match against.
type ConnFilterField int

const (
	ConnFilterAll ConnFilterField = iota
	ConnFilterProcess
	ConnFilterSourceIP
	ConnFilterChain
	connFilterFieldCount
)

// ConnQuery holds the complete filter + sort specification shared by rendering,
// mouse hit-testing, keyboard selection, and batch close operations.
type ConnQuery struct {
	Text     string
	Field    ConnFilterField
	Protocol string // "", "tcp", "udp"
	SortBy   ConnSortField
	SortDesc bool
}

func (q ConnQuery) HasActiveFilter() bool {
	return strings.TrimSpace(q.Text) != "" || q.Protocol != "" || q.Field != ConnFilterAll
}

func (q ConnQuery) HasActiveCriteria() bool {
	return q.HasActiveFilter() || q.SortBy != ConnSortDefault
}

func (f ConnSortField) Label() string {
	switch f {
	case ConnSortSpeed:
		return i18n.T("conns.sort.speed")
	case ConnSortTraffic:
		return i18n.T("conns.sort.traffic")
	case ConnSortDuration:
		return i18n.T("conns.sort.duration")
	case ConnSortStart:
		return i18n.T("conns.sort.start")
	default:
		return i18n.T("conns.sort.default")
	}
}

func (f ConnFilterField) Label() string {
	switch f {
	case ConnFilterProcess:
		return i18n.T("conns.filter.field_process")
	case ConnFilterSourceIP:
		return i18n.T("conns.filter.field_source_ip")
	case ConnFilterChain:
		return i18n.T("conns.filter.field_chain")
	default:
		return i18n.T("conns.filter.field_all")
	}
}

func (s State) currentQuery() ConnQuery {
	return ConnQuery{
		Text:     s.filterList.FilterValue(),
		Field:    s.filterField,
		Protocol: s.protoFilter,
		SortBy:   s.sortField,
		SortDesc: s.sortDesc,
	}
}

func (s State) visibleSourceConnections() []model.Connection {
	switch s.connViewMode {
	case ConnViewActive:
		visible := s.visibleConnections()
		if visible == nil {
			return nil
		}
		return visible.Connections
	case ConnViewHistory:
		return s.visibleClosedConnections()
	default:
		return nil
	}
}

// filteredConnections returns the visible connection list after applying the
// active filters and sort order.
func (s State) filteredConnections() []model.Connection {
	return queryConnections(s.visibleSourceConnections(), s.currentQuery(), time.Now())
}

// filteredConnCount 过滤后的连接数量
func (s State) filteredConnCount() int {
	return len(s.filteredConnections())
}

// queryConnections applies filtering and stable sorting, returning a fresh slice
// when filtering or sorting is active.
func queryConnections(conns []model.Connection, q ConnQuery, now time.Time) []model.Connection {
	if len(conns) == 0 {
		return nil
	}

	var result []model.Connection
	if !q.HasActiveFilter() {
		if q.SortBy == ConnSortDefault {
			return conns
		}
		result = make([]model.Connection, len(conns))
		copy(result, conns)
	} else {
		result = make([]model.Connection, 0, len(conns))
		for _, conn := range conns {
			if connMatchesQuery(conn, q) {
				result = append(result, conn)
			}
		}
	}

	if q.SortBy == ConnSortDefault || len(result) <= 1 {
		return result
	}

	sort.SliceStable(result, func(i, j int) bool {
		cmp := compareConnections(result[i], result[j], q.SortBy, now)
		if cmp == 0 {
			return false
		}
		if q.SortDesc {
			return cmp > 0
		}
		return cmp < 0
	})
	return result
}

func compareConnections(a, b model.Connection, by ConnSortField, now time.Time) int {
	switch by {
	case ConnSortSpeed:
		sa := a.DownloadSpeed + a.UploadSpeed
		sb := b.DownloadSpeed + b.UploadSpeed
		return compareInt64(sa, sb)
	case ConnSortTraffic:
		ta := a.Download + a.Upload
		tb := b.Download + b.Upload
		return compareInt64(ta, tb)
	case ConnSortDuration:
		da := connectionDuration(a, now)
		db := connectionDuration(b, now)
		return compareInt64(int64(da), int64(db))
	case ConnSortStart:
		ta := parseConnStart(a.Start)
		tb := parseConnStart(b.Start)
		switch {
		case ta.Before(tb):
			return -1
		case ta.After(tb):
			return 1
		default:
			return 0
		}
	default:
		return 0
	}
}

func compareInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func parseConnStart(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t
	}
	return time.Time{}
}

func connectionDuration(conn model.Connection, now time.Time) time.Duration {
	start := parseConnStart(conn.Start)
	if start.IsZero() {
		return 0
	}
	end := now
	if !conn.ClosedAt.IsZero() {
		end = conn.ClosedAt
	}
	d := end.Sub(start)
	if d < 0 {
		return 0
	}
	return d
}

// connMatchesQuery checks protocol filter, field-scoped text, and optional
// structured tokens (`proc:`, `src:`, `proto:`, `chain:`).
func connMatchesQuery(conn model.Connection, q ConnQuery) bool {
	if q.Protocol != "" && !strings.EqualFold(conn.Metadata.Network, q.Protocol) {
		return false
	}
	text := strings.TrimSpace(q.Text)
	if text == "" {
		return true
	}

	// If structured prefixes appear, evaluate every whitespace-separated token with AND semantics.
	tokens := strings.Fields(text)
	hasStructured := false
	for _, tok := range tokens {
		if _, _, ok := splitFilterToken(tok); ok {
			hasStructured = true
			break
		}
	}
	if hasStructured {
		for _, tok := range tokens {
			if prefix, val, ok := splitFilterToken(tok); ok {
				if val == "" {
					continue
				}
				switch prefix {
				case "proc", "process":
					if !matchConnField(conn, ConnFilterProcess, val) {
						return false
					}
				case "src", "ip", "source":
					if !matchConnField(conn, ConnFilterSourceIP, val) {
						return false
					}
				case "proto", "protocol", "net":
					if !strings.EqualFold(conn.Metadata.Network, val) {
						return false
					}
				case "chain", "proxy":
					if !matchConnField(conn, ConnFilterChain, val) {
						return false
					}
				}
				continue
			}
			if !matchConnField(conn, q.Field, strings.ToLower(tok)) {
				return false
			}
		}
		return true
	}

	return matchConnField(conn, q.Field, strings.ToLower(text))
}

func splitFilterToken(tok string) (prefix, val string, ok bool) {
	p, v, found := strings.Cut(tok, ":")
	if !found {
		return "", "", false
	}
	p = strings.ToLower(strings.TrimSpace(p))
	switch p {
	case "proc", "process", "src", "ip", "source", "proto", "protocol", "net", "chain", "proxy":
		return p, strings.ToLower(strings.TrimSpace(v)), true
	default:
		return "", "", false
	}
}

func matchConnField(conn model.Connection, field ConnFilterField, filter string) bool {
	if filter == "" {
		return true
	}
	switch field {
	case ConnFilterProcess:
		return strings.Contains(strings.ToLower(conn.Metadata.Process), filter) ||
			strings.Contains(strings.ToLower(conn.Metadata.ProcessPath), filter)
	case ConnFilterSourceIP:
		return strings.Contains(strings.ToLower(conn.Metadata.SourceIP), filter)
	case ConnFilterChain:
		if len(conn.Chains) == 0 {
			return strings.Contains("direct", filter)
		}
		return containsAnyLower(conn.Chains, filter)
	default:
		return connMatchesFilter(conn, filter)
	}
}

// connMatchesFilter 检查连接是否匹配通用过滤词（涵盖主机、目标/来源 IP、进程、协议、规则和代理链）
func connMatchesFilter(conn model.Connection, filter string) bool {
	if filter == "" {
		return true
	}
	if strings.Contains(strings.ToLower(conn.Metadata.Host), filter) ||
		strings.Contains(strings.ToLower(conn.Rule), filter) ||
		strings.Contains(strings.ToLower(conn.RulePayload), filter) ||
		strings.Contains(strings.ToLower(conn.Metadata.DestinationIP), filter) ||
		strings.Contains(strings.ToLower(conn.Metadata.SourceIP), filter) ||
		strings.Contains(strings.ToLower(conn.Metadata.Process), filter) ||
		strings.Contains(strings.ToLower(conn.Metadata.ProcessPath), filter) ||
		strings.Contains(strings.ToLower(conn.Metadata.Network), filter) {
		return true
	}
	if len(conn.Chains) == 0 && strings.Contains("direct", filter) {
		return true
	}
	return containsAnyLower(conn.Chains, filter)
}
