package connections

import (
	"sort"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections/components"
)

type topNCache struct {
	items    []components.TopNItem
	connGen  uint64
	lastCalc time.Time
}

// CalculateTopN 计算 Top N 吞吐量 (使用缓存)
func (s State) CalculateTopN(n int, within time.Duration) []components.TopNItem {
	var fullItems []components.TopNItem

	if s.topNCache != nil {
		now := time.Now()
		if s.topNCache.connGen == s.connGen && now.Sub(s.topNCache.lastCalc) < time.Second {
			fullItems = s.topNCache.items
		} else {
			fullItems = s.calculateTopNUncached(0, within)
			s.topNCache.items = fullItems
			s.topNCache.connGen = s.connGen
			s.topNCache.lastCalc = now
		}
	} else {
		fullItems = s.calculateTopNUncached(0, within)
	}

	if n > 0 && len(fullItems) > n {
		return fullItems[:n]
	}
	return fullItems
}

// calculateTopNUncached 实际计算 Top N，使用 sort.Slice 优化排序
func (s State) calculateTopNUncached(n int, within time.Duration) []components.TopNItem {
	stats := make(map[string]int64)
	now := time.Now()

	// 汇总活跃连接
	if s.Connections != nil {
		for _, conn := range s.Connections.Connections {
			name := conn.Metadata.Process
			if name == "" {
				name = conn.Metadata.Host
			}
			if name == "" {
				name = conn.Metadata.DestinationIP
			}
			stats[name] += conn.Download + conn.Upload
		}
	}

	// 汇总历史连接
	for i := 0; i < s.closedCount; i++ {
		idx := (s.closedHead - 1 - i + common.ClosedConnCap) % common.ClosedConnCap
		closedTime := s.closedTimes[idx]
		if now.Sub(closedTime) <= within {
			conn := s.closedConns[idx]
			name := conn.Metadata.Process
			if name == "" {
				name = conn.Metadata.Host
			}
			if name == "" {
				name = conn.Metadata.DestinationIP
			}
			stats[name] += conn.Download + conn.Upload
		}
	}

	var items []components.TopNItem
	for k, v := range stats {
		if k != "" && v > 0 {
			items = append(items, components.TopNItem{Name: k, TotalBytes: v})
		}
	}

	// 排序: 使用标准库 sort.Slice
	sort.Slice(items, func(i, j int) bool {
		return items[i].TotalBytes > items[j].TotalBytes
	})

	if n > 0 && len(items) > n {
		items = items[:n]
	}
	return items
}

// TopNModalMode 返回是否处于 TopN 弹窗模式
func (s State) TopNModalMode() bool { return s.topNModalMode }

func (s *State) closeTopNModal() {
	s.topNModalMode = false
	s.topNModalPanel.Reset()
}
