package nodes

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	"github.com/charmbracelet/lipgloss"
)

const (
	nodesGroupMinLines      = 3
	nodesProxyMinLines      = 5
	nodesSectionHeaderLines = 2 // PageHeaderStyle 文本 + 下边框
)

// MouseTarget 表示 nodes 页面鼠标命中的列表组件
type MouseTarget int

const (
	MouseTargetNone MouseTarget = iota
	MouseTargetGroup
	MouseTargetProxy
	MouseTargetMode
)

// MouseHit 是 nodes 页面鼠标命中结果
type MouseHit struct {
	Target MouseTarget
	Index  int
}

type nodesListWindow struct {
	ScrollTop int
	End       int
}

// ResolveMouseHit 根据 pageContent 内的 Y 坐标定位命中的策略组/节点行。
func ResolveMouseHit(state PageState, pageX, pageY int) MouseHit {
	// 模式切换菜单位于最顶部，占据 [0, 1] 行
	// 格式： 规则 │ 全局 │ 直连
	//       ──────────────────────
	if pageY >= 0 && pageY <= 1 {
		// 样式中有 Padding(0, 0, 0, 1)，所以从 X=1 开始
		// 每个选项: 1(padding) + 1(space) + 4(label) + 1(space) + 1(padding) = 8
		// 选项之间有 1 个分隔符 │
		// 所以 index 0: X[1-8], index 1: X[10-17], index 2: X[19-26]
		if pageX >= 1 && pageX < 28 {
			index := (pageX - 1) / 9
			// 简单校验是否点在分隔符上
			if (pageX-1)%9 < 8 {
				if index >= 0 && index < 3 {
					return MouseHit{
						Target: MouseTargetMode,
						Index:  index,
					}
				}
			}
		}
	}

	groupMaxLines, proxyMaxLines := CalcNodesListMaxLines(state.Height)

	// Mode switch uses 2 lines (no empty line after it)
	groupListLines := 1
	groupListStart := 2 + nodesSectionHeaderLines
	if len(state.GroupNames) > 0 {
		groupWindow := resolveListWindow(state.SelectedGroup, state.GroupScrollTop, groupMaxLines, len(state.GroupNames))
		groupRows := groupWindow.End - groupWindow.ScrollTop
		groupListLines = 1 + groupRows

		groupDataStart := groupListStart + 1 // 跳过表头
		groupDataEnd := groupDataStart + groupRows
		if pageY >= groupDataStart && pageY < groupDataEnd {
			return MouseHit{
				Target: MouseTargetGroup,
				Index:  groupWindow.ScrollTop + (pageY - groupDataStart),
			}
		}
	}

	proxyHeaderStart := groupListStart + groupListLines + 1
	proxyListStart := proxyHeaderStart + nodesSectionHeaderLines
	if len(state.CurrentProxies) > 0 {
		proxyWindow := resolveListWindow(state.SelectedProxy, state.ProxyScrollTop, proxyMaxLines, len(state.CurrentProxies))
		proxyRows := proxyWindow.End - proxyWindow.ScrollTop
		proxyDataStart := proxyListStart + 1
		proxyDataEnd := proxyDataStart + proxyRows
		if pageY >= proxyDataStart && pageY < proxyDataEnd {
			return MouseHit{
				Target: MouseTargetProxy,
				Index:  proxyWindow.ScrollTop + (pageY - proxyDataStart),
			}
		}
	}

	return MouseHit{Target: MouseTargetNone, Index: -1}
}

func CalcNodesListMaxLines(height int) (int, int) {
	availableHeight := height - nodesFixedLines
	if availableHeight < nodesMinHeight {
		availableHeight = nodesMinHeight
	}

	groupMaxLines := availableHeight / 3
	if groupMaxLines < nodesGroupMinLines {
		groupMaxLines = nodesGroupMinLines
	}

	proxyMaxLines := availableHeight - groupMaxLines
	if proxyMaxLines < nodesProxyMinLines {
		proxyMaxLines = nodesProxyMinLines
	}

	return groupMaxLines, proxyMaxLines
}

