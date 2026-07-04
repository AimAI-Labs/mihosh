package connections

import (
	"encoding/json"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	tea "github.com/charmbracelet/bubbletea"
)

// DetailMode 返回是否处于详情模式
func (s State) DetailMode() bool { return s.connDetailMode }

func (s State) openSelectedConnectionDetail() (State, tea.Cmd) {
	conn := s.selectedConnection()
	if conn == nil {
		return s, nil
	}

	snapshot := *conn
	s.connDetailSnapshot = &snapshot
	s.connDetailMode = true
	s.connIPInfo = nil
	s.connDetailJSONLineCount = countConnJSONLines(&snapshot)
	s.connDetailRightPanel.SetMaxScroll(s.connDetailJSONLineCount - 5)
	s.connDetailLeftPanel.SetMaxScroll(50)
	return s, FetchIPInfo(conn.Metadata.DestinationIP)
}

func (s *State) closeConnectionDetail() {
	s.connDetailMode = false
	s.connDetailSnapshot = nil
	s.connIPInfo = nil
	s.connDetailLeftPanel.Reset()
	s.connDetailRightPanel.Reset()
	s.connDetailFocusPanel = 0
	s.connDetailJSONLineCount = 0
}

// clampRightScroll 约束右侧(JSON)滚动偏移上限
func (s *State) clampRightScroll() {
	maxScroll := s.connDetailJSONLineCount - 5
	if maxScroll < 0 {
		maxScroll = 0
	}
	s.connDetailRightPanel.SetMaxScroll(maxScroll)
}

// clampLeftScroll 约束左侧滚动偏移上限（左侧内容行数有限，使用静态上限）
func (s *State) clampLeftScroll() {
	s.connDetailLeftPanel.SetMaxScroll(50)
}

// countConnJSONLines 计算连接JSON的行数
func countConnJSONLines(conn *model.Connection) int {
	if conn == nil {
		return 0
	}
	data, err := json.MarshalIndent(conn, "", "  ")
	if err != nil {
		return 0
	}
	return len(strings.Split(string(data), "\n"))
}
