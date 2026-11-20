package nodes

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	"github.com/charmbracelet/lipgloss"
)

const (
	cardsWideBreakpoint      = 100
	cardsUltraWideBreakpoint = 180
	cardsColumnGap           = 2
	cardsMinCardWidth        = 28
	cardsMinCellWidth        = 16
	cardsMaxGridCols         = 8
	cardsDotWidth            = 2
	cardsMaxDotRows          = 6
	cardsModeRows            = 4 // 模式切换(3) + 下方空行(1)
	cardsModeSwitchHeight    = 3
	cardsFrameRows           = 2 // 上下边框
	cardsHeaderRows          = 2 // 标题行 + 当前节点行
	cardsMinAreaHeight       = 6
)

// cardBox 描述一张策略组卡片在卡片流中的位置与内部几何。
// Y 为未滚动卡片流中的行号，渲染与鼠标命中共享同一份布局。
type cardBox struct {
	GroupIndex int
	Col        int
	X          int // 卡片左边框所在列
	Y          int
	Width      int
	Height     int
	InnerWidth int
	Expanded   bool

	DataStartY int // 圆点/节点网格首行（未滚动行号）
	DotRows    int
	DotCols    int
	DotCount   int

	GridCols        int
	GridCellW       int
	GridTotalRows   int
	GridVisibleRows int
	GridScrollRow   int
}

type cardLayout struct {
	AreaHeight  int
	AreaStartY  int
	Columns     int
	ColumnWidth []int
	Boxes       []cardBox
	TotalHeight int
	ScrollRows  int
}

// nodesBottomLines 返回页面底部固定占用的行数（搜索行 + 测速结果徽标）。
func nodesBottomLines(state PageState) int {
	if len(state.TestResults) > 0 {
		return 2
	}
	return 1
}

// cardAreaHeight 返回卡片流可用的行数。
func cardAreaHeight(state PageState) int {
	h := state.Height - cardsModeRows - nodesBottomLines(state)
	if h < cardsMinAreaHeight {
		h = cardsMinAreaHeight
	}
	return h
}

