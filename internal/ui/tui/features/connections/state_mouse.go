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
	if s.dns.open {
		return s.clickDNS(pageX, pageY, pageWidth, pageHeight)
	}
	if s.unlock.open {
		return s.clickUnlock(pageX, pageY, pageWidth, pageHeight)
	}
	if s.currentIP.modalOpen {
		return s.clickCurrentIPModal(pageX, pageY, pageWidth, pageHeight)
	}
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
		return s.EnsureCurrentIP()

	case MouseTargetIPMaskToggle:
		return s.ToggleCurrentIPMask(), nil

	case MouseTargetIPRefresh:
		return s.RefreshCurrentIP()

	case MouseTargetIPDetail:
		s.currentIP.width = pageWidth
		s.currentIP.height = pageHeight
		s.openCurrentIPModal()
		return s, nil

	case MouseTargetViewActive:
		s.setConnViewMode(ConnViewActive)
		return s, nil

	case MouseTargetViewHistory:
		s.setConnViewMode(ConnViewHistory)
		return s, nil

	case MouseTargetPauseToggle:
		if s.connViewMode == ConnViewActive || s.connViewMode == ConnViewHistory {
			s = s.TogglePause()
		}
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
	if s.dns.open {
		if up {
			if s.dns.scroll > 0 {
				s.dns.scroll--
			}
		} else {
			s.dns.scroll++
		}
		return s, nil
	}
	if s.unlock.open {
		s.unlock.width = mainWidth
		delta := unlockColumns(mainWidth)
		if up {
			delta = -delta
		}
		s.unlock.move(delta)
		return s, nil
	}
	if s.currentIP.modalOpen {
		return s.scrollCurrentIPModal(up, mainWidth, mainHeight), nil
	}
	if s.connDetailMode {
		layout := layoutConnectionDetail(s.ToPageState(s.chartData, mainWidth, mainHeight))
		panel := layout.detail.PanelAt(mainX, mainY-layout.detailY)
		switch panel {
		case 0:
			s.connDetailLeftPanel.HandleMouseScroll(up)
			if !up {
				s.clampLeftScroll()
			}
		case 1:
			s.connDetailRightPanel.HandleMouseScroll(up)
			if !up {
				s.clampRightScroll()
			}
		default:
			return s, nil
		}
		s.connDetailFocusPanel = panel

		return s, nil
	}

	if s.topNModalMode {
		s.topNModalPanel.HandleMouseScroll(up)
		return s, nil
	}
	if s.FilterMode() {
		return s, nil
	}

	layout := layoutConnectionList(s.ToPageState(s.chartData, mainWidth, mainHeight))
	if layout.hit(mainX, mainY, mainWidth).Target == MouseTargetInlineDetail {
		s.inlineDetailFocused = true
		s.inlineDetailPanel.HandleMouseScroll(up)
		return s, nil
	}
	if mainX < 0 || mainX >= mainWidth || mainY < layout.rowsY || mainY >= layout.rowsY+layout.listHeight {
		return s, nil
	}
	s.inlineDetailFocused = false

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
