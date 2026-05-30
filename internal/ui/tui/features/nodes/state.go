package nodes

import (
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

const testFailureCap = 100 // 最多保留最近 100 条测速失败记录

// ProxySortOrder 节点列表排序方式
type ProxySortOrder int

const (
	SortOrderOriginal ProxySortOrder = iota // 默认顺序
	SortOrderNameAsc                        // A-Z 升序
	SortOrderDelayAsc                       // 延迟升序
	SortOrderAvailable                      // 可用性过滤
)

var sortOrderLabels = []string{"默认顺序", "按名称排序", "按延迟排序", "仅可用节点"}

type nodesMouseFocus int

const (
	nodesMouseFocusProxy nodesMouseFocus = iota
	nodesMouseFocusGroup
	nodesDoubleClickThreshold = 350 * time.Millisecond
)

// State 节点页面完整状态
type State struct {
	Mode           string // 运行模式 "Rule", "Global", "Direct", in mihomo API lowercase.
	Groups         map[string]model.Group
	Proxies        map[string]model.Proxy
	GroupNames     []string
	CurrentProxies []string
	SelectedGroup  int
	SelectedProxy  int
	GroupScrollTop int
	ProxyScrollTop int
	TestPending    int
	Testing        bool
	TestingTarget  string
	TestAllActive  bool
	TestAllPending []string
	TestAllRunning []string
	TestAllTotal   int
	TestAllDone    int
	// Ring Buffer for test failures
	TestFailuresArr      [testFailureCap]string
	failHead          int // 写入位置
	failCount         int // 已写入总数（上限 testFailureCap）
	ShowFailureDetail bool
	FailureScrollTop  int
	// 排序
	ProxySortOrder  ProxySortOrder
	OriginalProxies []string // 记录原始顺序，便于恢复
	// 搜索
	NodeFilter           string
	NodeFilterMode       bool
	FilteredProxyIndices []int // 过滤结果的索引缓存（对应 CurrentProxies 的下标）
	// 鼠标
	MouseFocus      nodesMouseFocus
	LastMouseTarget MouseTarget
	LastMouseIndex  int
	LastMouseAt     time.Time
}

// appendTestFailure 向 Ring Buffer 追加一条测速失败记录
func (s *State) appendTestFailure(msg string) {
	s.TestFailuresArr[s.failHead] = msg
	s.failHead = (s.failHead + 1) % testFailureCap
	if s.failCount < testFailureCap {
		s.failCount++
	}
}

// TestFailures 返回测速失败列表（最新在前）
func (s State) TestFailures() []string {
	if s.failCount == 0 {
		return nil
	}
	result := make([]string, s.failCount)
	for i := 0; i < s.failCount; i++ {
		idx := (s.failHead - 1 - i + testFailureCap) % testFailureCap
		result[i] = s.TestFailuresArr[idx]
	}
	return result
}

// sortedTestFailures 返回按当前排序模式排列的测速失败列表
func (s State) sortedTestFailures() []string {
	result := s.TestFailures()
	if len(result) == 0 {
		return result
	}
	// 格式为 "节点名: 错误信息"，按冒号前的节点名排序
	nodeName := func(entry string) string {
		if i := strings.Index(entry, ": "); i >= 0 {
			return strings.ToLower(entry[:i])
		}
		return strings.ToLower(entry)
	}
	switch s.ProxySortOrder {
	case SortOrderNameAsc:
		sort.Slice(result, func(i, j int) bool {
			return nodeName(result[i]) < nodeName(result[j])
		})
	case SortOrderDelayAsc, SortOrderAvailable:
		// 测速失败记录没有延迟数据，保持按名称排序以便查找
		sort.Slice(result, func(i, j int) bool {
			return nodeName(result[i]) < nodeName(result[j])
		})
		// SortOrderOriginal: 保持 TestFailures() 返回的最新在前顺序
	}
	return result
}

// clearTestFailures 清空测速失败记录
func (s *State) clearTestFailures() {
	s.failHead = 0
	s.failCount = 0
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
		Mode:              s.Mode,
		Groups:            s.Groups,
		Proxies:           s.Proxies,
		GroupNames:        s.GroupNames,
		SelectedGroup:     s.SelectedGroup,
		SelectedProxy:     s.SelectedProxy,
		CurrentProxies:    s.displayProxies(),
		Testing:           s.Testing,
		TestingTarget:     s.TestingTarget,
		TestFailures:      s.sortedTestFailures(),
		ShowFailureDetail: s.ShowFailureDetail,
		FailureScrollTop:  s.FailureScrollTop,
		SortOrderLabels:   sortOrderLabels,
		CurrentSortOrder:  int(s.ProxySortOrder),
		Width:             width,
		Height:            height,
		GroupScrollTop:    s.GroupScrollTop,
		ProxyScrollTop:    s.ProxyScrollTop,
		FilterText:        s.NodeFilter,
		FilterMode:        s.NodeFilterMode,
	}
}

