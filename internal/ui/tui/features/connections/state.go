package connections

import (
	"context"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections/components"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	ConnViewTraffic = 0 // 流量监控（默认）
	ConnViewActive  = 1 // 活跃连接
	ConnViewHistory = 2 // 历史连接

	connsChartDoubleClickMax = 650 * time.Millisecond
	connsTopNDefaultCount    = 5
)

// State 连接页面完整状态
type State struct {
	Connections *model.ConnectionsResponse
	PrevConnIDs map[string]model.Connection
	// Ring Buffer for closed connections
	closedConns       [common.ClosedConnCap]model.Connection
	closedTimes       [common.ClosedConnCap]time.Time // 新增：记录关闭时间
	closedHead        int                             // 写入位置（下一条写入的索引）
	closedCount       int                             // 已写入的总条数（上限 ClosedConnCap）
	cachedClosedConns []model.Connection              // 缓存的已关闭连接列表

	filterList common.FilterList
	connDetailMode          bool
	connDetailSnapshot      *model.Connection
	connIPInfo              *model.IPInfo
	connDetailLeftPanel     common.ScrollablePanel
	connDetailRightPanel    common.ScrollablePanel
	connDetailFocusPanel    int  // 0=左侧(基础+地理), 1=右侧(JSON)
	connDetailJSONLineCount int  // JSON行数缓存，用于滚动上限约束
	connViewMode            int  // 0=流量监控, 1=活跃, 2=历史
	inlineDetailMode        bool // 新增：内联详情模式
	inlineDetailFocused     bool
	inlineDetailPanel       common.ScrollablePanel

	siteTests        []model.SiteTest
	selectedSiteTest int
	proxyAddr        string
	topNModalMode    bool
	topNModalPanel   common.ScrollablePanel

	doubleClickDetector common.DoubleClickDetector[MouseTarget]

	connGen   uint64
	topNCache *topNCache

	client    *api.Client
	timeout   int
	chartData *model.ChartData
	
	testCtx    context.Context
	testCancel context.CancelFunc
}

// NewState 初始化连接状态
func NewState(proxyAddr string, siteTests []model.SiteTest, client *api.Client, timeout int, chartData *model.ChartData) State {
	fl := common.NewFilterList()
	return State{
		proxyAddr:  proxyAddr,
		siteTests:  siteTests,
		filterList: fl,
		topNCache:  &topNCache{},
		client:     client,
		timeout:    timeout,
		chartData:  chartData,
	}
}

// ClosedConnections 返回历史连接（最新在前）
func (s State) ClosedConnections() []model.Connection {
	return s.cachedClosedConns
}

// rebuildCachedClosedConns 重建历史连接列表缓存
func (s *State) rebuildCachedClosedConns() {
	if s.closedCount == 0 {
		s.cachedClosedConns = nil
		return
	}
	if cap(s.cachedClosedConns) < s.closedCount {
		s.cachedClosedConns = make([]model.Connection, s.closedCount)
	} else {
		s.cachedClosedConns = s.cachedClosedConns[:s.closedCount]
	}
	for i := 0; i < s.closedCount; i++ {
		idx := (s.closedHead - 1 - i + common.ClosedConnCap) % common.ClosedConnCap
		s.cachedClosedConns[i] = s.closedConns[idx]
	}
}

// appendClosed 向 Ring Buffer 追加一条历史连接
func (s *State) appendClosed(conn model.Connection) {
	s.connGen++
	s.closedConns[s.closedHead] = conn
	s.closedTimes[s.closedHead] = time.Now()
	s.closedHead = (s.closedHead + 1) % common.ClosedConnCap
	if s.closedCount < common.ClosedConnCap {
		s.closedCount++
	}
}

// CancelTest 取消当前正在进行的测试
func (s *State) CancelTest() {
	if s.testCancel != nil {
		s.testCancel()
		s.testCancel = nil
	}
}

// ToPageState 转换为渲染层所需的 PageState
func (s State) ToPageState(chartData *model.ChartData, width, height int) PageState {
	var topNItems []components.TopNItem
	if s.connViewMode == ConnViewTraffic {
		topNItems = s.CalculateTopN(connsTopNDefaultCount, 5*time.Minute)
	}

	var topNModalItems []components.TopNItem
	if s.topNModalMode {
		topNModalItems = s.CalculateTopN(0, 5*time.Minute)
	}

	return PageState{
		Connections:        s.Connections,
		Width:              width,
		Height:             height,
		SelectedIndex:      s.filterList.Cursor,
		ScrollTop:          s.filterList.ScrollTop,
		FilterText:         s.filterList.FilterValue(),
		FilterInput:        s.filterList.FilterView(),
		FilterMode:         s.filterList.FilterMode(),
		DetailMode:         s.connDetailMode,
		SelectedConnection: s.connDetailSnapshot,
		IPInfo:             s.connIPInfo,
		DetailLeftScroll:   s.connDetailLeftPanel.ScrollTop,
		DetailRightScroll:  s.connDetailRightPanel.ScrollTop,
		DetailFocusPanel:   s.connDetailFocusPanel,
		ChartData:          chartData,
		ViewMode:           s.connViewMode,
		ClosedConnections:  s.ClosedConnections(),
		SiteTests:          s.siteTests,
		SelectedSiteTest:   s.selectedSiteTest,
		TopNItems:          topNItems,
		TopNModalMode:      s.topNModalMode,
		TopNModalItems:     topNModalItems,
		TopNModalScroll:    s.topNModalPanel.ScrollTop,
		InlineMode:         s.inlineDetailMode,
		InlineFocused:      s.inlineDetailFocused,
		InlineScroll:       s.inlineDetailPanel.ScrollTop,
	}
}

