package nodes

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

const testResultCap = 100 // 最多保留最近 100 条测速结果记录

// TestResultEntry 单条测速结果（成功或失败）
type TestResultEntry struct {
	Name    string // 节点名
	Delay   int    // 延迟值（ms），失败时为 -1
	Error   string // 错误信息，成功时为空
	TestURL string // 测试的URL
}

type FilterEngine = common.SearchEngine

const (
	FilterEngineSubstring = common.SearchSubstring
	FilterEngineRegex     = common.SearchRegex
	FilterEngineFuzzy     = common.SearchFuzzy
)

// ProxySortOrder 节点列表排序方式
type ProxySortOrder int

const (
	SortOrderOriginal  ProxySortOrder = iota // 默认顺序
	SortOrderNameAsc                         // A-Z 升序
	SortOrderDelayAsc                        // 延迟升序
	SortOrderAvailable                       // 可用性过滤
)

var sortOrderLabels = []string{"默认顺序", "按名称排序", "按延迟排序", "仅可用节点"}

type nodesMouseFocus int

const (
	nodesMouseFocusProxy nodesMouseFocus = iota
	nodesMouseFocusGroup
)

// NewState 初始化节点状态
func NewState(client *api.Client, testURL string, timeout int) State {
	return State{
		client:  client,
		testURL: testURL,
		timeout: timeout,
	}
}

// State 节点页面完整状态
type State struct {
	client         *api.Client
	testURL        string
	timeout        int
	Mode           string // 运行模式 "Rule", "Global", "Direct", in mihomo API lowercase.
	Groups         map[string]model.Group
	Proxies        map[string]model.Proxy
	GroupNames     []string
	CurrentProxies []string
	SelectedGroup  int
	SelectedProxy  int
	Testing        bool
	TestingTarget  string
	testBatch      service.TestBatch
	batchMode      bool
	// Ring Buffer for test results (success + failure)
	TestResultsArr  []TestResultEntry
	resultHead      int // 写入位置
	resultCount     int // 已写入总数（上限 testResultCap）
	ShowTestDetail  bool
	DetailScrollTop int
	// 排序
	ProxySortOrder  ProxySortOrder
	OriginalProxies []string // 记录原始顺序，便于恢复
	// 搜索
	NodeFilter           string
	NodeFilterMode       bool
	FilterEngine         FilterEngine
	FilteredProxyIndices []int // 过滤结果的索引缓存（对应 CurrentProxies 的下标）
	// 鼠标
	MouseFocus          nodesMouseFocus
	doubleClickDetector common.DoubleClickDetector[MouseTarget]
}

// appendTestResult 向 Ring Buffer 追加一条测速结果记录
func (s *State) appendTestResult(entry TestResultEntry) {
	if len(s.TestResultsArr) < testResultCap {
		buf := make([]TestResultEntry, testResultCap)
		copy(buf, s.TestResultsArr)
		s.TestResultsArr = buf
	}
	s.TestResultsArr[s.resultHead] = entry
	s.resultHead = (s.resultHead + 1) % testResultCap
	if s.resultCount < testResultCap {
		s.resultCount++
	}
}

// TestResults 返回测速结果列表（最新在前）
func (s State) TestResults() []TestResultEntry {
	if s.resultCount == 0 || len(s.TestResultsArr) < testResultCap {
		return nil
	}
	result := make([]TestResultEntry, s.resultCount)
	for i := 0; i < s.resultCount; i++ {
		idx := (s.resultHead - 1 - i + testResultCap) % testResultCap
		result[i] = s.TestResultsArr[idx]
	}
	return result
}

// sortedTestResults 返回按当前排序模式排列的测速结果列表
func (s State) sortedTestResults() []TestResultEntry {
	result := s.TestResults()
	if len(result) == 0 {
		return result
	}
	switch s.ProxySortOrder {
	case SortOrderNameAsc:
		sort.Slice(result, func(i, j int) bool {
			return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
		})
	case SortOrderDelayAsc:
		sort.Slice(result, func(i, j int) bool {
			rank := func(e TestResultEntry) int {
				if e.Error != "" || e.Delay <= 0 {
					return 9999999
				}
				return e.Delay
			}
			return rank(result[i]) < rank(result[j])
		})
	case SortOrderAvailable:
		sort.Slice(result, func(i, j int) bool {
			return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
		})
		// SortOrderOriginal: 保持 TestResults() 返回的最新在前顺序
	}
	return result
}