func calcCardLayout(state PageState, areaH int) cardLayout {
	width := state.Width
	if width < 20 {
		width = 20
	}
	if areaH < cardsMinAreaHeight {
		areaH = cardsMinAreaHeight
	}

	cols := 1
	switch {
	case width >= cardsUltraWideBreakpoint:
		cols = 3
	case width >= cardsWideBreakpoint:
		cols = 2
	}
	for cols > 1 && (width-cardsColumnGap*(cols-1))/cols < cardsMinCardWidth {
		cols--
	}

	colW := make([]int, cols)
	base := (width - cardsColumnGap*(cols-1)) / cols
	for i := range colW {
		colW[i] = base
	}
	colW[cols-1] = width - cardsColumnGap*(cols-1) - base*(cols-1)

	l := cardLayout{
		AreaHeight:  areaH,
		AreaStartY:  cardsModeRows,
		Columns:     cols,
		ColumnWidth: colW,
	}
	if len(state.GroupNames) == 0 {
		l.TotalHeight = 1
		return l
	}

	colX := make([]int, cols)
	for c := 1; c < cols; c++ {
		colX[c] = colX[c-1] + colW[c-1] + cardsColumnGap
	}
	colY := make([]int, cols)

	// 按数量均分到各列，保持与 GroupNames 相同的阅读顺序。
	perCol := (len(state.GroupNames) + cols - 1) / cols

	// 预估展开卡片所在列的其它卡片总高度，用于限制展开卡片的可见网格行数，
	// 尽量让同列卡片都能完整显示而不被视口截断。
	colCards := make([]int, cols)
	collapsedHeight := make([]int, cols)
	expandedCol := -1
	for gi := range state.GroupNames {
		col := gi / perCol
		if col >= cols {
			col = cols - 1
		}
		colCards[col]++
		if gi == state.SelectedGroup {
			expandedCol = col
			continue
		}
		inner := colW[col] - 4
		if inner < 12 {
			inner = 12
		}
		dotCols := maxInt(1, inner/cardsDotWidth)
		rows := (len(state.Groups[state.GroupNames[gi]].All) + dotCols - 1) / dotCols
		collapsedHeight[col] += cardsFrameRows + cardsHeaderRows + clampInt(rows, 1, cardsMaxDotRows)
	}
	maxVisibleRows := clampInt(areaH/4, 3, 10)
	if expandedCol >= 0 {
		// 展开卡片固定占用：上下边框 + 标题/当前节点 + 底部提示行
		fixed := cardsFrameRows + cardsHeaderRows + 1
		budget := areaH - collapsedHeight[expandedCol] - maxInt(0, colCards[expandedCol]-1) - fixed
		maxVisibleRows = minInt(maxVisibleRows, maxInt(3, budget/2))
	}

	for gi := range state.GroupNames {
		col := gi / perCol
		if col >= cols {
			col = cols - 1
		}
		b := cardBox{
			GroupIndex: gi,
			Col:        col,
			X:          colX[col],
			Y:          colY[col],
			Width:      colW[col],
			Expanded:   gi == state.SelectedGroup,
		}
		b.InnerWidth = b.Width - 4 // 左右边框 + 左右各 1 列内边距
		if b.InnerWidth < 12 {
			b.InnerWidth = 12
		}
		b.DataStartY = b.Y + 1 + cardsHeaderRows

		if b.Expanded {
			gridCols := clampInt(b.InnerWidth/cardsMinCellWidth, 1, cardsMaxGridCols)
			for gridCols > 1 && b.InnerWidth/gridCols < 10 {
				gridCols--
			}
			b.GridCols = gridCols
			b.GridCellW = b.InnerWidth / gridCols
			total := (len(state.CurrentProxies) + gridCols - 1) / gridCols
			if total < 1 {
				total = 1
			}
			b.GridTotalRows = total
			b.GridVisibleRows = minInt(total, maxVisibleRows)
			selRow := 0
			if len(state.CurrentProxies) > 0 {
				selRow = clampInt(state.SelectedProxy/gridCols, 0, total-1)
			}
			b.GridScrollRow = clampInt(selRow-b.GridVisibleRows/2, 0, maxInt(0, total-b.GridVisibleRows))
			// 上下边框 + 标题/当前节点 + 可见网格行(每行列名+信息) + 底部滚动提示行
			b.Height = cardsFrameRows + cardsHeaderRows + b.GridVisibleRows*2 + 1
		} else {
			b.DotCount = len(state.Groups[state.GroupNames[gi]].All)
			b.DotCols = maxInt(1, b.InnerWidth/cardsDotWidth)
			rows := (b.DotCount + b.DotCols - 1) / b.DotCols
			if rows < 1 {
				rows = 1
			}
			b.DotRows = minInt(rows, cardsMaxDotRows)
			b.Height = cardsFrameRows + cardsHeaderRows + b.DotRows
		}

		l.Boxes = append(l.Boxes, b)
		colY[col] += b.Height + 1 // 卡片之间空一行
	}

	total := 0
	for _, y := range colY {
		if y > 0 {
			y--
		}
		total = maxInt(total, y)
	}
	l.TotalHeight = maxInt(total, 1)

	maxScroll := maxInt(0, l.TotalHeight-areaH)
	scroll := 0
	for i := range l.Boxes {
		b := &l.Boxes[i]
		if !b.Expanded {
			continue
		}
		if b.Y < scroll {
			scroll = b.Y
		}
		if b.Y+b.Height > scroll+areaH {
			scroll = b.Y + b.Height - areaH
		}
		break
	}
	l.ScrollRows = clampInt(scroll, 0, maxScroll)
	return l
}

