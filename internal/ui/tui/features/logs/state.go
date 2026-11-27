package logs

import (
	"time"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	filteredLogIndexCap = 1000 // 过滤日志最多显示数量
)

// State 日志页面完整状态（使用 Ring Buffer 存储日志）
type State struct {
	// Ring Buffer (bounded at common.LogsCap)
	logBuf   []model.LogEntry
	logHead  int // 写入位置
	logCount int // 已写入总数（上限 LogsCap）

	cachedLogs     []model.LogEntry // 缓存的日志列表，避免每帧分配
	paused         bool
	frozenLogs     []model.LogEntry // 暂停期间冻结的可见快照（独立拷贝）
	pausedNewCount int
	droppedLogs    uint64
	coreLogCursor  uint64
	coreLogPending string

	// 过滤日志索引（预分配容量避免动态增长）
	filteredLogIndices []int
	logLevel           int // 0=debug,1=info,2=warning,3=error,4=silent
	logFilter          string
	logFilterMode      bool
	filterEngine       common.SearchEngine
	selectedLog        int
	logScrollTop       int
	logHScrollOffset   int
	maxHScrollOffset   int

	// 详情弹窗
	detailMode          bool
	detailLog           *model.LogEntry
	detailParsed        *ParsedLog
	detailResolved      *model.ResolvedIP
	detailSourcePrivate bool
	detailScroll        int

	// 鼠标双击检测
	doubleClickDetector common.DoubleClickDetector[struct{}]

	resolver *service.IPResolver
}

// NewState 初始化日志状态
func NewState(resolver *service.IPResolver) State {
	return State{
		logBuf:             make([]model.LogEntry, common.LogsCap),
		logLevel:           1, // 默认 info
		maxHScrollOffset:   4096,
		filteredLogIndices: make([]int, 0, filteredLogIndexCap),
		resolver:           resolver,
	}
}

func (s *State) ensureLogBuf() {
	if len(s.logBuf) != common.LogsCap {
		s.logBuf = make([]model.LogEntry, common.LogsCap)
	}
}

// DetailMode 返回是否处于详情模式
func (s State) DetailMode() bool { return s.detailMode }

// FilterMode 返回是否处于过滤模式
func (s State) FilterMode() bool { return s.logFilterMode }

// Paused 返回是否处于暂停实时刷新状态
func (s State) Paused() bool { return s.paused }

// TogglePause 切换日志页实时刷新暂停状态（暂停时复制快照，后台继续写入 Ring Buffer）
func (s State) TogglePause() State {
	if !s.paused {
		s.paused = true
		s.pausedNewCount = 0
		s.frozenLogs = append([]model.LogEntry(nil), s.cachedLogs...)
		return s
	}
	s.paused = false
	s.frozenLogs = nil
	s.pausedNewCount = 0
	s.updateFilteredLogs()
	if s.selectedLog >= len(s.filteredLogIndices) {
		s.selectedLog = max(0, len(s.filteredLogIndices)-1)
	}
	if s.logScrollTop > s.selectedLog {
		s.logScrollTop = s.selectedLog
	}
	return s
}

// Logs 返回日志列表（最新在前，暂停时返回冻结快照）
func (s State) Logs() []model.LogEntry {
	if s.paused {
		return s.frozenLogs
	}
	return s.cachedLogs
}

// rebuildCachedLogs 重建日志列表缓存
func (s *State) rebuildCachedLogs() {
	if s.logCount == 0 {
		s.cachedLogs = nil
		return
	}
	if cap(s.cachedLogs) < s.logCount {
		s.cachedLogs = make([]model.LogEntry, s.logCount)
	} else {
		s.cachedLogs = s.cachedLogs[:s.logCount]
	}
	for i := 0; i < s.logCount; i++ {
		idx := (s.logHead - 1 - i + common.LogsCap) % common.LogsCap
		s.cachedLogs[i] = s.logBuf[idx]
	}
}

// AppendLog 追加一条日志并更新过滤缓存
func (s State) AppendLog(logType, payload string) State {
	return s.AppendLogs([]model.LogEntry{{Type: logType, Payload: payload, Timestamp: time.Now()}}, 0)
}

// AppendLogs updates the cache and filter once for an entire batch.
func (s State) AppendLogs(entries []model.LogEntry, dropped uint64) State {
	s.droppedLogs += dropped
	if len(entries) == 0 {
		return s
	}
	s.ensureLogBuf()
	for _, entry := range entries {
		s.pushLog(entry)
	}
	s.finishLogBatch(len(entries))
	return s
}

func (s *State) pushLog(entry model.LogEntry) {
	s.logBuf[s.logHead] = entry
	s.logHead = (s.logHead + 1) % common.LogsCap
	s.logCount = min(s.logCount+1, common.LogsCap)
}

