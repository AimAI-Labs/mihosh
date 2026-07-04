package common

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// FilterList 是一个通用的过滤列表组件，负责管理光标、滚动和过滤输入框。
type FilterList struct {
	Cursor     int
	ScrollTop  int
	MaxDisplay int
	ItemCount  int

	filterMode  bool
	filterInput textinput.Model
}

// NewFilterList 初始化一个新的过滤列表
func NewFilterList() FilterList {
	ti := textinput.New()
	ti.Placeholder = "filter..."
	ti.CharLimit = 100
	return FilterList{
		filterInput: ti,
	}
}

// Update 处理列表的按键事件
func (m *FilterList) Update(msg tea.Msg) (bool, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.filterMode {
			switch {
			case key.Matches(msg, Keys.Escape):
				m.filterMode = false
				m.filterInput.Blur()
				return true, nil
			case key.Matches(msg, Keys.Enter):
				m.filterMode = false
				m.filterInput.Blur()
				m.Cursor = 0
				m.ScrollTop = 0
				return true, nil
			default:
				m.filterInput, cmd = m.filterInput.Update(msg)
				m.Cursor = 0
				m.ScrollTop = 0
				return true, cmd
			}
		}

		switch {
		case msg.String() == "/":
			m.filterMode = true
			m.filterInput.Focus()
			return true, nil
		case key.Matches(msg, Keys.Escape):
			if m.filterInput.Value() != "" {
				m.filterInput.Reset()
				m.Cursor = 0
				m.ScrollTop = 0
				return true, nil
			}
		case key.Matches(msg, Keys.Up):
			if m.Cursor > 0 {
				m.Cursor--
				m.clampScroll()
			}
			return true, nil
		case key.Matches(msg, Keys.Down):
			if m.Cursor < m.ItemCount-1 {
				m.Cursor++
				m.clampScroll()
			}
			return true, nil
		}
	}
	return false, nil
}

func (m *FilterList) clampScroll() {
	if m.Cursor < m.ScrollTop {
		m.ScrollTop = m.Cursor
	} else if m.MaxDisplay > 0 && m.Cursor >= m.ScrollTop+m.MaxDisplay {
		m.ScrollTop = m.Cursor - m.MaxDisplay + 1
	}
}

// SetItemCount 设置总数并修正光标越界
func (m *FilterList) SetItemCount(count int) {
	m.ItemCount = count
	if m.Cursor >= count && count > 0 {
		m.Cursor = count - 1
	} else if count <= 0 {
		m.Cursor = 0
	}
	m.clampScroll()
}

// SetMaxDisplay 设置可是行数并修正滚动
func (m *FilterList) SetMaxDisplay(maxDisplay int) {
	m.MaxDisplay = maxDisplay
	m.clampScroll()
}

func (m *FilterList) SetCursor(cursor int) {
	m.Cursor = cursor
	if m.Cursor >= m.ItemCount && m.ItemCount > 0 {
		m.Cursor = m.ItemCount - 1
	} else if m.ItemCount <= 0 || m.Cursor < 0 {
		m.Cursor = 0
	}
	m.clampScroll()
}

func (m *FilterList) FilterValue() string {
	return m.filterInput.Value()
}

func (m *FilterList) FilterView() string {
	return m.filterInput.View()
}

func (m FilterList) FilterMode() bool {
	return m.filterMode
}

func (m *FilterList) ResetFilter() {
	m.filterInput.Reset()
	m.Cursor = 0
	m.ScrollTop = 0
}

func (m *FilterList) HandleMouseScroll(up bool) {
	if up {
		if m.ScrollTop > 0 {
			m.ScrollTop--
		}
	} else {
		maxScroll := m.ItemCount - m.MaxDisplay
		if maxScroll < 0 {
			maxScroll = 0
		}
		if m.ScrollTop < maxScroll {
			m.ScrollTop++
		}
	}
}