// Update 处理连接页面按键和鼠标事件
func (s State) Update(msg tea.Msg) (State, tea.Cmd) {
	switch msg := msg.(type) {
	case messages.PageMouseScrollMsg:
		return s.HandleMouseScroll(msg.Up, msg.X, msg.Y, msg.Width, msg.Height)
	case messages.PageMouseClickMsg:
		return s.HandleMouseLeft(msg.X, msg.Y, msg.Width, msg.Height)
	case tea.KeyMsg:
		// 继续处理 KeyMsg
	if s.connDetailMode {
		switch {
		case key.Matches(msg, common.Keys.Escape), key.Matches(msg, common.Keys.Enter), msg.String() == "q":
			s.closeConnectionDetail()
			return s, nil
		case key.Matches(msg, common.Keys.Left), msg.String() == "h":
			if s.connDetailFocusPanel > 0 {
				s.connDetailFocusPanel--
			}
			return s, nil
		case key.Matches(msg, common.Keys.Right), msg.String() == "l":
			if s.connDetailFocusPanel < 1 {
				s.connDetailFocusPanel++
			}
			return s, nil
		}

		if s.connDetailFocusPanel == 0 {
			s.connDetailLeftPanel.Update(msg)
			s.clampLeftScroll() // apply specific static limits if needed
		} else {
			s.connDetailRightPanel.Update(msg)
			s.clampRightScroll() // apply specific json limit if needed
		}
		return s, nil
	}

	if s.topNModalMode {
		switch {
		case key.Matches(msg, common.Keys.Escape), key.Matches(msg, common.Keys.Enter), msg.String() == "q":
			s.closeTopNModal()
			return s, nil
		}
		s.topNModalPanel.Update(msg)
		return s, nil
	}

	// 过滤输入模式
	if s.filterList.FilterMode() {
		s.filterList.SetItemCount(s.filteredConnCount())
		_, cmd := s.filterList.Update(msg)
		s.filterList.SetItemCount(s.filteredConnCount()) // Update again after filter string changes
		return s, cmd
	}

	// tab 切换键（所有 viewMode 通用）
	if msg.String() == "h" {
		s.CancelTest()
		s.setConnViewMode((s.connViewMode + 1) % 3)
		return s, nil
	}

	// 流量监控 tab：仅响应站点测速和选择
	if s.connViewMode == ConnViewTraffic {
		switch {
		case msg.String() == "s":
			return s.triggerSiteTestByIndex(s.selectedSiteTest, s.timeout)
		case msg.String() == "S":
			if len(s.siteTests) > 0 {
				for i := range s.siteTests {
					s.siteTests[i].Testing = true
				}
				s.CancelTest()
				s.testCtx, s.testCancel = context.WithCancel(context.Background())
				return s, TestAllSites(s.testCtx, s.proxyAddr, s.siteTests, s.timeout)
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
			s.inlineDetailPanel.Update(msg)
		} else {
			s.filterList.SetItemCount(s.filteredConnCount())
			s.filterList.Update(msg)
			s.inlineDetailPanel.Reset()
		}

	case key.Matches(msg, common.Keys.Down):
		if s.inlineDetailFocused {
			s.inlineDetailPanel.Update(msg)
		} else {
			s.filterList.SetItemCount(s.filteredConnCount())
			s.filterList.Update(msg)
			s.inlineDetailPanel.Reset()
		}

	case key.Matches(msg, common.Keys.Enter):
		return s.openSelectedConnectionDetail()

	case msg.String() == "x":
		if s.connViewMode == ConnViewActive {
			conn := s.selectedConnection()
			if conn != nil {
				return s, tea.Batch(
					CloseConnection(s.client, conn.ID),
					FetchConnections(s.client),
				)
			}
		}

	case msg.String() == "X":
		if s.connViewMode == ConnViewActive {
			return s, tea.Batch(
				CloseAllConnections(s.client),
				FetchConnections(s.client),
			)
		}

	case msg.String() == "/":
		s.filterList.Update(msg)

	case key.Matches(msg, common.Keys.Escape):
		if s.inlineDetailFocused {
			s.inlineDetailFocused = false
		} else if s.filterList.FilterValue() != "" {
			s.filterList.ResetFilter()
		}
	}

	}

	return s, nil
}

// ViewMode 返回当前连接视图模式
func (s State) ViewMode() int { return s.connViewMode }

// FilterMode 返回是否处于过滤模式
func (s State) FilterMode() bool { return s.filterList.FilterMode() }

func (s State) triggerSiteTestByIndex(idx int, timeout int) (State, tea.Cmd) {
	if idx < 0 || idx >= len(s.siteTests) {
		return s, nil
	}
	s.CancelTest()
	s.testCtx, s.testCancel = context.WithCancel(context.Background())
	site := s.siteTests[idx]
	s.siteTests[idx].Testing = true
	return s, TestSiteDelay(s.testCtx, s.proxyAddr, site.Name, site.URL, timeout)
}

func (s *State) setConnViewMode(mode int) {
	if mode < ConnViewTraffic || mode > ConnViewHistory {
		mode = ConnViewTraffic
	}
	s.connViewMode = mode
	s.filterList.SetCursor(0)
	s.filterList.ScrollTop = 0
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

	if s.filterList.FilterValue() == "" {
		if s.filterList.Cursor >= 0 && s.filterList.Cursor < len(conns) {
			return &conns[s.filterList.Cursor]
		}
		return nil
	}

	filter := strings.ToLower(s.filterList.FilterValue())
	idx := 0
	for i := range conns {
		if connMatchesFilter(conns[i], filter) {
			if idx == s.filterList.Cursor {
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