func resolveListWindow(selected, scrollTop, maxLines, total int) nodesListWindow {
	if total <= 0 {
		return nodesListWindow{}
	}

	if selected < 0 {
		selected = 0
	}
	if selected >= total {
		selected = total - 1
	}

	if scrollTop < 0 {
		scrollTop = 0
	}
	if scrollTop >= total {
		scrollTop = total - 1
	}

	if selected < scrollTop {
		scrollTop = selected
	}
	if selected >= scrollTop+maxLines {
		scrollTop = selected - maxLines + 1
	}
	if scrollTop < 0 {
		scrollTop = 0
	}

	endIdx := scrollTop + maxLines
	if endIdx > total {
		endIdx = total
	}

	return nodesListWindow{
		ScrollTop: scrollTop,
		End:       endIdx,
	}
}

// renderScrollbar 渲染垂直滚动条
func renderScrollbar(height, total, scrollTop, currentIdx int) string {
	if total <= height {
		return " "
	}

	barHeight := float64(height) * float64(height) / float64(total)
	if barHeight < 1 {
		barHeight = 1
	}

	barStart := float64(scrollTop) * float64(height) / float64(total)
	if float64(currentIdx) >= barStart && float64(currentIdx) < barStart+barHeight {
		return common.SymbolScrollbarThumb
	}
	return common.SymbolScrollbarTrack
}

func RenderGroupListComponent(state PageState, groupMaxLines int) string {
	if len(state.GroupNames) == 0 {
		return "  正在加载..."
	}

	maxNameLen := nodesDefaultNameLen
	maxTypeLen := nodesDefaultNameLen
	maxNowLen := nodesDefaultNameLen
	for _, name := range state.GroupNames {
		if w := displayWidth(name); w > maxNameLen {
			maxNameLen = w
		}
		group := state.Groups[name]
		if w := displayWidth(group.Type); w > maxTypeLen {
			maxTypeLen = w
		}
		if w := displayWidth(group.Now); w > maxNowLen {
			maxNowLen = w
		}
	}

	window := resolveListWindow(state.SelectedGroup, state.GroupScrollTop, groupMaxLines, len(state.GroupNames))
	header := fmt.Sprintf("  %s │ %s │ %s",
		padString("名称", maxNameLen),
		padString("类型", maxTypeLen),
		padString("当前节点", maxNowLen),
	)

	lines := make([]string, 0, window.End-window.ScrollTop)
	for i := window.ScrollTop; i < window.End; i++ {
		name := state.GroupNames[i]
		group := state.Groups[name]

		prefix := common.SymbolSelectInactive
		if i == state.SelectedGroup {
			prefix = common.SymbolSelectActive
		}

		content := fmt.Sprintf("%s%s │ %s │ %s",
			prefix,
			padString(name, maxNameLen),
			padString(group.Type, maxTypeLen),
			padString(group.Now, maxNowLen),
		)

		if i == state.SelectedGroup {
			content = common.SelectedStyle.Render(content)
		} else if group.Now != "" {
			content = common.InactiveStyle.Render(content)
		}

		bar := renderScrollbar(groupMaxLines, len(state.GroupNames), window.ScrollTop, i-window.ScrollTop)
		lines = append(lines, content+" "+common.DimStyle.Render(bar))
	}

	return common.TableHeaderStyle.Render(header) + "\n" + strings.Join(lines, "\n")
}