// Update 处理节点页面按键
func (s State) Update(msg tea.KeyMsg, client *api.Client, proxySvc *service.ProxyService, testURL string, timeout int) (State, tea.Cmd) {
	_ = proxySvc // 保留签名以兼容调用方（批量测速由页面状态控制并发）

	// 搜索输入模式：拦截所有按键用于输入
	if s.NodeFilterMode {
		return s.handleNodeFilterMode(msg)
	}

	// 失败详情弹窗打开时，↑/↓ 控制弹窗滚动，f/Esc 关闭弹窗
	if s.ShowFailureDetail {
		switch {
		case key.Matches(msg, common.Keys.Up):
			if s.FailureScrollTop > 0 {
				s.FailureScrollTop--
			}
		case key.Matches(msg, common.Keys.Down):
			s.FailureScrollTop++
		case key.Matches(msg, common.Keys.Home):
			s.FailureScrollTop = 0
		case key.Matches(msg, common.Keys.End):
			// 交由渲染层按可见行数钳制到末尾
			s.FailureScrollTop = 1 << 30
		case msg.String() == "f", msg.String() == "esc":
			s.ShowFailureDetail = false
			s.FailureScrollTop = 0
		}
		return s, nil
	}

	display := s.displayProxies()

	switch {
	case key.Matches(msg, common.Keys.Up):
		s.MouseFocus = nodesMouseFocusProxy
		if s.SelectedProxy > 0 {
			s.SelectedProxy--
			if s.SelectedProxy < s.ProxyScrollTop {
				s.ProxyScrollTop = s.SelectedProxy
			}
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
			s.ProxyScrollTop = 0
			if s.SelectedGroup < s.GroupScrollTop {
				s.GroupScrollTop = s.SelectedGroup
			}
		}

	case key.Matches(msg, common.Keys.Right):
		s.MouseFocus = nodesMouseFocusGroup
		if s.SelectedGroup < len(s.GroupNames)-1 {
			s.SelectedGroup++
			s.updateCurrentProxies()
			s.SelectedProxy = 0
			s.ProxyScrollTop = 0
		}

	case key.Matches(msg, common.Keys.Enter):
		if len(s.GroupNames) > 0 && s.SelectedGroup < len(s.GroupNames) &&
			len(display) > 0 && s.SelectedProxy < len(display) {
			groupName := s.GroupNames[s.SelectedGroup]
			proxyName := display[s.SelectedProxy]
			return s, SelectProxy(client, groupName, proxyName)
		}

	case key.Matches(msg, common.Keys.Test):
		if len(display) > 0 && s.SelectedProxy < len(display) {
			proxyName := display[s.SelectedProxy]
			s.Testing = true
			s.TestingTarget = proxyName
			s.TestAllActive = false
			s.TestAllPending = nil
			s.TestAllRunning = nil
			s.TestAllTotal = 0
			s.TestAllDone = 0
			return s, TestProxy(client, proxyName, testURL, timeout)
		}

	case key.Matches(msg, common.Keys.TestAll):
		if len(s.CurrentProxies) > 0 {
			s.Testing = true
			s.clearTestFailures()
			s.ShowFailureDetail = false
			s.TestAllActive = true
			s.TestAllPending = append([]string(nil), s.CurrentProxies...)
			s.TestAllRunning = nil
			s.TestAllTotal = len(s.CurrentProxies)
			s.TestAllDone = 0
			return s.LaunchBatchTests(client, testURL, timeout)
		}

	case msg.String() == "f":
		if s.failCount > 0 {
			s.ShowFailureDetail = true
			s.FailureScrollTop = 0
		}

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
		return s, UpdateConfigMode(client, nextMode)

	case msg.String() == "s":
		s.ProxySortOrder = (s.ProxySortOrder + 1) % ProxySortOrder(len(sortOrderLabels))
		s.applySortOrder()
		s.updateFilteredProxies()
		s.SelectedProxy = 0
		s.ProxyScrollTop = 0

	case msg.String() == "/":
		s.NodeFilterMode = true

	case key.Matches(msg, common.Keys.Escape):
		if s.NodeFilter != "" {
			s.NodeFilter = ""
			s.SelectedProxy = 0
			s.ProxyScrollTop = 0
			s.updateFilteredProxies()
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
		s.ProxyScrollTop = 0
		s.updateFilteredProxies()
	case key.Matches(msg, common.Keys.Enter):
		s.NodeFilterMode = false
		s.SelectedProxy = 0
		s.ProxyScrollTop = 0
	case key.Matches(msg, common.Keys.Backspace):
		if len(s.NodeFilter) > 0 {
			// 正确截断多字节字符
			runes := []rune(s.NodeFilter)
			s.NodeFilter = string(runes[:len(runes)-1])
			s.updateFilteredProxies()
			s.SelectedProxy = 0
			s.ProxyScrollTop = 0
		}
	default:
		input := msg.String()
		// 接受可打印的单字符（ASCII）或多字节字符（如中文）
		runes := []rune(input)
		if len(runes) == 1 && runes[0] >= 32 {
			s.NodeFilter += input
			s.updateFilteredProxies()
			s.SelectedProxy = 0
			s.ProxyScrollTop = 0
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
	filter := strings.ToLower(s.NodeFilter)
	for i, name := range s.CurrentProxies {
		if strings.Contains(strings.ToLower(name), filter) {
			s.FilteredProxyIndices = append(s.FilteredProxyIndices, i)
		}
	}
}

// HandleMouseLeft 处理 nodes 页面左键单击/双击
func (s State) HandleMouseLeft(pageX, pageY, pageWidth, pageHeight int, client *api.Client) (State, tea.Cmd) {
	if s.ShowFailureDetail {
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
		s.isMouseDoubleClick(MouseTargetGroup, hit.Index, now)
		return s, nil

	case MouseTargetProxy:
		display := s.displayProxies()
		if hit.Index < 0 || hit.Index >= len(display) {
			return s, nil
		}
		s.MouseFocus = nodesMouseFocusProxy
		s.SelectedProxy = hit.Index
		if s.SelectedProxy < s.ProxyScrollTop {
			s.ProxyScrollTop = s.SelectedProxy
		}

		if !s.isMouseDoubleClick(MouseTargetProxy, hit.Index, now) {
			return s, nil
		}
		if len(s.GroupNames) == 0 || s.SelectedGroup < 0 || s.SelectedGroup >= len(s.GroupNames) {
			return s, nil
		}
		groupName := s.GroupNames[s.SelectedGroup]
		proxyName := display[s.SelectedProxy]
		return s, SelectProxy(client, groupName, proxyName)

	case MouseTargetMode:
		modes := []string{"rule", "global", "direct"}
		if hit.Index >= 0 && hit.Index < len(modes) {
			mode := modes[hit.Index]
			return s, UpdateConfigMode(client, mode)
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
	s.ProxyScrollTop = 0
	if s.SelectedGroup < s.GroupScrollTop {
		s.GroupScrollTop = s.SelectedGroup
	}
}

func (s *State) isMouseDoubleClick(target MouseTarget, idx int, now time.Time) bool {
	isDoubleClick := target == s.LastMouseTarget &&
		idx == s.LastMouseIndex &&
		!s.LastMouseAt.IsZero() &&
		now.Sub(s.LastMouseAt) <= nodesDoubleClickThreshold

	s.LastMouseTarget = target
	s.LastMouseIndex = idx
	s.LastMouseAt = now

	return isDoubleClick
}

// HandleMouseScroll 处理鼠标滚轮（弹窗打开时控制弹窗滚动，否则根据鼠标位置或焦点控制列表滚动）
func (s State) HandleMouseScroll(up bool, pageX, pageY, pageWidth, pageHeight int) State {
	if s.ShowFailureDetail {
		if up {
			if s.FailureScrollTop > 0 {
				s.FailureScrollTop--
			}
		} else {
			s.FailureScrollTop++
		}
		return s
	}

	hit := ResolveMouseHit(s.ToPageState(pageWidth, pageHeight), pageX, pageY)
	groupMaxLines, proxyMaxLines := CalcNodesListMaxLines(pageHeight)

	// 如果鼠标在策略组区域，或鼠标不在任何有效区域但当前焦点是策略组
	if hit.Target == MouseTargetGroup || (hit.Target == MouseTargetNone && s.MouseFocus == nodesMouseFocusGroup) {
		if up {
			if s.SelectedGroup > 0 {
				s.SelectedGroup--
				s.updateCurrentProxies()
				s.SelectedProxy = 0
				s.ProxyScrollTop = 0
				if s.SelectedGroup < s.GroupScrollTop {
					s.GroupScrollTop = s.SelectedGroup
				}
			}
		} else {
			if s.SelectedGroup < len(s.GroupNames)-1 {
				s.SelectedGroup++
				s.updateCurrentProxies()
				s.SelectedProxy = 0
				s.ProxyScrollTop = 0
				if s.SelectedGroup >= s.GroupScrollTop+groupMaxLines {
					s.GroupScrollTop = s.SelectedGroup - groupMaxLines + 1
				}
			}
		}
		return s
	}

	// 否则处理节点列表（鼠标在节点区域，或鼠标不在有效区域且非策略组焦点，默认处理节点列表）
	display := s.displayProxies()
	displayCount := len(display)
	if up {
		if s.SelectedProxy > 0 {
			s.SelectedProxy--
			if s.SelectedProxy < s.ProxyScrollTop {
				s.ProxyScrollTop = s.SelectedProxy
			}
		}
	} else {
		if s.SelectedProxy < displayCount-1 {
			s.SelectedProxy++
			if s.SelectedProxy >= s.ProxyScrollTop+proxyMaxLines {
				s.ProxyScrollTop = s.SelectedProxy - proxyMaxLines + 1
			}
		}
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

// ApplyTestDone 单节点测速完成
func (s State) ApplyTestDone(name string, delay int, err error) State {
	if err != nil {
		s.appendTestFailure(fmt.Sprintf("%s: %s", name, err.Error()))
	}
	if s.TestAllActive {
		s.removeRunningTest(name)
		s.TestAllDone++
		if s.TestAllDone >= s.TestAllTotal {
			s.Testing = false
			s.TestingTarget = ""
			s.TestAllActive = false
			s.TestAllPending = nil
			s.TestAllRunning = nil
			s.TestAllTotal = 0
			s.TestAllDone = 0
			return s
		}
		s.Testing = true
		s.updateBatchTestingTarget()
		return s
	}
	if s.TestPending > 0 {
		s.TestPending--
		if s.TestPending == 0 {
			s.Testing = false
			s.TestingTarget = ""
		}
	} else {
		s.Testing = false
		s.TestingTarget = ""
	}
	return s
}

// ApplyTestAllDone 批量测速完成
func (s State) ApplyTestAllDone(results map[string]int) State {
	for name, delay := range results {
		if delay == -1 {
			s.appendTestFailure(fmt.Sprintf("%s: timeout or error", name))
		}
	}
	s.Testing = false
	s.TestingTarget = ""
	s.TestAllActive = false
	s.TestAllPending = nil
	s.TestAllRunning = nil
	s.TestAllTotal = 0
	s.TestAllDone = 0
	return s
}

// launchBatchTests 启动/补位批量测速任务（受并发上限控制）
func (s State) LaunchBatchTests(client *api.Client, testURL string, timeout int) (State, tea.Cmd) {
	if !s.TestAllActive || s.TestAllTotal == 0 {
		return s, nil
	}

	slots := model.TestConcurrency - len(s.TestAllRunning)
	if slots <= 0 || len(s.TestAllPending) == 0 {
		s.updateBatchTestingTarget()
		return s, nil
	}

	cmds := make([]tea.Cmd, 0, slots)
	for i := 0; i < slots && len(s.TestAllPending) > 0; i++ {
		name := s.TestAllPending[0]
		s.TestAllPending = s.TestAllPending[1:]
		s.TestAllRunning = append(s.TestAllRunning, name)
		cmds = append(cmds, TestProxy(client, name, testURL, timeout))
	}

	s.Testing = true
	s.updateBatchTestingTarget()
	return s, tea.Batch(cmds...)
}

func (s *State) updateBatchTestingTarget() {
	if !s.Testing || !s.TestAllActive {
		return
	}
	if len(s.TestAllRunning) == 0 {
		if len(s.TestAllPending) > 0 {
			s.TestingTarget = fmt.Sprintf("%s（已完成 %d/%d）", s.TestAllPending[0], s.TestAllDone, s.TestAllTotal)
		} else {
			s.TestingTarget = ""
		}
		return
	}
	s.TestingTarget = fmt.Sprintf("%s（已完成 %d/%d）", s.TestAllRunning[0], s.TestAllDone, s.TestAllTotal)
}

func (s *State) removeRunningTest(name string) {
	for i, running := range s.TestAllRunning {
		if running == name {
			last := len(s.TestAllRunning) - 1
			s.TestAllRunning[i] = s.TestAllRunning[last]
			s.TestAllRunning = s.TestAllRunning[:last]
			return
		}
	}
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
