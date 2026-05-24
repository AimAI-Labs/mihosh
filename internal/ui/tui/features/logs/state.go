package logs

import (
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	filteredLogIndexCap = 1000 // 过滤日志最多显示数量
)

// State 日志页面完整状态（使用 Ring Buffer 存储日志）
type State struct {
	// Ring Buffer
	logBuf   [common.LogsCap]model.LogEntry
	logHead  int // 写入位置
	logCount int // 已写入总数（上限 LogsCap）

	// 过滤日志索引（预分配容量避免动态增长）
	filteredLogIndices []int
	logLevel           int // 0=debug,1=info,2=warning,3=error,4=silent
	logFilter          string
	logFilterMode      bool
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
	lastClickIndex int
	lastClickAt    time.Time
}

// NewState 初始化日志状态
func NewState() State {
	return State{
		logLevel:           1, // 默认 info
		filteredLogIndices: make([]int, 0, filteredLogIndexCap),
	}
}

// logs 返回日志列表（最新在前，用于渲染）
func (s State) logs() []model.LogEntry {
	if s.logCount == 0 {
		return nil
	}
	result := make([]model.LogEntry, s.logCount)
	for i := 0; i < s.logCount; i++ {
		idx := (s.logHead - 1 - i + common.LogsCap) % common.LogsCap
		result[i] = s.logBuf[idx]
	}
	return result
}

// AppendLog 追加一条日志并更新过滤缓存
func (s State) AppendLog(logType, payload string) State {
	entry := model.LogEntry{
		Type:      logType,
		Payload:   payload,
		Timestamp: time.Now(),
	}
	s.logBuf[s.logHead] = entry
	s.logHead = (s.logHead + 1) % common.LogsCap
	if s.logCount < common.LogsCap {
		s.logCount++
	}
	s.updateFilteredLogs()
	return s
}

// ClearLogs 清空所有日志
func (s State) ClearLogs() State {
	s.logHead = 0
	s.logCount = 0
	s.filteredLogIndices = s.filteredLogIndices[:0] // 保留底层数组，避免重新分配
	s.selectedLog = 0
	s.logScrollTop = 0
	return s
}

// ToPageState 转换为渲染层所需的 PageState
func (s State) ToPageState(width, height int) PageState {
	return PageState{
		Logs:                s.logs(),
		FilteredLogIndices:  s.filteredLogIndices,
		LogLevel:            s.logLevel,
		FilterText:          s.logFilter,
		FilterMode:          s.logFilterMode,
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
	}
}

// Update 处理日志页面按键
func (s State) Update(msg tea.KeyMsg, resolver *service.IPResolver) (State, tea.Cmd) {
	// 详情模式拦截所有按键
	if s.detailMode {
		return s.handleDetailMode(msg)
	}

	if s.logFilterMode {
		return s.handleLogFilterMode(msg)
	}

	switch {
	case key.Matches(msg, common.Keys.Enter):
		return s.openLogDetail(resolver)

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
func (s State) openLogDetail(resolver *service.IPResolver) (State, tea.Cmd) {
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
		return s, ResolveLogSourceIP(resolver, parsed.SourceIP)
	}

	return s, nil
}

// selectedLogEntry 获取当前选中的日志条目
func (s State) selectedLogEntry() *model.LogEntry {
	if s.selectedLog < 0 || s.selectedLog >= len(s.filteredLogIndices) {
		return nil
	}
	logIdx := s.filteredLogIndices[s.selectedLog]
	allLogs := s.logs()
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
func (s State) HandleMouseLeft(pageY int, pageX int, pageWidth int, resolver *service.IPResolver) (State, tea.Cmd) {
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

	// 点击级别栏（pageY == 0）切换日志级别
	if pageY == 0 {
		clickedLevel := ClickedLevel(pageX, pageWidth, s.logLevel)
		if clickedLevel >= 0 && clickedLevel != s.logLevel {
			s.logLevel = clickedLevel
			s.updateFilteredLogs()
			s.selectedLog = 0
			s.logScrollTop = 0
		}
		return s, nil
	}

	const headerLines = 6 // levelBar + space + search + space + stats + space
	clickedIndex := (pageY - headerLines) + s.logScrollTop
	if clickedIndex < 0 || clickedIndex >= len(s.filteredLogIndices) {
		return s, nil
	}

	s.selectedLog = clickedIndex

	// 双击检测
	now := time.Now()
	const doubleClickThreshold = 350 * time.Millisecond
	isDoubleClick := clickedIndex == s.lastClickIndex &&
		!s.lastClickAt.IsZero() &&
		now.Sub(s.lastClickAt) <= doubleClickThreshold

	s.lastClickIndex = clickedIndex
	s.lastClickAt = now

	if isDoubleClick {
		return s.openLogDetail(resolver)
	}

	return s, nil
}

// HandleMouseScroll 鼠标滚轮处理
func (s State) HandleMouseScroll(up bool) State {
	count := len(s.filteredLogIndices)
	if up {
		if s.selectedLog > 0 {
			s.selectedLog--
			if s.selectedLog < s.logScrollTop {
				s.logScrollTop = s.selectedLog
			}
		}
	} else {
		if s.selectedLog < count-1 {
			s.selectedLog++
		}
	}
	return s
}

// UpdateMaxHScrollOffset 根据页面尺寸更新最大水平滚动偏移
func (s State) UpdateMaxHScrollOffset(width, height int) State {
	pageWidth := width - 2
	if pageWidth < 1 {
		pageWidth = 1
	}
	fixedOverhead := 8 + 1 + 20
	maxOffset := pageWidth - fixedOverhead - 20
	if maxOffset < 0 {
		maxOffset = 0
	}
	s.maxHScrollOffset = maxOffset
	if s.logHScrollOffset > s.maxHScrollOffset {
		s.logHScrollOffset = s.maxHScrollOffset
	}
	return s
}

// handleLogFilterMode 日志过滤输入模式
func (s State) handleLogFilterMode(msg tea.KeyMsg) (State, tea.Cmd) {
	switch {
	case key.Matches(msg, common.Keys.Escape):
		s.logFilterMode = false
	case key.Matches(msg, common.Keys.Enter):
		s.logFilterMode = false
		s.selectedLog = 0
		s.logScrollTop = 0
	case key.Matches(msg, common.Keys.Backspace):
		if len(s.logFilter) > 0 {
			s.logFilter = s.logFilter[:len(s.logFilter)-1]
			s.updateFilteredLogs()
		}
	default:
		input := msg.String()
		if len(input) == 1 && input[0] >= 32 && input[0] < 127 {
			s.logFilter += input
			s.updateFilteredLogs()
		}
	}
	return s, nil
}

// updateFilteredLogs 重建过滤索引缓存（索引对应 logs() 的下标）
func (s *State) updateFilteredLogs() {
	logList := s.logs()
	// 重用预分配的切片，重置长度
	s.filteredLogIndices = s.filteredLogIndices[:0]

	for i, log := range logList {
		logLevelIndex := getLogLevelIndex(log.Type)
		if logLevelIndex < s.logLevel {
			continue
		}
		if s.logFilter != "" && !strings.Contains(strings.ToLower(log.Payload), strings.ToLower(s.logFilter)) {
			continue
		}
		s.filteredLogIndices = append(s.filteredLogIndices, i)
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