// clearTestResults 清空测速结果记录
func (s *State) clearTestResults() {
	s.resultHead = 0
	s.resultCount = 0
}

// displayProxies 返回当前应显示的节点列表（搜索时返回过滤结果，否则返回全部）
func (s State) displayProxies() []string {
	if s.NodeFilter == "" {
		return s.CurrentProxies
	}
	result := make([]string, len(s.FilteredProxyIndices))
	for i, idx := range s.FilteredProxyIndices {
		result[i] = s.CurrentProxies[idx]
	}
	return result
}

// ToPageState 转换为渲染层所需的 PageState
func (s State) ToPageState(width, height int) PageState {
	return PageState{
		Mode:             s.Mode,
		Groups:           s.Groups,
		Proxies:          s.Proxies,
		GroupNames:       s.GroupNames,
		SelectedGroup:    s.SelectedGroup,
		SelectedProxy:    s.SelectedProxy,
		CurrentProxies:   s.displayProxies(),
		Testing:          s.Testing,
		TestingTarget:    s.TestingTarget,
		TestResults:      s.sortedTestResults(),
		ShowTestDetail:   s.ShowTestDetail,
		DetailScrollTop:  s.DetailScrollTop,
		SortOrderLabels:  sortOrderLabels,
		CurrentSortOrder: int(s.ProxySortOrder),
		Width:            width,
		Height:           height,
		FilterText:       s.NodeFilter,
		FilterMode:       s.NodeFilterMode,
		FilterEngine:     s.FilterEngine,
	}
}

// Update 处理节点页面按键
func (s State) Update(msg tea.Msg) (State, tea.Cmd) {

	switch msg := msg.(type) {
	case messages.CoreTargetChangedMsg:
		if msg.Reset {
			return NewState(s.client, s.testURL, s.timeout), nil
		}
		if msg.Invalidate {
			return s.ResetTesting(), nil
		}
		return s, nil
	case messages.PageMouseScrollMsg:
		return s.HandleMouseScroll(msg.Up, msg.X, msg.Y, msg.Width, msg.Height), nil
	case messages.PageMouseClickMsg:
		return s.HandleMouseLeft(msg.X, msg.Y, msg.Width, msg.Height)
	case tea.KeyMsg:
		// 搜索输入模式：拦截所有按键用于输入
		if s.NodeFilterMode {
			return s.handleNodeFilterMode(msg)
		}

		// 测速结果弹窗打开时，↑/↓ 控制弹窗滚动，f/Esc 关闭弹窗
		if s.ShowTestDetail {
			switch {
			case key.Matches(msg, common.Keys.Up):
				if s.DetailScrollTop > 0 {
					s.DetailScrollTop--
				}
			case key.Matches(msg, common.Keys.Down):
				s.DetailScrollTop++
			case key.Matches(msg, common.Keys.Home):
				s.DetailScrollTop = 0
			case key.Matches(msg, common.Keys.End):
				// 交由渲染层按可见行数钳制到末尾
				s.DetailScrollTop = 1 << 30
			case msg.String() == "f", msg.String() == "esc":
				s.ShowTestDetail = false
				s.DetailScrollTop = 0
			}
			return s, nil
		}

		display := s.displayProxies()

		switch {
		case key.Matches(msg, common.Keys.Up):
			s.MouseFocus = nodesMouseFocusProxy
			if s.SelectedProxy > 0 {
				s.SelectedProxy--
			}

		case key.Matches(msg, common.Keys.Down):
			s.MouseFocus = nodesMouseFocusProxy
			if s.SelectedProxy < len(display)-1 {
				s.SelectedProxy++
			}

		case key.Matches(msg, common.Keys.Left):
			s.MouseFocus = nodesMouseFocusGroup
			if s.SelectedGroup > 0 {
				s.SelectedGroup--
				s.updateCurrentProxies()
				s.SelectedProxy = 0
			}

		case key.Matches(msg, common.Keys.Right):
			s.MouseFocus = nodesMouseFocusGroup
			if s.SelectedGroup < len(s.GroupNames)-1 {
				s.SelectedGroup++
				s.updateCurrentProxies()
				s.SelectedProxy = 0
			}

		case key.Matches(msg, common.Keys.Enter):
			if len(s.GroupNames) > 0 && s.SelectedGroup < len(s.GroupNames) &&
				len(display) > 0 && s.SelectedProxy < len(display) {
				groupName := s.GroupNames[s.SelectedGroup]
				proxyName := display[s.SelectedProxy]
				return s, SelectProxy(s.client, groupName, proxyName)
			}

		case key.Matches(msg, common.Keys.Test):
			if len(display) > 0 && s.SelectedProxy < len(display) {
				proxyName := display[s.SelectedProxy]
				return s.StartSingleTest(proxyName)
			}

		case key.Matches(msg, common.Keys.TestAll):
			if len(s.CurrentProxies) > 0 {
				return s.startBatchTest(s.CurrentProxies)
			}

		case msg.String() == "f":
			s.ShowTestDetail = true
			s.DetailScrollTop = 0

		case msg.String() == "m":
			modes := []string{"rule", "global", "direct"}
			currentIdx := -1
			for i, v := range modes {
				if strings.ToLower(s.Mode) == v {
					currentIdx = i
					break
				}
			}
			if currentIdx == -1 {
				currentIdx = 0
			}
			nextMode := modes[(currentIdx+1)%len(modes)]
			return s, UpdateConfigMode(s.client, nextMode)

		case msg.String() == "s":
			s.ProxySortOrder = (s.ProxySortOrder + 1) % ProxySortOrder(len(sortOrderLabels))
			s.applySortOrder()
			s.updateFilteredProxies()
			s.SelectedProxy = 0

		case msg.String() == "/":
			s.NodeFilterMode = true

		case key.Matches(msg, common.Keys.Escape):
			if s.NodeFilter != "" {
				s.NodeFilter = ""
				s.SelectedProxy = 0
				s.updateFilteredProxies()
			}
		}
	}

	return s, nil
}