func (s *State) finishLogBatch(count int) {
	s.rebuildCachedLogs()
	if s.paused {
		s.pausedNewCount += count
	} else {
		s.updateFilteredLogs()
	}
}

func (s State) ClearLogs() State {
	s.coreLogPending = ""
	s.logHead = 0
	s.logCount = 0
	s.droppedLogs = 0
	clear(s.logBuf)
	s.frozenLogs = nil
	s.pausedNewCount = 0
	s.filteredLogIndices = s.filteredLogIndices[:0] // 保留底层数组，避免重新分配
	s.selectedLog = 0
	s.logScrollTop = 0
	s.rebuildCachedLogs()
	return s
}

// GetSysLogs 返回用于系统状态展示的最近日志（过滤掉 debug 等级，限制最多 50 条）
func (s State) GetSysLogs() []model.LogEntry {
	var infoLogs []model.LogEntry
	allLogs := s.cachedLogs
	for _, l := range allLogs {
		if l.Type != "debug" {
			infoLogs = append(infoLogs, l)
			if len(infoLogs) >= 50 {
				break
			}
		}
	}
	return infoLogs
}

// ToPageState 转换为渲染层所需的 PageState
func (s State) ToPageState(width, height int) PageState {
	return PageState{
		Logs:                s.Logs(),
		FilteredLogIndices:  s.filteredLogIndices,
		LogLevel:            s.logLevel,
		FilterText:          s.logFilter,
		FilterMode:          s.logFilterMode,
		FilterEngine:        s.filterEngine,
		SelectedLog:         s.selectedLog,
		ScrollTop:           s.logScrollTop,
		HScrollOffset:       s.logHScrollOffset,
		Width:               width,
		Height:              height,
		DetailMode:          s.detailMode,
		DetailLog:           s.detailLog,
		DetailParsed:        s.detailParsed,
		DetailResolved:      s.detailResolved,
		DetailSourcePrivate: s.detailSourcePrivate,
		DetailScroll:        s.detailScroll,
		Paused:              s.paused,
		PausedNewCount:      s.pausedNewCount,
		DroppedLogs:         s.droppedLogs,
	}
}

// Update 处理日志页面按键和消息
func (s State) Update(msg tea.Msg) (State, tea.Cmd) {
	switch msg := msg.(type) {
	case messages.CoreTargetChangedMsg:
		if msg.Reset {
			s.coreLogPending = ""
			return s.ClearLogs(), nil
		}
		return s, nil
	case messages.CoreProcessLogsMsg:
		return s.applyProcessLogs(msg), nil
	case messages.PageResizeMsg:
		return s.UpdateMaxHScrollOffset(msg.Width, msg.Height), nil
	case messages.PageMouseScrollMsg:
		return s.HandleMouseScroll(msg.Up, msg.Height), nil
	case messages.PageMouseClickMsg:
		return s.HandleMouseLeft(msg.Y, msg.X, msg.Width, msg.Height)
	case tea.KeyMsg:
		// 继续处理按键
		// 详情模式拦截所有按键
		if s.detailMode {
			return s.handleDetailMode(msg)
		}

		if s.logFilterMode {
			return s.handleLogFilterMode(msg)
		}

		switch {
		case key.Matches(msg, common.Keys.Enter):
			return s.openLogDetail()

		case key.Matches(msg, common.Keys.Up):
			if s.selectedLog > 0 {
				s.selectedLog--
				if s.selectedLog < s.logScrollTop {
					s.logScrollTop = s.selectedLog
				}
			}

		case key.Matches(msg, common.Keys.Down):
			if s.selectedLog < len(s.filteredLogIndices)-1 {
				s.selectedLog++
			}

		case key.Matches(msg, common.Keys.Left):
			if s.logHScrollOffset > 0 {
				s.logHScrollOffset -= 10
				if s.logHScrollOffset < 0 {
					s.logHScrollOffset = 0
				}
			}

		case key.Matches(msg, common.Keys.Right):
			if s.logHScrollOffset < s.maxHScrollOffset {
				s.logHScrollOffset += 10
				if s.logHScrollOffset > s.maxHScrollOffset {
					s.logHScrollOffset = s.maxHScrollOffset
				}
			}

		case key.Matches(msg, common.Keys.LogLevelDown):
			if s.logLevel > 0 {
				s.logLevel--
				s.updateFilteredLogs()
			}
			s.selectedLog = 0
			s.logScrollTop = 0

		case key.Matches(msg, common.Keys.LogLevelUp):
			if s.logLevel < 4 {
				s.logLevel++
				s.updateFilteredLogs()
			}
			s.selectedLog = 0
			s.logScrollTop = 0

		case msg.String() == "/":
			s.logFilterMode = true

		case msg.String() == "p", msg.String() == " ":
			s = s.TogglePause()

		case key.Matches(msg, common.Keys.Clear):
			s = s.ClearLogs()

		case key.Matches(msg, common.Keys.Escape):
			if s.logFilter != "" {
				s.logFilter = ""
				s.selectedLog = 0
				s.logScrollTop = 0
				s.updateFilteredLogs()
			}
		}

	}
	return s, nil
}

