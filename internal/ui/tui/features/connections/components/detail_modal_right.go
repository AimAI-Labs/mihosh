package components

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/charmbracelet/lipgloss"
)

// RenderDetailModalRight 渲染详情模态框的右侧（JSON详情）
func RenderDetailModalRight(conn *model.Connection, width, height, scrollTop int, isFocused bool) string {
	// 调整高度，为边框和标题留出空间
	maxHeight := height - 4
	if maxHeight < 5 {
		maxHeight = 5
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(common.CSecondary).
		MarginBottom(1)

	if isFocused {
		titleStyle = titleStyle.Foreground(lipgloss.Color("#00FF00"))
	}

	title := titleStyle.Render("JSON 详情")

	// 获取JSON数据
	jsonLines, err := getJSONLines(conn)
	if err != nil {
		jsonLines = []string{fmt.Sprintf("无法解析连接信息: %v", err)}
	}

	// 为长行添加截断，防止破坏布局
	maxLineWidth := width - 4
	if maxLineWidth < 10 {
		maxLineWidth = 10
	}

	for i, line := range jsonLines {
		if lipgloss.Width(line) > maxLineWidth {
			runes := []rune(line)
			if len(runes) > maxLineWidth {
				jsonLines[i] = string(runes[:maxLineWidth-3]) + "..."
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

	// 样式
	jsonStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))

	var contentLines []string
	for _, line := range visibleLines {
		contentLines = append(contentLines, jsonStyle.Render(line))
	}

	// 滚动提示
	var output []string
	dimStyle := common.DimStyle
	if isFocused {
		dimStyle = dimStyle.Foreground(lipgloss.Color("#00FF00"))
	}

	if scrollTop > 0 {
		output = append(output, dimStyle.Render(fmt.Sprintf("↑ 还有 %d 行", scrollTop)))
	} else {
		output = append(output, "") // 占位
	}

	output = append(output, contentLines...)

	// 补齐高度
	for len(output) < maxHeight+1 {
		output = append(output, "")
	}

	if endIdx < totalLines {
		output = append(output, dimStyle.Render(fmt.Sprintf("↓ 还有 %d 行", totalLines-endIdx)))
	} else {
		output = append(output, "") // 占位
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, strings.Join(output, "\n"))
}

func getJSONLines(conn *model.Connection) ([]string, error) {
	jsonData, err := json.MarshalIndent(conn, "", "  ")
	if err != nil {
		return nil, err
	}
	return strings.Split(string(jsonData), "\n"), nil
}
