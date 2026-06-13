package components

import (
	"encoding/json"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
)

// RenderDetailModalRight 渲染详情模态框的右侧（JSON详情）
func RenderDetailModalRight(conn *model.Connection, width, height, scrollTop int, isFocused bool) string {
	// maxHeight = height - 4 是因为：
	// 上下边框占 2 行
	// 上下滚动提示占 2 行
	// 调整高度，为边框留出空间
	maxHeight := height - 4
	if maxHeight < 5 {
		maxHeight = 5
	}

	// 获取JSON数据
	jsonLines, err := getJSONLines(conn)
	if err != nil {
		jsonLines = []string{i18n.Tf("conns.detail.parse_failed", err)}
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

	// 样式 — Tokyo Night JSON 高亮
	jsonStyle := lipgloss.NewStyle().Foreground(common.TokyoGreen)

	var contentLines []string
	for _, line := range visibleLines {
		contentLines = append(contentLines, jsonStyle.Render(line))
	}

	// 滚动提示
	var output []string
	dimStyle := common.DimStyle
	if isFocused {
		dimStyle = dimStyle.Foreground(common.TokyoCyan)
	}

	if scrollTop > 0 {
		output = append(output, dimStyle.Render(i18n.Tf("conns.detail.more_up", scrollTop)))
	} else {
		output = append(output, "") // 占位
	}

	output = append(output, contentLines...)

	// 补齐高度
	for len(output) < maxHeight+1 {
		output = append(output, "")
	}

	if endIdx < totalLines {
		output = append(output, dimStyle.Render(i18n.Tf("conns.detail.more_down", totalLines-endIdx)))
	} else {
		output = append(output, "") // 占位
	}

	body := strings.Join(output, "\n")

	borderColor := common.TokyoMuted
	titleColor := common.TokyoBlue
	if isFocused {
		borderColor = common.TokyoPurple
		titleColor = common.TokyoCyan
	}

	return common.RenderBorderedPanel(i18n.T("conns.detail.title_json"), body, width, borderColor, titleColor)
}

func getJSONLines(conn *model.Connection) ([]string, error) {
	jsonData, err := json.MarshalIndent(conn, "", "  ")
	if err != nil {
		return nil, err
	}
	return strings.Split(string(jsonData), "\n"), nil
}
