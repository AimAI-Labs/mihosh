package placeholder

import (
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
)

// FrameInterval 是像素动画切换一帧的间隔。
const FrameInterval = 160 * time.Millisecond

// textMaxWidth 限制说明文字的换行宽度，避免宽终端出现长行。
const textMaxWidth = 64

// State 记录占位屏像素动画的当前帧。
type State struct{ frame int }

// Frame 返回当前动画帧序号。
func (s State) Frame() int { return s.frame }

// Advance 推进到下一帧，循环播放。
func (s State) Advance() State {
	return State{frame: (s.frame + 1) % len(sparkFrames)}
}

// sprite 是一幅“插头与插座断开、火花在缺口跳动”的像素画。
// 每两个像素行合成为一个半方块字符，因此 30x14 的网格渲染为 30x7 文本行。
var sprite = []string{
	"..............................",
	"..............................",
	"..............................",
	"..##########.......##########.",
	"..#oooooooo#.......#oooooooo#.",
	"..#oooooooo###.......ooooooo#.",
	"..#oooooooo#.......#oooooooo#.",
	"..#oooooooo#.......#oooooooo#.",
	"..#oooooooo###.......ooooooo#.",
	"..#oooooooo#.......#oooooooo#.",
	"..##########.......##########.",
	"..............................",
	"..............................",
	"..............................",
}

// sparkFrames 是叠加在插头与插座之间空隙上的火花像素，按帧循环。
var sparkFrames = [][][2]int{
	{{16, 6}, {16, 7}},
	{{16, 5}, {15, 6}, {16, 6}, {17, 6}, {16, 7}, {16, 8}},
	{{15, 5}, {16, 5}, {17, 5}, {15, 6}, {16, 6}, {17, 6}, {15, 7}, {16, 7}, {17, 7}},
	{{14, 5}, {15, 6}, {16, 6}, {16, 7}, {17, 7}, {18, 8}},
}

// Render 渲染内核未连接时的统一占位屏，返回精确 width x height 的文本块。
func Render(width, height int, frame int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	lines := bodyLines(width, height, frame)
	top := (height - len(lines)) / 2
	if top < 0 {
		top = 0
	}

	out := make([]string, 0, height)
	for i := 0; i < top; i++ {
		out = append(out, "")
	}
	out = append(out, lines...)
	for len(out) < height {
		out = append(out, "")
	}
	if len(out) > height {
		out = out[:height]
	}
	return strings.Join(out, "\n")
}

// bodyLines 按可用高度选择保留的内容块：空间不足时先去掉像素画，再去掉原因说明。
func bodyLines(width, height, frame int) []string {
	title := textBlock(i18n.T("placeholder.title"), titleStyle(), width)
	reason := textBlock(i18n.T("placeholder.reason"), reasonStyle(), width)
	hint := textBlock(i18n.T("placeholder.hint"), hintStyle(), width)

	if width >= spriteWidth() {
		sprite := centerLines(spriteLines(frame), width)
		if lines := joinBlocks(sprite, title, reason, hint); len(lines) <= height {
			return lines
		}
	}
	if lines := joinBlocks(title, reason, hint); len(lines) <= height {
		return lines
	}
	if lines := joinBlocks(title, hint); len(lines) <= height {
		return lines
	}
	return joinBlocks(title)
}

// joinBlocks 用一行空行分隔各内容块，空块被跳过。
func joinBlocks(blocks ...[]string) []string {
	var lines []string
	for _, block := range blocks {
		if len(block) == 0 {
			continue
		}
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, block...)
	}
	return lines
}

// spriteLines 将像素网格渲染为居中的半方块文本行。
func spriteLines(frame int) []string {
	grid := spriteGrid(frame)
	width := len(grid[0])
	lines := make([]string, 0, (len(grid)+1)/2)
	for row := 0; row < len(grid); row += 2 {
		var line strings.Builder
		for col := 0; col < width; col++ {
			top := grid[row][col]
			bottom := byte('.')
			if row+1 < len(grid) {
				bottom = grid[row+1][col]
			}
			line.WriteString(halfBlock(top, bottom))
		}
		lines = append(lines, line.String())
	}
	return lines
}

// spriteGrid 在基础像素画上叠加当前帧的火花。
func spriteGrid(frame int) [][]byte {
	grid := make([][]byte, len(sprite))
	for i, row := range sprite {
		grid[i] = []byte(row)
	}
	for _, point := range sparkFrames[frame%len(sparkFrames)] {
		if point[1] >= 0 && point[1] < len(grid) && point[0] >= 0 && point[0] < len(grid[point[1]]) {
			grid[point[1]][point[0]] = '*'
		}
	}
	return grid
}

// halfBlock 用一个半方块字符表示上下两个像素。
func halfBlock(top, bottom byte) string {
	topColor, topOn := pixelColor(top)
	bottomColor, bottomOn := pixelColor(bottom)

	switch {
	case topOn && bottomOn:
		return lipgloss.NewStyle().
			Foreground(theme.Resolve(topColor)).
			Background(theme.Resolve(bottomColor)).
			Render("▀")
	case topOn:
		return lipgloss.NewStyle().Foreground(theme.Resolve(topColor)).Render("▀")
	case bottomOn:
		return lipgloss.NewStyle().Foreground(theme.Resolve(bottomColor)).Render("▄")
	default:
		return " "
	}
}

// pixelColor 返回像素字符对应的主题色；返回 false 表示该像素透明。
func pixelColor(pixel byte) (lipgloss.Color, bool) {
	t := theme.Current()
	switch pixel {
	case '#':
		return t.Primary, true
	case 'o':
		return t.Dim, true
	case '*':
		return t.Warning, true
	}
	return "", false
}

// spriteWidth 返回像素画渲染后的字符宽度。
func spriteWidth() int { return len(sprite[0]) }

func titleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(theme.Resolve(theme.Current().Primary))
}

func reasonStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(theme.Resolve(theme.Current().Foreground))
}

func hintStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(theme.Resolve(theme.Current().Muted))
}

// textBlock 将文本按终端宽度居中换行，每行宽度不超过 width。
func textBlock(text string, style lipgloss.Style, width int) []string {
	if text == "" || width <= 0 {
		return nil
	}
	inner := min(width-2, textMaxWidth)
	if inner < 8 {
		inner = width
	}
	rendered := style.Width(inner).Align(lipgloss.Center).Render(text)
	lines := strings.Split(rendered, "\n")
	pad := (width - inner) / 2
	if pad <= 0 {
		return lines
	}
	prefix := strings.Repeat(" ", pad)
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return lines
}

// centerLines 为每行补充左侧空白，使其在 width 内居中。
func centerLines(lines []string, width int) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		pad := (width - lipgloss.Width(line)) / 2
		if pad > 0 {
			line = strings.Repeat(" ", pad) + line
		}
		out[i] = line
	}
	return out
}
