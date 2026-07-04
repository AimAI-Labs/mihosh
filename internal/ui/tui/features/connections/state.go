package connections

import (
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

	selectedConn            int
	connScrollTop           int
	connFilterMode          bool
	connFilter              textinput.Model
	connDetailMode          bool
	connDetailSnapshot      *model.Connection
	connIPInfo              *model.IPInfo
	connDetailLeftScroll    int
	connDetailRightScroll   int
	connDetailFocusPanel    int  // 0=左侧(基础+地理), 1=右侧(JSON)
	connDetailJSONLineCount int  // JSON行数缓存，用于滚动上限约束
	connViewMode            int  // 0=流量监控, 1=活跃, 2=历史
	inlineDetailMode        bool // 新增：内联详情模式
	inlineDetailFocused     bool
	inlineDetailScroll      int

	siteTests        []model.SiteTest
	selectedSiteTest int
	proxyAddr        string
	topNModalMode    bool
	topNModalScroll  int

	doubleClickDetector common.DoubleClickDetector[MouseTarget]

	connGen   uint64
	topNCache *topNCache
}

// NewState 初始化连接状态
func NewState(proxyAddr string, siteTests []model.SiteTest) State {
	ti := textinput.New()
	ti.Placeholder = "filter..."
	ti.CharLimit = 100
	return State{
		proxyAddr:  proxyAddr,
		siteTests:  siteTests,
		connFilter: ti,
		topNCache:  &topNCache{},
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

// ViewMode 返回当前连接视图模式
func (s State) ViewMode() int { return s.connViewMode }

// FilterMode 返回是否处于过滤模式
func (s State) FilterMode() bool { return s.connFilterMode }

func (s State) triggerSiteTestByIndex(idx int, timeout int) (State, tea.Cmd) {
	if idx < 0 || idx >= len(s.siteTests) {
		return s, nil
	}
	site := s.siteTests[idx]
	s.siteTests[idx].Testing = true
	return s, TestSiteDelay(s.proxyAddr, site.Name, site.URL, timeout)
}

func (s *State) setConnViewMode(mode int) {
	if mode < ConnViewTraffic || mode > ConnViewHistory {
		mode = ConnViewTraffic
	}
	s.connViewMode = mode
	s.selectedConn = 0
	s.connScrollTop = 0
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
