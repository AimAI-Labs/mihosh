package connections

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections/components"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	ConnViewTraffic = 0 // 流量监控（默认）
	ConnViewActive  = 1 // 活跃连接
	ConnViewHistory = 2 // 历史连接

	connsDoubleClickThreshold = 350 * time.Millisecond
	connsChartDoubleClickMax  = 650 * time.Millisecond
	connsTopNDefaultCount     = 5
)

// State 连接页面完整状态
type State struct {
	Connections *model.ConnectionsResponse
	PrevConnIDs map[string]model.Connection
	// Ring Buffer for closed connections
	closedConns [common.ClosedConnCap]model.Connection
	closedTimes [common.ClosedConnCap]time.Time // 新增：记录关闭时间
	closedHead  int                             // 写入位置（下一条写入的索引）
	closedCount int                             // 已写入的总条数（上限 ClosedConnCap）

	selectedConn            int
	connScrollTop           int
	connFilterMode          bool
	connFilter              textinput.Model
	connDetailMode          bool
	connDetailSnapshot      *model.Connection
	connIPInfo              *model.IPInfo
	connDetailLeftScroll    int
	connDetailRightScroll   int
	connDetailFocusPanel    int // 0=左侧(基础+地理), 1=右侧(JSON)
	connDetailJSONLineCount int // JSON行数缓存，用于滚动上限约束
	connViewMode            int // 0=流量监控, 1=活跃, 2=历史
	inlineDetailMode        bool // 新增：内联详情模式
	inlineDetailFocused     bool
	inlineDetailScroll      int

	siteTests        []model.SiteTest
	selectedSiteTest int
	proxyAddr        string
	topNModalMode    bool
	topNModalScroll  int

	lastMouseTarget MouseTarget
	lastMouseIndex  int
	lastMouseAt     time.Time
}

// NewConnsState 初始化连接状态
func NewState(proxyAddr string, siteTests []model.SiteTest) State {
	ti := textinput.New()
	ti.Placeholder = "filter..."
	ti.CharLimit = 100
	return State{
		proxyAddr:  proxyAddr,
		siteTests:  siteTests,
		connFilter: ti,
	}
}

// CalculateTopN 计算 Top N 吞吐量
func (s State) CalculateTopN(n int, within time.Duration) []components.TopNItem {
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

	// 排序
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].TotalBytes > items[i].TotalBytes {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	if n > 0 && len(items) > n {
		items = items[:n]
	}
	return items
}

// ClosedConnections 返回历史连接（最新在前）
func (s State) ClosedConnections() []model.Connection {
	if s.closedCount == 0 {
		return nil
	}
	result := make([]model.Connection, s.closedCount)
	// Ring Buffer: 从最新写入位置向前读
	for i := 0; i < s.closedCount; i++ {
		idx := (s.closedHead - 1 - i + common.ClosedConnCap) % common.ClosedConnCap
		result[i] = s.closedConns[idx]
	}
	return result
}

// appendClosed 向 Ring Buffer 追加一条历史连接
func (s *State) appendClosed(conn model.Connection) {
	s.closedConns[s.closedHead] = conn
	s.closedTimes[s.closedHead] = time.Now()
	s.closedHead = (s.closedHead + 1) % common.ClosedConnCap
	if s.closedCount < common.ClosedConnCap {
		s.closedCount++
	}
}

// ToPageState 转换为渲染层所需的 PageState
func (s State) ToPageState(chartData *model.ChartData, width, height int) PageState {
	topNItems := s.CalculateTopN(connsTopNDefaultCount, 5*time.Minute)
	var topNModalItems []components.TopNItem
	if s.topNModalMode {
		topNModalItems = s.CalculateTopN(0, 5*time.Minute)
	}

	return PageState{
		Connections:        s.Connections,
		Width:              width,
		Height:             height,
		SelectedIndex:      s.selectedConn,
		ScrollTop:          s.connScrollTop,
		FilterText:         s.connFilter.Value(),
		FilterInput:        s.connFilter.View(),
		FilterMode:         s.connFilterMode,
		DetailMode:         s.connDetailMode,
		SelectedConnection: s.connDetailSnapshot,
		IPInfo:             s.connIPInfo,
		DetailLeftScroll:   s.connDetailLeftScroll,
		DetailRightScroll:  s.connDetailRightScroll,
		DetailFocusPanel:   s.connDetailFocusPanel,
		ChartData:          chartData,
		ViewMode:           s.connViewMode,
		ClosedConnections:  s.ClosedConnections(),
		SiteTests:          s.siteTests,
		SelectedSiteTest:   s.selectedSiteTest,
		TopNItems:          topNItems,
		TopNModalMode:      s.topNModalMode,
		TopNModalItems:     topNModalItems,
		TopNModalScroll:    s.topNModalScroll,
		InlineMode:         s.inlineDetailMode,
		InlineFocused:      s.inlineDetailFocused,
		InlineScroll:       s.inlineDetailScroll,
	}
}

