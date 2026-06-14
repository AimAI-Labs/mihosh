package components

import (
	"encoding/json"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
)

// RenderDetailModalRight 渲染详情模态框的右侧（JSON详情）。
//
// 当内容超过可视区域时，不再使用「还有 N 行」文本提示，
// 改为在正文右侧绘制一条垂直滚动条，与项目其它列表（rules/nodes）保持一致。
func RenderDetailModalRight(conn *model.Connection, width, height, scrollTop int, isFocused bool) string {
	// maxHeight = height - 2：上下边框各占 1 行，正文本身无额外的滚动提示行
	maxHeight := height - 2
	if maxHeight < 5 {
		maxHeight = 5
	}

	// 获取JSON数据
	jsonLines, err := getJSONLines(conn)
	if err != nil {
		jsonLines = []string{i18n.Tf("conns.detail.parse_failed", err)}
	}

	// 是否需要滚动条：仅当内容溢出可视区域时显示
	needScrollbar := len(jsonLines) > maxHeight

	// 为长行添加截断，防止破坏布局。
	// 显示滚动条时，正文需预留 1 列宽度给滚动条。
	contentWidth := width - 4
	if needScrollbar {
		contentWidth--
	}
	if contentWidth < 10 {
		contentWidth = 10
	}

	for i, line := range jsonLines {
		if lipgloss.Width(line) > contentWidth {
			runes := []rune(line)
			if len(runes) > contentWidth {
				jsonLines[i] = string(runes[:contentWidth-3]) + "..."
			}
		}
	}

	totalLines := len(jsonLines)

	// 处理滚动
	if scrollTop > totalLines-maxHeight {
		scrollTop = totalLines - maxHeight
	}
	if scrollTop < 0 {
		scrollTop = 0
	}

	endIdx := scrollTop + maxHeight
	if endIdx > totalLines {
		endIdx = totalLines
	}

	visibleLines := jsonLines[scrollTop:endIdx]

	// 样式 — Tokyo Night JSON 高亮
	jsonStyle := lipgloss.NewStyle().Foreground(common.TokyoGreen)

	var contentLines []string
	for _, line := range visibleLines {
		contentLines = append(contentLines, jsonStyle.Render(line))
	}

	// 补齐高度，保证面板与左侧等高
	for len(contentLines) < maxHeight {
		contentLines = append(contentLines, "")
	}

	// 仅在内容溢出时，将滚动条横向拼接到正文右侧
	if needScrollbar {
		scrollbar := common.BuildVerticalScrollbar(maxHeight, totalLines, scrollTop, isFocused)
		fixedContent := lipgloss.NewStyle().Width(contentWidth).Render(strings.Join(contentLines, "\n"))
		body := joinLineByLine(fixedContent, scrollbar)
		return wrapDetailJSONPanel(body, width, isFocused)
	}

	body := strings.Join(contentLines, "\n")
	return wrapDetailJSONPanel(body, width, isFocused)
}

// wrapDetailJSONPanel 用带标题的圆角边框包裹 JSON 正文
func wrapDetailJSONPanel(body string, width int, isFocused bool) string {
	borderColor := common.TokyoMuted
	titleColor := common.TokyoBlue
	if isFocused {
		borderColor = common.TokyoPurple
		titleColor = common.TokyoCyan
	}
	return common.RenderBorderedPanel(i18n.T("conns.detail.title_json"), body, width, borderColor, titleColor)
}

// joinLineByLine 将两个多行字符串逐行横向拼接（左块右块行数应相同）
func joinLineByLine(left, right string) string {
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")

	n := len(leftLines)
	if len(rightLines) < n {
		n = len(rightLines)
	}

	rows := make([]string, 0, n)
	for i := 0; i < n; i++ {
		rows = append(rows, leftLines[i]+rightLines[i])
	}
	return strings.Join(rows, "\n")
}

func getJSONLines(conn *model.Connection) ([]string, error) {
	jsonData, err := json.MarshalIndent(conn, "", "  ")
	if err != nil {
		return nil, err
	}
	return strings.Split(string(jsonData), "\n"), nil
}