// handleNodeFilterMode 搜索输入模式处理
func (s State) handleNodeFilterMode(msg tea.KeyMsg) (State, tea.Cmd) {
	switch {
	case key.Matches(msg, common.Keys.Escape):
		s.NodeFilterMode = false
		s.NodeFilter = ""
		s.SelectedProxy = 0
		s.updateFilteredProxies()
	case key.Matches(msg, common.Keys.Enter):
		s.NodeFilterMode = false
		s.SelectedProxy = 0
	default:
		query := common.SearchQuery{Text: s.NodeFilter, Engine: s.FilterEngine}
		if query.Edit(msg) {
			s.NodeFilter, s.FilterEngine = query.Text, query.Engine
			s.updateFilteredProxies()
			s.SelectedProxy = 0
		}
	}
	return s, nil
}

// updateFilteredProxies 重建搜索过滤索引缓存
func (s *State) updateFilteredProxies() {
	if s.FilteredProxyIndices == nil {
		s.FilteredProxyIndices = make([]int, 0, 64)
	} else {
		s.FilteredProxyIndices = s.FilteredProxyIndices[:0]
	}
	if s.NodeFilter == "" {
		return
	}

	match := (common.SearchQuery{Text: s.NodeFilter, Engine: s.FilterEngine}).Matcher(common.MatchWholeSubstring)
	for i, name := range s.CurrentProxies {
		if match(name) {
			s.FilteredProxyIndices = append(s.FilteredProxyIndices, i)
		}
	}
}