// ResolveMouseHit 根据页面坐标定位命中的模式/布局按钮、策略组或节点。
func ResolveMouseHit(state PageState, pageX, pageY int) MouseHit {
	none := MouseHit{Target: MouseTargetNone, Index: -1}

	if pageY >= 0 && pageY < cardsModeSwitchHeight && pageX >= 0 {
		return resolveModeBarMouseHit(state.Width, pageX, pageY)
	}

	if state.Layout == LayoutColumns {
		return resolveColumnsMouseHit(state, pageX, pageY)
	}

	l := calcCardLayout(state, cardAreaHeight(state))
	if pageY < l.AreaStartY || pageY >= l.AreaStartY+l.AreaHeight {
		return none
	}
	row := l.ScrollRows + pageY - l.AreaStartY

	col := -1
	x := 0
	for c := 0; c < l.Columns; c++ {
		if pageX >= x && pageX < x+l.ColumnWidth[c] {
			col = c
			break
		}
		x += l.ColumnWidth[c] + cardsColumnGap
	}
	if col < 0 {
		return none
	}

	var box *cardBox
	for i := range l.Boxes {
		b := &l.Boxes[i]
		if b.Col == col && row >= b.Y && row < b.Y+b.Height {
			box = b
			break
		}
	}
	if box == nil {
		return none
	}

	if row == box.Y+1 && box.GroupIndex >= 0 && box.GroupIndex < len(state.GroupNames) {
		group := state.Groups[state.GroupNames[box.GroupIndex]]
		rightText, _ := cardHeaderDelayText(state, group)
		hitW := maxInt(2, common.DisplayWidth(rightText))
		innerRight := box.X + 2 + box.InnerWidth
		if pageX >= innerRight-hitW && pageX < innerRight {
			return MouseHit{Target: MouseTargetGroupTest, Index: box.GroupIndex}
		}
	}

	if box.Expanded && row >= box.DataStartY &&
		row < box.DataStartY+box.GridVisibleRows*2 {
		cellRow := (row-box.DataStartY)/2 + box.GridScrollRow
		cellCol := (pageX - (box.X + 2)) / box.GridCellW
		if cellCol >= 0 && cellCol < box.GridCols {
			idx := cellRow*box.GridCols + cellCol
			if idx >= 0 && idx < len(state.CurrentProxies) {
				return MouseHit{Target: MouseTargetProxy, Index: idx}
			}
		}
		return MouseHit{Target: MouseTargetGroup, Index: box.GroupIndex}
	}
	return MouseHit{Target: MouseTargetGroup, Index: box.GroupIndex}
}

// renderCardArea 渲染自适应卡片流，返回恰好 areaH 行、每行宽度等于页面宽度的文本。
func renderCardArea(state PageState, areaH int) string {
	l := calcCardLayout(state, areaH)
	width := state.Width
	if width < 20 {
		width = 20
	}

	if len(l.Boxes) == 0 {
		lines := make([]string, areaH)
		blank := strings.Repeat(" ", width)
		for i := range lines {
			lines[i] = blank
		}
		loading := common.MutedStyle().Render(i18n.T("nodes.loading"))
		lines[0] = common.PadString(loading, width)
		return strings.Join(lines, "\n")
	}

	colLines := make([][]string, l.Columns)
	for c := 0; c < l.Columns; c++ {
		colLines[c] = make([]string, l.TotalHeight)
		blank := strings.Repeat(" ", l.ColumnWidth[c])
		for i := range colLines[c] {
			colLines[c][i] = blank
		}
	}
	for i := range l.Boxes {
		b := &l.Boxes[i]
		for j, line := range renderGroupCard(state, *b) {
			if b.Y+j < len(colLines[b.Col]) {
				colLines[b.Col][b.Y+j] = line
			}
		}
	}

	rows := make([]string, 0, areaH)
	for r := 0; r < areaH; r++ {
		idx := l.ScrollRows + r
		var parts []string
		for c := 0; c < l.Columns; c++ {
			line := strings.Repeat(" ", l.ColumnWidth[c])
			if idx >= 0 && idx < len(colLines[c]) {
				line = colLines[c][idx]
			}
			parts = append(parts, line)
			if c < l.Columns-1 {
				parts = append(parts, strings.Repeat(" ", cardsColumnGap))
			}
		}
		rows = append(rows, common.PadString(strings.Join(parts, ""), width))
	}
	return strings.Join(rows, "\n")
}

func renderGroupCard(state PageState, b cardBox) []string {
	border := common.Border()
	if b.Expanded {
		border = common.Info()
	}
	frame := lipgloss.NewStyle().Foreground(theme.Resolve(border))
	leftEdge := frame.Render("│ ")
	rightEdge := frame.Render(" │")

	name := state.GroupNames[b.GroupIndex]
	group := state.Groups[name]

	lines := make([]string, 0, b.Height)
	lines = append(lines, frame.Render(theme.Frame().TopLeft+strings.Repeat("─", b.Width-2)+theme.Frame().TopRight))
	lines = append(lines, cardContentLine(leftEdge, rightEdge, cardHeader(state, name, group, b), b.Width))
	lines = append(lines, cardContentLine(leftEdge, rightEdge, cardSubtitle(state, group), b.Width))
	if b.Expanded {
		lines = append(lines, cardGridLines(state, b, leftEdge, rightEdge)...)
		lines = append(lines, cardContentLine(leftEdge, rightEdge, cardFooter(state, b), b.Width))
	} else {
		lines = append(lines, cardDotLines(state, group, b, leftEdge, rightEdge)...)
	}
	lines = append(lines, frame.Render(theme.Frame().BottomLeft+strings.Repeat("─", b.Width-2)+theme.Frame().BottomRight))

	return lines
}

