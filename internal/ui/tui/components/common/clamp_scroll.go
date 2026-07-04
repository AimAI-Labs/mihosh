package common

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// ScrollablePanel 是一个通用的可滚动面板组件。
type ScrollablePanel struct {
	ScrollTop int
	MaxScroll int
}

// Update 处理面板的滚动按键事件 (Up/k, Down/j)
func (p *ScrollablePanel) Update(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, Keys.Up), msg.String() == "k":
			if p.ScrollTop > 0 {
				p.ScrollTop--
			}
			return true, nil
		case key.Matches(msg, Keys.Down), msg.String() == "j":
			if p.ScrollTop < p.MaxScroll {
				p.ScrollTop++
			} else if p.MaxScroll < 0 {
				// 如果外部没有正确设置 MaxScroll，可以无限制滚动（或者根据需要禁止）
				p.ScrollTop++
			}
			return true, nil
		}
	}
	return false, nil
}

// SetMaxScroll 设置最大滚动限制，并限制当前的滚动位置
func (p *ScrollablePanel) SetMaxScroll(maxScroll int) {
	if maxScroll < 0 {
		maxScroll = 0
	}
	p.MaxScroll = maxScroll
	if p.ScrollTop > p.MaxScroll {
		p.ScrollTop = p.MaxScroll
	}
}

// HandleMouseScroll 处理鼠标滚轮滚动
func (p *ScrollablePanel) HandleMouseScroll(up bool) {
	if up {
		if p.ScrollTop > 0 {
			p.ScrollTop--
		}
	} else {
		if p.ScrollTop < p.MaxScroll {
			p.ScrollTop++
		}
	}
}

// Reset 重置滚动位置
func (p *ScrollablePanel) Reset() {
	p.ScrollTop = 0
}
