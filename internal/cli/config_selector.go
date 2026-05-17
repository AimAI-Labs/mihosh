package cli

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type configChoice int

const (
	choiceMihosh configChoice = iota
	choiceMihomo
	choiceCancel
)

type selectorModel struct {
	choices  []string
	selected int
	choice   configChoice
	quitting bool
}

func initialSelectorModel() selectorModel {
	return selectorModel{
		choices:  []string{"Mihosh 配置", "Mihomo 配置"},
		selected: 0,
		choice:   choiceCancel,
	}
}

func (m selectorModel) Init() tea.Cmd {
	return nil
}

func (m selectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.selected < len(m.choices)-1 {
				m.selected++
			}
		case "enter", " ":
			if m.selected == 0 {
				m.choice = choiceMihosh
			} else {
				m.choice = choiceMihomo
			}
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m selectorModel) View() string {
	if m.quitting {
		return ""
	}

	var s strings.Builder
	s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(common.CActive).Render("请选择要编辑的配置文件：") + "\n\n")

	for i, choice := range m.choices {
		checkbox := "[ ]"
		if m.selected == i {
			checkbox = lipgloss.NewStyle().Foreground(common.CActive).Render("[x]")
			s.WriteString(fmt.Sprintf("%s %s\n", checkbox, lipgloss.NewStyle().Foreground(common.CActive).Bold(true).Render(choice)))
		} else {
			s.WriteString(fmt.Sprintf("%s %s\n", checkbox, choice))
		}
	}

	s.WriteString("\n" + lipgloss.NewStyle().Foreground(common.CMuted).Render("(使用方向键选择，回车确认，Esc/q 退出)") + "\n")

	return s.String()
}