// HandleMouseLeft 处理 nodes 页面左键单击/双击
func (s State) HandleMouseLeft(pageX, pageY, pageWidth, pageHeight int) (State, tea.Cmd) {
	if s.NodeFilterMode {
		return s, nil
	}
	if s.ShowTestDetail {
		pageState := s.ToPageState(pageWidth, pageHeight)
		modal := buildTestResultModal(pageState)
		overlayLines := strings.Split(modal, "\n")
		overlayH := len(overlayLines)
		overlayW := 0
		for _, l := range overlayLines {
			if w := common.DisplayWidth(l); w > overlayW {
				overlayW = w
			}
		}
		startRow := (pageHeight - overlayH) / 2
		if startRow < 0 {
			startRow = 0
		}
		startCol := (pageWidth - overlayW) / 2
		if startCol < 0 {
			startCol = 0
		}

		if pageY < startRow || pageY >= startRow+overlayH || pageX < startCol || pageX >= startCol+overlayW {
			s.ShowTestDetail = false
			s.DetailScrollTop = 0
		}
		return s, nil
	}

	hit := ResolveMouseHit(s.ToPageState(pageWidth, pageHeight), pageX, pageY)
	now := time.Now()

	switch hit.Target {
	case MouseTargetGroup:
		if hit.Index < 0 || hit.Index >= len(s.GroupNames) {
			return s, nil
		}
		s.MouseFocus = nodesMouseFocusGroup
		s.applyGroupSelection(hit.Index)
		s.doubleClickDetector.IsDoubleClick(MouseTargetGroup, hit.Index, now)
		return s, nil

	case MouseTargetGroupTest:
		if hit.Index < 0 || hit.Index >= len(s.GroupNames) {
			return s, nil
		}
		s.MouseFocus = nodesMouseFocusGroup
		group := s.Groups[s.GroupNames[hit.Index]]
		return s.startBatchTest(group.All)

	case MouseTargetProxy:
		display := s.displayProxies()
		if hit.Index < 0 || hit.Index >= len(display) {
			return s, nil
		}
		s.MouseFocus = nodesMouseFocusProxy
		s.SelectedProxy = hit.Index

		if !s.doubleClickDetector.IsDoubleClick(MouseTargetProxy, hit.Index, now) {
			return s, nil
		}
		if len(s.GroupNames) == 0 || s.SelectedGroup < 0 || s.SelectedGroup >= len(s.GroupNames) {
			return s, nil
		}
		groupName := s.GroupNames[s.SelectedGroup]
		proxyName := display[s.SelectedProxy]
		return s, SelectProxy(s.client, groupName, proxyName)

	case MouseTargetMode:
		modes := []string{"rule", "global", "direct"}
		if hit.Index >= 0 && hit.Index < len(modes) {
			mode := modes[hit.Index]
			return s, UpdateConfigMode(s.client, mode)
		}
		return s, nil
	}

	return s, nil
}

// ApplyConfigMode 应用模式数据
func (s State) ApplyConfigMode(mode string) State {
	s.Mode = mode
	return s
}

func (s *State) applyGroupSelection(groupIdx int) {
	if groupIdx < 0 || groupIdx >= len(s.GroupNames) {
		return
	}
	if groupIdx == s.SelectedGroup {
		return
	}

	s.SelectedGroup = groupIdx
	s.updateCurrentProxies()
	s.SelectedProxy = 0
}

// HandleMouseScroll 处理鼠标滚轮（弹窗打开时控制弹窗滚动，否则切换选中的策略组或节点，
// 卡片流与节点网格会根据选中项自动滚动到可见位置）
func (s State) HandleMouseScroll(up bool, pageX, pageY, pageWidth, pageHeight int) State {
	if s.NodeFilterMode {
		return s
	}
	if s.ShowTestDetail {
		if up {
			if s.DetailScrollTop > 0 {
				s.DetailScrollTop--
			}
		} else {
			s.DetailScrollTop++
		}
		return s
	}

	hit := ResolveMouseHit(s.ToPageState(pageWidth, pageHeight), pageX, pageY)

	// 鼠标在策略组卡片上，或鼠标不在节点格子上且当前焦点是策略组
	if hit.Target == MouseTargetGroup || (hit.Target != MouseTargetProxy && s.MouseFocus == nodesMouseFocusGroup) {
		if up {
			if s.SelectedGroup > 0 {
				s.SelectedGroup--
				s.updateCurrentProxies()
				s.SelectedProxy = 0
			}
		} else if s.SelectedGroup < len(s.GroupNames)-1 {
			s.SelectedGroup++
			s.updateCurrentProxies()
			s.SelectedProxy = 0
		}
		return s
	}

	// 否则在展开卡片的节点网格中移动选中项
	display := s.displayProxies()
	if up {
		if s.SelectedProxy > 0 {
			s.SelectedProxy--
		}
	} else if s.SelectedProxy < len(display)-1 {
		s.SelectedProxy++
	}
	return s
}