// Update 处理连接页面按键
func (s State) Update(msg tea.KeyMsg, client *api.Client, timeout int) (State, tea.Cmd) {
	// 详情模式
	if s.connDetailMode {
		switch {
		case key.Matches(msg, common.Keys.Escape), key.Matches(msg, common.Keys.Enter), msg.String() == "q":
			s.closeConnectionDetail()
		case key.Matches(msg, common.Keys.Left), msg.String() == "h":
			if s.connDetailFocusPanel > 0 {
				s.connDetailFocusPanel--
			}
		case key.Matches(msg, common.Keys.Right), msg.String() == "l":
			if s.connDetailFocusPanel < 1 {
				s.connDetailFocusPanel++
			}
		case key.Matches(msg, common.Keys.Up), msg.String() == "k":
			if s.connDetailFocusPanel == 0 {
				if s.connDetailLeftScroll > 0 {
					s.connDetailLeftScroll--
				}
			} else {
				if s.connDetailRightScroll > 0 {
					s.connDetailRightScroll--
				}
			}
		case key.Matches(msg, common.Keys.Down), msg.String() == "j":
			if s.connDetailFocusPanel == 0 {
				s.connDetailLeftScroll++
				s.clampLeftScroll()
			} else {
				s.connDetailRightScroll++
				s.clampRightScroll()
			}
		}
		return s, nil
	}

	if s.topNModalMode {
		switch {
		case key.Matches(msg, common.Keys.Escape), key.Matches(msg, common.Keys.Enter), msg.String() == "q":
			s.closeTopNModal()
		case key.Matches(msg, common.Keys.Up), msg.String() == "k":
			if s.topNModalScroll > 0 {
				s.topNModalScroll--
			}
		case key.Matches(msg, common.Keys.Down), msg.String() == "j":
			s.topNModalScroll++
		}
		return s, nil
	}

	// 过滤输入模式
	if s.connFilterMode {
		return s.handleConnFilterMode(msg)
	}

	// tab 切换键（所有 viewMode 通用）
	if msg.String() == "h" {
		s.setConnViewMode((s.connViewMode + 1) % 3)
		return s, nil
	}

	// 流量监控 tab：仅响应站点测速和选择
	if s.connViewMode == ConnViewTraffic {
		switch {
		case msg.String() == "s":
			return s.triggerSiteTestByIndex(s.selectedSiteTest, timeout)
		case msg.String() == "S":
			if len(s.siteTests) > 0 {
				for i := range s.siteTests {
					s.siteTests[i].Testing = true
				}
				return s, TestAllSites(s.proxyAddr, s.siteTests, timeout)
			}
		case key.Matches(msg, common.Keys.Left):
			if s.selectedSiteTest > 0 {
				s.selectedSiteTest--
			}
		case key.Matches(msg, common.Keys.Right):
			if s.selectedSiteTest < len(s.siteTests)-1 {
				s.selectedSiteTest++
			}
		}
		return s, nil
	}

	// 活跃/历史连接 tab：连接列表操作
	switch {
	case msg.String() == "i":
		if s.connViewMode == ConnViewActive || s.connViewMode == ConnViewHistory {
			s.inlineDetailMode = !s.inlineDetailMode
		}

	case key.Matches(msg, common.Keys.Up):
		if s.inlineDetailFocused {
			if s.inlineDetailScroll > 0 {
				s.inlineDetailScroll--
			}
		} else {
			if s.selectedConn > 0 {
				s.selectedConn--
				if s.selectedConn < s.connScrollTop {
					s.connScrollTop = s.selectedConn
				}
				s.inlineDetailScroll = 0 // 切换连接时重置内联详情滚动
			}
		}

	case key.Matches(msg, common.Keys.Down):
		if s.inlineDetailFocused {
			s.inlineDetailScroll++
		} else {
			connCount := s.filteredConnCount()
			if s.selectedConn < connCount-1 {
				s.selectedConn++
			}
			s.inlineDetailScroll = 0
		}

	case key.Matches(msg, common.Keys.Enter):
		return s.openSelectedConnectionDetail()

	case msg.String() == "x":
		if s.connViewMode == ConnViewActive {
			conn := s.selectedConnection()
			if conn != nil {
				return s, tea.Batch(
					CloseConnection(client, conn.ID),
					FetchConnections(client),
				)
			}
		}

	case msg.String() == "X":
		if s.connViewMode == ConnViewActive {
			return s, tea.Batch(
				CloseAllConnections(client),
				FetchConnections(client),
			)
		}

	case msg.String() == "/":
		s.connFilterMode = true
		s.connFilter.Focus()

	case key.Matches(msg, common.Keys.Escape):
		if s.inlineDetailFocused {
			s.inlineDetailFocused = false
		} else if s.connFilter.Value() != "" {
			s.connFilter.Reset()
			s.selectedConn = 0
			s.connScrollTop = 0
		}
	}

	return s, nil
}