func cardContentLine(leftEdge, rightEdge, content string, width int) string {
	innerWidth := width - 4
	inner := common.PadString(common.TruncateDisplay(content, innerWidth), innerWidth)
	return leftEdge + inner + rightEdge
}

func cardHeaderDelayText(state PageState, group model.Group) (string, int) {
	d := resolveNowDelay(state, group)
	if d > 0 {
		return fmt.Sprintf("%d", d), d
	}
	return "⚡", 0
}

func cardHeader(state PageState, name string, group model.Group, b cardBox) string {
	alive := 0
	for _, node := range group.All {
		if proxyDelay(state, node) > 0 {
			alive++
		}
	}

	nameStyle := lipgloss.NewStyle().Foreground(theme.Resolve(common.TokyoForeground())).Bold(true)
	if color, ok := parseHexColor(name); ok {
		nameStyle = nameStyle.Foreground(theme.Resolve(color))
	}
	left := nameStyle.Render(name) +
		common.MutedStyle().Render(fmt.Sprintf(" : %s (%d/%d)", group.Type, alive, len(group.All)))

	rightText, d := cardHeaderDelayText(state, group)
	var right string
	if d > 0 {
		right = lipgloss.NewStyle().Foreground(theme.Resolve(utils.GetDelayColor(d))).Bold(true).Render(rightText)
	} else {
		right = lipgloss.NewStyle().Foreground(theme.Resolve(common.Warning())).Render(rightText)
	}
	return joinSides(left, right, b.InnerWidth)
}

func cardSubtitle(state PageState, group model.Group) string {
	now := group.Now
	if now == "" {
		now = i18n.T("nodes.unknown_node")
	}
	style := lipgloss.NewStyle().Foreground(theme.Resolve(common.TokyoForeground()))
	switch now {
	case "REJECT":
		style = common.ErrorStyle()
	case "DIRECT":
		style = common.MutedStyle()
	default:
		if color, ok := parseHexColor(now); ok {
			style = lipgloss.NewStyle().Foreground(theme.Resolve(color))
		} else if d := resolveNowDelay(state, group); d > 0 {
			style = lipgloss.NewStyle().Foreground(theme.Resolve(utils.GetDelayColor(d)))
		}
	}
	return common.MutedStyle().Render("◎ ") + style.Render(now)
}

func cardDotLines(state PageState, group model.Group, b cardBox, leftEdge, rightEdge string) []string {
	lines := make([]string, 0, b.DotRows)
	dotCache := make(map[lipgloss.Color]string, 8)
	for r := 0; r < b.DotRows; r++ {
		var sb strings.Builder
		for i := r * b.DotCols; i < (r+1)*b.DotCols && i < b.DotCount; i++ {
			c := cardDotColor(state, group.All[i])
			dot, ok := dotCache[c]
			if !ok {
				dot = lipgloss.NewStyle().Foreground(theme.Resolve(c)).Render("●")
				dotCache[c] = dot
			}
			sb.WriteString(dot)
			sb.WriteByte(' ')
		}
		lines = append(lines, cardContentLine(leftEdge, rightEdge, sb.String(), b.Width))
	}
	return lines
}

func cardGridLines(state PageState, b cardBox, leftEdge, rightEdge string) []string {
	lines := make([]string, 0, b.GridVisibleRows*2)
	for vr := 0; vr < b.GridVisibleRows; vr++ {
		gr := b.GridScrollRow + vr
		var nameLine, metaLine string
		for c := 0; c < b.GridCols; c++ {
			name, meta := renderGridCell(state, b, gr*b.GridCols+c)
			nameLine += name
			metaLine += meta
		}
		lines = append(lines, cardContentLine(leftEdge, rightEdge, nameLine, b.Width))
		lines = append(lines, cardContentLine(leftEdge, rightEdge, metaLine, b.Width))
	}
	return lines
}