// ApplyGroups 应用策略组刷新结果（保留选中项）
func (s State) ApplyGroups(Groups map[string]model.Group, orderedNames []string) State {
	var selectedGroupName, selectedProxyName string
	if len(s.GroupNames) > 0 && s.SelectedGroup < len(s.GroupNames) {
		selectedGroupName = s.GroupNames[s.SelectedGroup]
	}
	if len(s.CurrentProxies) > 0 && s.SelectedProxy < len(s.CurrentProxies) {
		selectedProxyName = s.CurrentProxies[s.SelectedProxy]
	}

	s.Groups = Groups
	s.GroupNames = orderedNames

	if selectedGroupName != "" {
		for i, name := range s.GroupNames {
			if name == selectedGroupName {
				s.SelectedGroup = i
				break
			}
		}
	}
	s.updateCurrentProxies()
	if selectedProxyName != "" {
		for i, name := range s.CurrentProxies {
			if name == selectedProxyName {
				s.SelectedProxy = i
				break
			}
		}
	}
	return s
}

// ApplyProxies 应用节点数据
func (s State) ApplyProxies(Proxies map[string]model.Proxy) State {
	s.Proxies = Proxies
	return s
}

// CompleteTest applies one owned result and fills only the newly available slots.
func (s State) CompleteTest(msg messages.TestDoneMsg) (State, tea.Cmd) {
	if !s.testBatch.Complete(msg.BatchID, msg.TaskID) {
		return s, nil
	}
	entry := TestResultEntry{Name: msg.Name, Delay: msg.Delay, TestURL: msg.TestURL}
	if msg.Err != nil {
		entry.Error = msg.Err.Error()
	}
	s.appendTestResult(entry)
	if s.testBatch.Active() {
		return s.LaunchBatchTests()
	}
	s.testBatch.Cancel()
	s.Testing = false
	s.TestingTarget = ""
	s.batchMode = false
	return s, FetchProxies(s.client)
}

// ResetTesting invalidates queued work and any completion already in flight.
func (s State) ResetTesting() State {
	s.Testing = false
	s.TestingTarget = ""
	s.testBatch.Cancel()
	s.batchMode = false
	return s
}

func (s State) LaunchBatchTests() (State, tea.Cmd) {
	var cmds []tea.Cmd
	for _, task := range s.testBatch.Take() {
		cmds = append(cmds, TestProxy(s.client, task, s.testURL, s.timeout))
	}
	s.Testing = s.testBatch.Active()
	s.TestingTarget = s.testBatch.Current()
	if s.Testing && s.batchMode {
		done, total := s.testBatch.Progress()
		s.TestingTarget = fmt.Sprintf("%s（已完成 %d/%d）", s.TestingTarget, done, total)
	}
	return s, tea.Batch(cmds...)
}

// updateCurrentProxies 更新当前策略组的节点列表（指针接收者，修改自身）
func (s *State) updateCurrentProxies() {
	if len(s.GroupNames) > 0 && s.SelectedGroup < len(s.GroupNames) {
		groupName := s.GroupNames[s.SelectedGroup]
		if group, ok := s.Groups[groupName]; ok {
			// 保存原始顺序副本
			original := make([]string, len(group.All))
			copy(original, group.All)
			s.OriginalProxies = original
			// 应用当前排序
			s.CurrentProxies = original
			s.applySortOrder()
			s.updateFilteredProxies()
		}
	}
}

// applySortOrder 根据当前排序模式重新排列 CurrentProxies
func (s *State) applySortOrder() {
	switch s.ProxySortOrder {
	case SortOrderOriginal:
		// 恢复原始顺序
		if len(s.OriginalProxies) > 0 {
			copied := make([]string, len(s.OriginalProxies))
			copy(copied, s.OriginalProxies)
			s.CurrentProxies = copied
		}
	case SortOrderNameAsc:
		if len(s.OriginalProxies) > 0 {
			copied := make([]string, len(s.OriginalProxies))
			copy(copied, s.OriginalProxies)
			sort.Slice(copied, func(i, j int) bool {
				return strings.ToLower(copied[i]) < strings.ToLower(copied[j])
			})
			s.CurrentProxies = copied
		}
	case SortOrderDelayAsc:
		if len(s.OriginalProxies) > 0 {
			copied := make([]string, len(s.OriginalProxies))
			copy(copied, s.OriginalProxies)
			sort.Slice(copied, func(i, j int) bool {
				delayI, delayJ := s.getProxyDelayUnsafe(copied[i]), s.getProxyDelayUnsafe(copied[j])
				rank := func(d int) int {
					if d <= 0 {
						return 9999999 - d
					}
					return d
				}
				return rank(delayI) < rank(delayJ)
			})
			s.CurrentProxies = copied
		}
	case SortOrderAvailable:
		var filtered []string
		for _, name := range s.OriginalProxies {
			if s.getProxyDelayUnsafe(name) > 0 {
				filtered = append(filtered, name)
			}
		}
		sort.Slice(filtered, func(i, j int) bool {
			return s.getProxyDelayUnsafe(filtered[i]) < s.getProxyDelayUnsafe(filtered[j])
		})
		s.CurrentProxies = filtered
	}
}