// handleDetailMode 详情弹窗按键处理
func (s State) handleDetailMode(msg tea.KeyMsg) (State, tea.Cmd) {
	switch {
	case key.Matches(msg, common.Keys.Escape), key.Matches(msg, common.Keys.Enter), msg.String() == "q":
		s.detailMode = false
		s.detailLog = nil
		s.detailParsed = nil
		s.detailResolved = nil
		s.detailSourcePrivate = false
		s.detailScroll = 0
	case key.Matches(msg, common.Keys.Up):
		if s.detailScroll > 0 {
			s.detailScroll--
		}
	case key.Matches(msg, common.Keys.Down):
		s.detailScroll++
	}
	return s, nil
}

// openLogDetail 打开选中日志的详情弹窗
func (s State) openLogDetail() (State, tea.Cmd) {
	entry := s.selectedLogEntry()
	if entry == nil {
		return s, nil
	}

	snapshot := *entry
	parsed := ParseLogPayload(snapshot.Payload)

	s.detailMode = true
	s.detailLog = &snapshot
	s.detailParsed = parsed
	s.detailResolved = nil
	s.detailSourcePrivate = false
	s.detailScroll = 0

	// 如果源IP是内网IP，异步查询应用来源
	if parsed.SourceIP != "" && service.IsPrivateIP(parsed.SourceIP) {
		s.detailSourcePrivate = true
		return s, ResolveLogSourceIP(s.resolver, parsed.SourceIP)
	}

	return s, nil
}

// selectedLogEntry 获取当前选中的日志条目
func (s State) selectedLogEntry() *model.LogEntry {
	if s.selectedLog < 0 || s.selectedLog >= len(s.filteredLogIndices) {
		return nil
	}
	logIdx := s.filteredLogIndices[s.selectedLog]
	allLogs := s.Logs()
	if logIdx < 0 || logIdx >= len(allLogs) {
		return nil
	}
	entry := allLogs[logIdx]
	return &entry
}

// ApplyIPResolved 应用异步 IP 解析结果
func (s State) ApplyIPResolved(ip string, resolved *model.ResolvedIP) State {
	if s.detailMode && s.detailParsed != nil && s.detailParsed.SourceIP == ip {
		s.detailResolved = resolved
	}
	return s
}

// HandleMouseLeft 处理日志页面鼠标左键（含双击检测和级别栏点击）
func (s State) HandleMouseLeft(pageY, pageX, pageWidth, pageHeight int) (State, tea.Cmd) {
	if s.detailMode {
		// 详情模式下点击关闭弹窗
		s.detailMode = false
		s.detailLog = nil
		s.detailParsed = nil
		s.detailResolved = nil
		s.detailSourcePrivate = false
		s.detailScroll = 0
		return s, nil
	}

	if s.logFilterMode {
		if pageY >= logsModeSwitchHeight {
			s.logFilterMode = false
		}
		return s, nil
	}
	// 点击顶部栏（级别/暂停栏与搜索栏互斥，占据 Y=0 到 Y=2）
	if pageY >= 0 && pageY < logsModeSwitchHeight {
		if s.logFilter != "" {
			if ClickedSearch(pageX, pageY, pageWidth, s.logLevel, s.paused, s.pausedNewCount, true) {
				s.logFilterMode = true
			}
			return s, nil
		}
		// 只检测中间行（Y=1）的按钮或空白处点击
		if pageY == 1 {
			if ClickedPause(pageX, pageWidth, s.logLevel, s.paused, s.pausedNewCount) {
				s = s.TogglePause()
				return s, nil
			}
			clickedLevel := ClickedLevel(pageX, pageWidth, s.logLevel)
			if clickedLevel >= 0 && clickedLevel != s.logLevel {
				s.logLevel = clickedLevel
				s.updateFilteredLogs()
				s.selectedLog = 0
				s.logScrollTop = 0
				return s, nil
			}
			if ClickedSearch(pageX, pageY, pageWidth, s.logLevel, s.paused, s.pausedNewCount, false) {
				s.logFilterMode = true
			}
		}
		return s, nil
	}

	offset := pageY - logsHeaderLines
	window := common.ResolveListWindow(len(s.filteredLogIndices), s.selectedLog, s.logScrollTop, logListHeight(pageHeight))
	clickedIndex := window.Start + offset
	if offset < 0 || pageY >= pageHeight || clickedIndex >= window.End {
		return s, nil
	}

	s.selectedLog = clickedIndex
	s.logScrollTop = window.Start

	// 双击检测
	now := time.Now()
	isDoubleClick := s.doubleClickDetector.IsDoubleClick(struct{}{}, clickedIndex, now)

	if isDoubleClick {
		return s.openLogDetail()
	}

	return s, nil
}

