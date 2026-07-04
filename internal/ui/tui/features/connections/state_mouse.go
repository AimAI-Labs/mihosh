package connections

import (
	"time"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections/components"
	tea "github.com/charmbracelet/bubbletea"
)

// HandleMouseLeft 处理 connections 页面左键单击/双击
func (s State) HandleMouseLeft(
	pageX, pageY, pageWidth, pageHeight int,
) (State, tea.Cmd) {
	chartData := s.chartData
	timeout := s.timeout

	if s.connDetailMode {
		if s.connDetailSnapshot == nil {
			s.closeConnectionDetail()
			return s, nil
		}

		hit := ResolveMouseHit(s.ToPageState(chartData, pageWidth, pageHeight), pageX, pageY)
		if hit.Target == MouseTargetViewTraffic || hit.Target == MouseTargetViewActive || hit.Target == MouseTargetViewHistory {
			s.closeConnectionDetail()
			// 落下到下方的 switch hit.Target 中处理模式切换
		} else {
			now := time.Now()
			// 在详情页区域双击则退出
			if s.doubleClickDetector.IsDoubleClick(ConnectionsMouseTargetNone, 0, now) {
				s.closeConnectionDetail()
			}
			return s, nil
		}
	}

	if s.topNModalMode {
		// 先尝试 ResolveMouseHit
		hit := ResolveMouseHit(s.ToPageState(chartData, pageWidth, pageHeight), pageX, pageY)
		now := time.Now()

		if hit.Target == MouseTargetTopNModalItem {
			if s.doubleClickDetector.IsDoubleClick(MouseTargetTopNModalItem, hit.Index, now) {
				items := s.CalculateTopN(0, 5*time.Minute)
				if hit.Index >= 0 && hit.Index < len(items) {
					item := items[hit.Index]
					// 查找匹配的连接
					if conn := s.findConnectionByName(item.Name); conn != nil {
						// 不要关闭 topNModalMode，详情渲染优先级更高
						s.connDetailMode = true
						s.connDetailSnapshot = conn
						s.connDetailLeftPanel.Reset()
						s.connDetailRightPanel.Reset()
						s.connDetailFocusPanel = 0
						s.connIPInfo = nil
						s.connDetailJSONLineCount = countConnJSONLines(conn)
						return s, FetchIPInfo(conn.Metadata.DestinationIP)
					}
				}
			}
			return s, nil
		}

		// 点击在弹窗外则关闭
		items := s.CalculateTopN(0, 5*time.Minute)
		left, top, right, bottom := components.ResolveTopNModalBounds(items, pageWidth, pageHeight, s.topNModalPanel.ScrollTop)
		insideModal := pageX >= left && pageX < right && pageY >= top && pageY < bottom
		if !insideModal {
			s.closeTopNModal()
		}
		return s, nil
	}

	if s.filterList.FilterMode() {
		return s, nil
	}

	hit := ResolveMouseHit(s.ToPageState(chartData, pageWidth, pageHeight), pageX, pageY)
	now := time.Now()

	if s.inlineDetailFocused && hit.Target != MouseTargetInlineDetail {
		s.inlineDetailFocused = false
	}

	switch hit.Target {
	case MouseTargetChart, MouseTargetTopN:
		if s.doubleClickDetector.IsDoubleClickWithThreshold(hit.Target, 0, now, connsChartDoubleClickMax) {
			s.topNModalMode = true
			s.topNModalPanel.Reset()
		}
		return s, nil

	case MouseTargetViewTraffic:
		s.setConnViewMode(ConnViewTraffic)
		return s, nil

	case MouseTargetViewActive:
		s.setConnViewMode(ConnViewActive)
		return s, nil

	case MouseTargetViewHistory:
		s.setConnViewMode(ConnViewHistory)
		return s, nil

	case MouseTargetInlineToggle:
		if s.connViewMode == ConnViewActive || s.connViewMode == ConnViewHistory {
			s.inlineDetailMode = !s.inlineDetailMode
		}
		if s.inlineDetailFocused {
			s.inlineDetailFocused = false
		}
		return s, nil

	case MouseTargetInlineDetail:
		s.inlineDetailFocused = true
		return s, nil

	case MouseTargetConnection:
		if s.inlineDetailFocused {
			s.inlineDetailFocused = false
		}
		if hit.Index < 0 {
			return s, nil
		}
		if s.filterList.Cursor != hit.Index {
			s.filterList.SetCursor(hit.Index)
			s.inlineDetailPanel.Reset()
		}
		if !s.doubleClickDetector.IsDoubleClick(MouseTargetConnection, hit.Index, now) {
			return s, nil
		}
		return s.openSelectedConnectionDetail()

	case MouseTargetSiteTest:
		idx := hit.Index
		if idx < 0 || idx >= len(s.siteTests) {
			return s, nil
		}
		s.selectedSiteTest = idx
		if s.doubleClickDetector.IsDoubleClick(MouseTargetSiteTest, idx, now) {
			s.selectedSiteTest = idx
			return s.triggerSiteTestByIndex(idx, timeout)
		}
	}

	return s, nil
}

// HandleMouseScroll 鼠标滚轮处理
func (s State) HandleMouseScroll(up bool, mainX, mainY, mainWidth, mainHeight int) (State, tea.Cmd) {
	if s.connDetailMode {
		isRightSide := false
		if mainWidth >= 100 {
			// 宽屏布局，左右排布，分界点大概是 mainWidth / 3
			isRightSide = mainX > (mainWidth / 3)
		} else {
			// 窄屏布局，上下排布，分界点大概是 mainHeight / 2
			isRightSide = mainY > (mainHeight / 2)
		}

		if up {
			if isRightSide {
				s.connDetailRightPanel.HandleMouseScroll(up)
			} else {
				s.connDetailLeftPanel.HandleMouseScroll(up)
			}
		} else {
			if isRightSide {
				s.connDetailRightPanel.HandleMouseScroll(up)
				s.clampRightScroll()
			} else {
				s.connDetailLeftPanel.HandleMouseScroll(up)
				s.clampLeftScroll()
			}
		}

		// 根据鼠标位置自动设置焦点
		if isRightSide {
			s.connDetailFocusPanel = 1
		} else {
			s.connDetailFocusPanel = 0
		}

		return s, nil
	}

	if s.topNModalMode {
		s.topNModalPanel.HandleMouseScroll(up)
		return s, nil
	}

	if s.inlineDetailFocused {
		s.inlineDetailPanel.HandleMouseScroll(up)
		return s, nil
	}

	count := s.filteredConnCount()
	s.filterList.SetItemCount(count)
	if up {
		if s.filterList.Cursor > 0 {
			s.filterList.SetCursor(s.filterList.Cursor - 1)
			s.inlineDetailPanel.Reset()
		}
	} else {
		if s.filterList.Cursor < count-1 {
			s.filterList.SetCursor(s.filterList.Cursor + 1)
			s.inlineDetailPanel.Reset()
		}
	}

	return s, nil
}