// getProxyDelayUnsafe 获取节点的最新延迟值（未测速为0，超时/失败为-1或0）
func (s *State) getProxyDelayUnsafe(name string) int {
	if proxy, ok := s.Proxies[name]; ok && len(proxy.History) > 0 {
		return proxy.History[len(proxy.History)-1].Delay
	}
	return 0
}

// StartSingleTest 封装单节点测速的状态重置逻辑
func (s State) StartSingleTest(proxyName string) (State, tea.Cmd) {
	s = s.ResetTesting()
	s.testBatch = service.NewTestBatch([]string{proxyName})
	return s.LaunchBatchTests()
}

// startBatchTest 封装策略组/批量节点测速逻辑
func (s State) startBatchTest(targets []string) (State, tea.Cmd) {
	if len(targets) == 0 {
		return s, nil
	}
	s = s.ResetTesting()
	s.clearTestResults()
	s.ShowTestDetail = false
	s.testBatch = service.NewTestBatch(targets)
	s.batchMode = true
	return s.LaunchBatchTests()
}

// GetActiveProxyAndDelay 获取当前推断的活跃节点及其延时
func (s *State) GetActiveProxyAndDelay() (proxyName string, delay int, exists bool) {
	if strings.EqualFold(s.Mode, "direct") {
		return "", 0, false
	}

	if strings.EqualFold(s.Mode, "global") {
		if group, ok := s.Groups["GLOBAL"]; ok && group.Now != "" {
			return group.Now, s.getProxyDelayUnsafe(group.Now), true
		}
	} else if strings.EqualFold(s.Mode, "rule") {
		priorityGroups := []string{"PROXIES", "Proxies", "Proxy", "proxy", "🚀 节点选择", "节点选择"}
		for _, gName := range priorityGroups {
			if group, ok := s.Groups[gName]; ok && group.Now != "" {
				return group.Now, s.getProxyDelayUnsafe(group.Now), true
			}
		}
	}
	// Fallback to currently selected group in UI
	if len(s.GroupNames) > 0 && s.SelectedGroup >= 0 && s.SelectedGroup < len(s.GroupNames) {
		gName := s.GroupNames[s.SelectedGroup]
		if group, ok := s.Groups[gName]; ok && group.Now != "" {
			return group.Now, s.getProxyDelayUnsafe(group.Now), true
		}
	}
	return "", 0, false
}

// ResolveActiveGroupName 返回当前活跃代理所属的策略组名称，逻辑与 GetActiveProxyAndDelay 一致
func (s *State) ResolveActiveGroupName() string {
	if strings.EqualFold(s.Mode, "direct") {
		return ""
	}

	if strings.EqualFold(s.Mode, "global") {
		if group, ok := s.Groups["GLOBAL"]; ok && group.Now != "" {
			return "GLOBAL"
		}
	} else if strings.EqualFold(s.Mode, "rule") {
		priorityGroups := []string{"PROXIES", "Proxies", "Proxy", "proxy", "🚀 节点选择", "节点选择"}
		for _, gName := range priorityGroups {
			if group, ok := s.Groups[gName]; ok && group.Now != "" {
				return gName
			}
		}
	}
	// Fallback to currently selected group in UI
	if len(s.GroupNames) > 0 && s.SelectedGroup >= 0 && s.SelectedGroup < len(s.GroupNames) {
		gName := s.GroupNames[s.SelectedGroup]
		if group, ok := s.Groups[gName]; ok && group.Now != "" {
			return gName
		}
	}
	return ""
}

// InputCapture declares which input belongs to the current page mode.
func (s State) InputCapture() common.InputCapture {
	return common.InputCapture{Active: s.NodeFilterMode || s.ShowTestDetail, Interrupt: s.NodeFilterMode}
}