// HandleMouseScroll 鼠标滚轮处理
func (s State) HandleMouseScroll(up bool, pageHeight int) State {
	if s.detailMode {
		if up {
			if s.detailScroll > 0 {
				s.detailScroll--
			}
		} else {
			s.detailScroll++
		}
		return s
	}
	if s.logFilterMode {
		return s
	}

	count := len(s.filteredLogIndices)
	if count == 0 {
		return s
	}

	maxLines := logListHeight(pageHeight)
	window := common.ResolveListWindow(count, s.selectedLog, s.logScrollTop, maxLines)
	s.selectedLog, s.logScrollTop = window.Selected, window.Start

	// 每次滚动 3 行，更符合常见的鼠标滚轮习惯
	step := 3

	if up {
		s.logScrollTop -= step
		if s.logScrollTop < 0 {
			s.logScrollTop = 0
		}
		// 如果 selectedLog 超出了可视范围下方，也把它移上来，保持在可视范围内
		if s.selectedLog >= s.logScrollTop+maxLines {
			s.selectedLog = s.logScrollTop + maxLines - 1
		}
	} else {
		s.logScrollTop += step
		maxScrollTop := count - maxLines
		if maxScrollTop < 0 {
			maxScrollTop = 0
		}
		if s.logScrollTop > maxScrollTop {
			s.logScrollTop = maxScrollTop
		}
		// 如果 selectedLog 落在了上面不可见的区域，把它移下来，保持在可视范围内
		if s.selectedLog < s.logScrollTop {
			s.selectedLog = s.logScrollTop
		}
	}

	return s
}

// UpdateMaxHScrollOffset 根据页面尺寸更新最大水平滚动偏移
func (s State) UpdateMaxHScrollOffset(width, height int) State {
	// 允许用户向右滚动足够多的距离以查看长日志
	// 以前的逻辑限制了滚动范围为单个屏幕宽度，导致长日志无法完全查看
	s.maxHScrollOffset = 4096
	if s.logHScrollOffset > s.maxHScrollOffset {
		s.logHScrollOffset = s.maxHScrollOffset
	}
	return s
}

// handleLogFilterMode 日志过滤输入模式
func (s State) handleLogFilterMode(msg tea.KeyMsg) (State, tea.Cmd) {
	query := common.SearchQuery{Text: s.logFilter, Engine: s.filterEngine}
	switch query.HandleKey(msg) {
	case common.SearchActionCancel:
		s.logFilterMode = false
	case common.SearchActionConfirm:
		s.logFilterMode = false
		s.selectedLog = 0
		s.logScrollTop = 0
	case common.SearchActionChanged:
		s.logFilter, s.filterEngine = query.Text, query.Engine
		s.updateFilteredLogs()
	}
	return s, nil
}

// updateFilteredLogs 重建过滤索引缓存（索引对应 logs() 的下标）
func (s *State) updateFilteredLogs() {
	logList := s.Logs()
	// 重用预分配的切片，重置长度
	s.filteredLogIndices = s.filteredLogIndices[:0]

	match := (common.SearchQuery{Text: s.logFilter, Engine: s.filterEngine}).Matcher(common.MatchAllKeywords)

	for i, log := range logList {
		logLevelIndex := getLogLevelIndex(log.Type)
		if logLevelIndex < s.logLevel {
			continue
		}
		if s.logFilter != "" && !match(log.Payload) {
			continue
		}
		s.filteredLogIndices = append(s.filteredLogIndices, i)
	}
	if len(s.filteredLogIndices) == 0 {
		s.selectedLog = 0
		s.logScrollTop = 0
	} else {
		if s.selectedLog >= len(s.filteredLogIndices) {
			s.selectedLog = len(s.filteredLogIndices) - 1
		}
		if s.selectedLog < 0 {
			s.selectedLog = 0
		}
		if s.logScrollTop > s.selectedLog {
			s.logScrollTop = s.selectedLog
		}
	}
}

// getLogLevelIndex 获取日志级别索引（0=debug,1=info,2=warning,3=error,4=silent）
func getLogLevelIndex(level string) int {
	levels := []string{"debug", "info", "warning", "error", "silent"}
	for i, l := range levels {
		if l == level {
			return i
		}
	}
	return 1
}

// InputCapture declares which input belongs to the current page mode.
func (s State) InputCapture() common.InputCapture {
	return common.InputCapture{
		Active:    s.FilterMode() || s.detailMode,
		Help:      s.FilterMode(),
		Interrupt: s.FilterMode(),
	}
}