func RenderProxyListComponent(state PageState, proxyMaxLines int) string {
	if len(state.CurrentProxies) == 0 {
		if state.FilterText != "" {
			return "  无搜索结果"
		}
		return "  无可用节点"
	}

	var currentNode string
	if len(state.GroupNames) > 0 && state.SelectedGroup < len(state.GroupNames) {
		groupName := state.GroupNames[state.SelectedGroup]
		if group, ok := state.Groups[groupName]; ok {
			currentNode = group.Now
		}
	}

	maxNameLen := nodesDefaultNameLen
	for _, name := range state.CurrentProxies {
		if w := displayWidth(name); w > maxNameLen {
			maxNameLen = w
		}
	}

	delayColWidth := 6
	statusColWidth := 2
	window := resolveListWindow(state.SelectedProxy, state.ProxyScrollTop, proxyMaxLines, len(state.CurrentProxies))

	header := fmt.Sprintf("  %s │ %s │ %s",
		padString("名称", maxNameLen),
		padString("延迟", delayColWidth),
		padString("状态", statusColWidth),
	)

	lines := make([]string, 0, window.End-window.ScrollTop)
	for i := window.ScrollTop; i < window.End; i++ {
		name := state.CurrentProxies[i]
		proxy, exists := state.Proxies[name]

		prefix := common.SymbolSelectInactive
		if i == state.SelectedProxy {
			prefix = common.SelectedStyle.Render(common.SymbolSelectActive)
		}

		namePart := padString(name, maxNameLen)
		if i == state.SelectedProxy {
			namePart = common.SelectedStyle.Render(namePart)
		} else if name == currentNode {
			namePart = common.InactiveStyle.Render(namePart)
		}

		delayStr := "      "
		if exists && len(proxy.History) > 0 {
			lastEntry := proxy.History[len(proxy.History)-1]
			lastDelay := lastEntry.Delay
			if lastEntry.Error != "" || lastDelay < 0 {
				delayStr = " Error"
				if i != state.SelectedProxy && name != currentNode {
					delayStr = common.ErrorStyle.Render(delayStr)
				} else if i == state.SelectedProxy {
					delayStr = common.SelectedStyle.Render(delayStr)
				} else if name == currentNode {
					delayStr = common.InactiveStyle.Render(delayStr)
				}
			} else if lastDelay >= 0 {
				delayStr = fmt.Sprintf("%4dms", lastDelay)
				if i != state.SelectedProxy && name != currentNode {
					delayColor := utils.GetDelayColor(lastDelay)
					delayStr = lipgloss.NewStyle().Foreground(delayColor).Render(delayStr)
				} else if i == state.SelectedProxy {
					delayStr = common.SelectedStyle.Render(delayStr)
				} else if name == currentNode {
					delayStr = common.InactiveStyle.Render(delayStr)
				}
			}
		}

		status := common.SymbolSelectInactive
		if name == currentNode {
			status = common.SymbolCheck
		}
		if i == state.SelectedProxy {
			status = common.SelectedStyle.Render(status)
		} else if name == currentNode {
			status = common.InactiveStyle.Render(status)
		}

		line := prefix + namePart + " │ " + delayStr + " │ " + status
		bar := renderScrollbar(proxyMaxLines, len(state.CurrentProxies), window.ScrollTop, i-window.ScrollTop)
		lines = append(lines, line+" "+common.DimStyle.Render(bar))
	}

	return common.TableHeaderStyle.Render(header) + "\n" + strings.Join(lines, "\n")
}

// RenderModeSwitchComponent 渲染模式切换按钮
func RenderModeSwitchComponent(currentMode string) string {
	modes := []struct {
		Label string
		Value string
	}{
		{"规则", "rule"},
		{"全局", "global"},
		{"直连", "direct"},
	}

	activeStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#007BFF")). // Blue background
		Foreground(lipgloss.Color("#FFFFFF")).
		Padding(0, 1)
	inactiveStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#007BFF")). // Blue text
		Padding(0, 1)
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#007BFF"))

	var parts []string
	for i, m := range modes {
		text := m.Label
		if strings.ToLower(currentMode) == m.Value {
			parts = append(parts, activeStyle.Render(" "+text+" "))
		} else {
			parts = append(parts, inactiveStyle.Render(" "+text+" "))
		}
		if i < len(modes)-1 {
			parts = append(parts, borderStyle.Render("│"))
		}
	}

	content := lipgloss.JoinHorizontal(lipgloss.Left, parts...)

	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false). // Only bottom border
		BorderForeground(lipgloss.Color("#007BFF")).
		Padding(0, 0, 0, 1).
		Render(content)
}