func renderGridCell(state PageState, b cardBox, idx int) (string, string) {
	w := b.GridCellW
	contentW := maxInt(1, w-1) // 末尾留一列作为列间距
	blank := strings.Repeat(" ", w)
	if idx < 0 || idx >= len(state.CurrentProxies) {
		return blank, blank
	}

	name := state.CurrentProxies[idx]
	group := model.Group{}
	if b.GroupIndex >= 0 && b.GroupIndex < len(state.GroupNames) {
		group = state.Groups[state.GroupNames[b.GroupIndex]]
	}
	d := proxyDelay(state, name)

	delay := "⚡"
	if d > 0 {
		delay = fmt.Sprintf("%d", d)
	} else if d < 0 {
		delay = "Err"
	}
	proto := proxyProtocol(state, name)
	left := common.TruncateDisplay(proto, maxInt(1, contentW-common.DisplayWidth(delay)-1))
	gap := maxInt(0, contentW-common.DisplayWidth(left)-common.DisplayWidth(delay))
	metaPlain := common.PadString(left+strings.Repeat(" ", gap)+delay, contentW)
	nameText := common.TruncateDisplay(name, contentW)

	if idx == state.SelectedProxy {
		selected := lipgloss.NewStyle().
			Background(theme.Resolve(common.Selected())).
			Foreground(theme.Resolve(common.TokyoForeground())).
			Bold(true).
			Width(contentW)
		return common.PadString(selected.Render(nameText), w),
			common.PadString(selected.Render(metaPlain), w)
	}

	nameColor := common.TokyoForeground()
	if color, ok := parseHexColor(name); ok {
		nameColor = color
	} else if name == group.Now {
		nameColor = common.Success()
	} else if d > 0 {
		nameColor = utils.GetDelayColor(d)
	}
	delayColor := common.Muted()
	if d > 0 {
		delayColor = utils.GetDelayColor(d)
	} else if d < 0 {
		delayColor = common.Danger()
	}

	nameLine := common.PadString(lipgloss.NewStyle().Foreground(theme.Resolve(nameColor)).Render(nameText), w)
	metaLine := common.PadString(
		common.MutedStyle().Render(left+strings.Repeat(" ", gap))+
			lipgloss.NewStyle().Foreground(theme.Resolve(delayColor)).Render(delay),
		w,
	)
	return nameLine, metaLine
}

func cardFooter(state PageState, b cardBox) string {
	above := b.GridScrollRow * b.GridCols
	visibleEnd := (b.GridScrollRow + b.GridVisibleRows) * b.GridCols
	below := len(state.CurrentProxies) - visibleEnd
	if below < 0 {
		below = 0
	}
	if above == 0 && below == 0 {
		return ""
	}
	return common.DimStyle().Render(fmt.Sprintf("↑%d  ↓%d", above, below))
}

func proxyProtocol(state PageState, name string) string {
	p, ok := state.Proxies[name]
	if !ok {
		return "—"
	}
	proto := strings.ToLower(p.Type)
	if proto == "" {
		proto = "group"
	}
	if p.UDP {
		proto += "/udp"
	}
	return proto
}

// proxyDelay 返回节点最近一次延迟：>0 可用，-1 失败，0 未测速。
func proxyDelay(state PageState, name string) int {
	p, ok := state.Proxies[name]
	if !ok || len(p.History) == 0 {
		return 0
	}
	last := p.History[len(p.History)-1]
	if last.Error != "" || last.Delay < 0 {
		return -1
	}
	return last.Delay
}

// resolveNowDelay 解析当前节点延迟；当 Now 指向另一个策略组时向下追溯一层。
func resolveNowDelay(state PageState, group model.Group) int {
	d := proxyDelay(state, group.Now)
	if d == 0 {
		if p, ok := state.Proxies[group.Now]; ok && p.Now != "" {
			d = proxyDelay(state, p.Now)
		}
	}
	return d
}

func cardDotColor(state PageState, name string) lipgloss.Color {
	if color, ok := parseHexColor(name); ok {
		return color
	}
	d := proxyDelay(state, name)
	switch {
	case d < 0:
		return common.Danger()
	case d > 0:
		return utils.GetDelayColor(d)
	case name == "REJECT":
		return common.Danger()
	default:
		return common.Muted()
	}
}

func joinSides(left, right string, width int) string {
	rw := common.DisplayWidth(right)
	avail := maxInt(1, width-rw-1)
	left = common.TruncateDisplay(left, avail)
	gap := maxInt(1, width-common.DisplayWidth(left)-rw)
	return left + strings.Repeat(" ", gap) + right
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