// HandleMouseLeft 处理 connections 页面左键单击/双击
func (s State) HandleMouseLeft(
	pageX int,
	pageY int,
	pageWidth int,
	pageHeight int,
	chartData *model.ChartData,
	timeout int,
) (State, tea.Cmd) {
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
			if s.isMouseDoubleClick(ConnectionsMouseTargetNone, 0, now) {
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
			if s.isMouseDoubleClick(MouseTargetTopNModalItem, hit.Index, now) {
				items := s.CalculateTopN(0, 5*time.Minute)
				if hit.Index >= 0 && hit.Index < len(items) {
					item := items[hit.Index]
					// 查找匹配的连接
					if conn := s.findConnectionByName(item.Name); conn != nil {
						// 不要关闭 topNModalMode，详情渲染优先级更高
						s.connDetailMode = true
						s.connDetailSnapshot = conn
						s.connDetailLeftScroll = 0
						s.connDetailRightScroll = 0
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
		left, top, right, bottom := components.ResolveTopNModalBounds(items, pageWidth, pageHeight, s.topNModalScroll)
		insideModal := pageX >= left && pageX < right && pageY >= top && pageY < bottom
		if !insideModal {
			s.closeTopNModal()
		}
		return s, nil
	}

	if s.connFilterMode {
		return s, nil
	}

	hit := ResolveMouseHit(s.ToPageState(chartData, pageWidth, pageHeight), pageX, pageY)
	now := time.Now()

	if s.inlineDetailFocused && hit.Target != MouseTargetInlineDetail {
		s.inlineDetailFocused = false
	}

	switch hit.Target {
	case MouseTargetChart, MouseTargetTopN:
		if s.isMouseDoubleClickWithThreshold(hit.Target, 0, now, connsChartDoubleClickMax) {
			s.topNModalMode = true
			s.topNModalScroll = 0
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
		if s.selectedConn != hit.Index {
			s.selectedConn = hit.Index
			s.inlineDetailScroll = 0
			if s.selectedConn < s.connScrollTop {
				s.connScrollTop = s.selectedConn
			}
		}
		if !s.isMouseDoubleClick(MouseTargetConnection, hit.Index, now) {
			return s, nil
		}
		return s.openSelectedConnectionDetail()

	case MouseTargetSiteTest:
		if hit.Index < 0 || hit.Index >= len(s.siteTests) {
			return s, nil
		}
		s.selectedSiteTest = hit.Index
		if !s.isMouseDoubleClick(MouseTargetSiteTest, hit.Index, now) {
			return s, nil
		}
		return s.triggerSiteTestByIndex(hit.Index, timeout)
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
				if s.connDetailRightScroll > 0 {
					s.connDetailRightScroll--
				}
			} else {
				if s.connDetailLeftScroll > 0 {
					s.connDetailLeftScroll--
				}
			}
		} else {
			if isRightSide {
				s.connDetailRightScroll++
				s.clampRightScroll()
			} else {
				s.connDetailLeftScroll++
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
		if up {
			if s.topNModalScroll > 0 {
				s.topNModalScroll--
			}
		} else {
			s.topNModalScroll++
		}
		return s, nil
	}

	if s.inlineDetailFocused {
		if up {
			if s.inlineDetailScroll > 0 {
				s.inlineDetailScroll--
			}
		} else {
			s.inlineDetailScroll++
		}
		return s, nil
	}

	count := s.filteredConnCount()
	if up {
		if s.selectedConn > 0 {
			s.selectedConn--
			if s.selectedConn < s.connScrollTop {
				s.connScrollTop = s.selectedConn
			}
			s.inlineDetailScroll = 0
		}
	} else {
		if s.selectedConn < count-1 {
			s.selectedConn++
			s.inlineDetailScroll = 0
		}
	}

	return s, nil
}

func (s *State) isMouseDoubleClick(target MouseTarget, idx int, now time.Time) bool {
	return s.isMouseDoubleClickWithThreshold(target, idx, now, connsDoubleClickThreshold)
}

func (s *State) isMouseDoubleClickWithThreshold(target MouseTarget, idx int, now time.Time, threshold time.Duration) bool {
	isDoubleClick := target == s.lastMouseTarget &&
		idx == s.lastMouseIndex &&
		!s.lastMouseAt.IsZero() &&
		now.Sub(s.lastMouseAt) <= threshold

	s.lastMouseTarget = target
	s.lastMouseIndex = idx
	s.lastMouseAt = now

	return isDoubleClick
}

// ApplyWSConnections 处理 WebSocket 连接推送（含历史记录检测）
func (s State) ApplyWSConnections(data api.ConnectionsData) State {
	currentIDs := make(map[string]model.Connection, len(data.Connections))
	for _, conn := range data.Connections {
		currentIDs[conn.ID] = model.Connection{
			ID:            conn.ID,
			Upload:        conn.Upload,
			Download:      conn.Download,
			Start:         conn.Start,
			Chains:        conn.Chains,
			Rule:          conn.Rule,
			RulePayload:   conn.RulePayload,
			DownloadSpeed: conn.DownloadSpeed,
			UploadSpeed:   conn.UploadSpeed,
			Metadata: model.Metadata{
				Network:         conn.Metadata.Network,
				Type:            conn.Metadata.Type,
				SourceIP:        conn.Metadata.SourceIP,
				DestinationIP:   conn.Metadata.DestinationIP,
				SourcePort:      conn.Metadata.SourcePort,
				DestinationPort: conn.Metadata.DestinationPort,
				Host:            conn.Metadata.Host,
				Process:         conn.Metadata.Process,
				ProcessPath:     conn.Metadata.ProcessPath,
			},
		}
	}

	// 检测已关闭的连接写入 Ring Buffer
	if s.PrevConnIDs != nil {
		for id, conn := range s.PrevConnIDs {
			if _, exists := currentIDs[id]; !exists {
				s.appendClosed(conn)
			}
		}
	}
	s.PrevConnIDs = currentIDs
	s.Connections = ConvertToConnectionsResponse(data)
	return s
}

// ApplyConnections 应用 REST API 返回的连接数据
func (s State) ApplyConnections(resp *model.ConnectionsResponse) State {
	s.Connections = resp
	return s
}

// ApplySiteTestResult 应用网站测速结果
func (s State) ApplySiteTestResult(name string, delay int, err error) State {
	for i := range s.siteTests {
		if s.siteTests[i].Name == name {
			s.siteTests[i].Testing = false
			if err != nil {
				s.siteTests[i].Delay = 0
				s.siteTests[i].Error = "timeout"
			} else {
				s.siteTests[i].Delay = delay
				s.siteTests[i].Error = ""
			}
			break
		}
	}
	return s
}

// ApplyConnectionClosed 处理单连接关闭后的索引调整
func (s State) ApplyConnectionClosed() State {
	if s.selectedConn > 0 {
		s.selectedConn--
	}
	// 确保选中索引不超过连接数量上限
	connCount := s.filteredConnCount()
	if connCount > 0 && s.selectedConn >= connCount {
		s.selectedConn = connCount - 1
	}
	if s.selectedConn < 0 {
		s.selectedConn = 0
	}
	return s
}

// ApplyAllConnectionsClosed 所有连接关闭后重置状态
func (s State) ApplyAllConnectionsClosed() State {
	s.selectedConn = 0
	s.connScrollTop = 0
	return s
}

// ApplyIPInfo 更新 IP 地理信息
func (s State) ApplyIPInfo(info *model.IPInfo) State {
	s.connIPInfo = info
	return s
}

// ResetPrevConnIDs 重置连接快照（切换到连接页时调用）
func (s State) ResetPrevConnIDs() State {
	s.PrevConnIDs = nil
	return s
}

// UpdateProxyAddr 更新代理地址（设置页保存后调用）
func (s State) UpdateProxyAddr(addr string) State {
	s.proxyAddr = addr
	return s
}

// ViewMode 返回当前连接视图模式
func (s State) ViewMode() int { return s.connViewMode }

// DetailMode 返回是否处于详情模式
func (s State) DetailMode() bool { return s.connDetailMode }

// TopNModalMode 返回是否处于 TopN 弹窗模式
func (s State) TopNModalMode() bool { return s.topNModalMode }

// FilterMode 返回是否处于过滤模式
func (s State) FilterMode() bool { return s.connFilterMode }

func (s State) openSelectedConnectionDetail() (State, tea.Cmd) {
	conn := s.selectedConnection()
	if conn == nil {
		return s, nil
	}

	snapshot := *conn
	s.connDetailSnapshot = &snapshot
	s.connDetailMode = true
	s.connIPInfo = nil
	s.connDetailJSONLineCount = countConnJSONLines(&snapshot)
	return s, FetchIPInfo(conn.Metadata.DestinationIP)
}

func (s State) triggerSiteTestByIndex(idx int, timeout int) (State, tea.Cmd) {
	if idx < 0 || idx >= len(s.siteTests) {
		return s, nil
	}
	site := s.siteTests[idx]
	s.siteTests[idx].Testing = true
	return s, TestSiteDelay(s.proxyAddr, site.Name, site.URL, timeout)
}

func (s *State) closeConnectionDetail() {
	s.connDetailMode = false
	s.connDetailSnapshot = nil
	s.connIPInfo = nil
	s.connDetailLeftScroll = 0
	s.connDetailRightScroll = 0
	s.connDetailFocusPanel = 0
	s.connDetailJSONLineCount = 0
}

// clampRightScroll 约束右侧(JSON)滚动偏移上限
func (s *State) clampRightScroll() {
	if s.connDetailJSONLineCount > 0 {
		maxScroll := s.connDetailJSONLineCount - 5 // 5 = 最小可视行数
		if maxScroll < 0 {
			maxScroll = 0
		}
		if s.connDetailRightScroll > maxScroll {
			s.connDetailRightScroll = maxScroll
		}
	}
}

// clampLeftScroll 约束左侧滚动偏移上限（左侧内容行数有限，使用静态上限）
func (s *State) clampLeftScroll() {
	const maxLeftScroll = 50
	if s.connDetailLeftScroll > maxLeftScroll {
		s.connDetailLeftScroll = maxLeftScroll
	}
}

// countConnJSONLines 计算连接JSON的行数
func countConnJSONLines(conn *model.Connection) int {
	if conn == nil {
		return 0
	}
	data, err := json.MarshalIndent(conn, "", "  ")
	if err != nil {
		return 0
	}
	return len(strings.Split(string(data), "\n"))
}

func (s *State) closeTopNModal() {
	s.topNModalMode = false
	s.topNModalScroll = 0
}

func (s *State) setConnViewMode(mode int) {
	if mode < ConnViewTraffic || mode > ConnViewHistory {
		mode = ConnViewTraffic
	}
	s.connViewMode = mode
	s.selectedConn = 0
	s.connScrollTop = 0
}

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

// selectedConnection 获取当前选中的连接
func (s State) selectedConnection() *model.Connection {
	if s.connViewMode == ConnViewTraffic {
		return nil
	}
	var conns []model.Connection
	if s.connViewMode == ConnViewActive {
		if s.Connections == nil || len(s.Connections.Connections) == 0 {
			return nil
		}
		conns = s.Connections.Connections
	} else {
		closed := s.ClosedConnections()
		if len(closed) == 0 {
			return nil
		}
		conns = closed
	}

	if s.connFilter.Value() == "" {
		if s.selectedConn >= 0 && s.selectedConn < len(conns) {
			return &conns[s.selectedConn]
		}
		return nil
	}

	filter := strings.ToLower(s.connFilter.Value())
	idx := 0
	for i := range conns {
		if connMatchesFilter(conns[i], filter) {
			if idx == s.selectedConn {
				return &conns[i]
			}
			idx++
		}
	}
	return nil
}

// findConnectionByName 根据名称（进程/域名/IP）查找第一个匹配的连接
func (s State) findConnectionByName(name string) *model.Connection {
	if name == "" {
		return nil
	}

	// 1. 查找活跃连接
	if s.Connections != nil {
		for i := range s.Connections.Connections {
			conn := &s.Connections.Connections[i]
			if conn.Metadata.Process == name || conn.Metadata.Host == name || conn.Metadata.DestinationIP == name {
				return conn
			}
		}
	}

	// 2. 查找历史连接
	closed := s.ClosedConnections()
	for i := range closed {
		conn := &closed[i]
		if conn.Metadata.Process == name || conn.Metadata.Host == name || conn.Metadata.DestinationIP == name {
			return conn
		}
	}

	return nil
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
