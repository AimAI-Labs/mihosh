package rules

import (
	"regexp"
	"slices"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/dialog"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
)

var customRuleType = regexp.MustCompile(`^[A-Z][A-Z0-9-]*$`)

type ruleTypeChoice struct {
	name   string
	custom bool
}

func typeChoices(query string, available []string) []ruleTypeChoice {
	query = strings.ToUpper(strings.TrimSpace(query))
	types := ruleFilterTypes(available)
	var choices []ruleTypeChoice
	for _, name := range types {
		if strings.Contains(name, query) {
			choices = append(choices, ruleTypeChoice{name: name})
		}
	}
	// The controller aliases IPv6 ranges to IP-CIDR, while both are valid in YAML.
	if strings.Contains("IP-CIDR6", query) && !slices.Contains(types, "IP-CIDR6") {
		choices = append(choices, ruleTypeChoice{name: "IP-CIDR6"})
	}
	exact := slices.IndexFunc(choices, func(choice ruleTypeChoice) bool {
		return choice.name == query
	})
	if exact >= 0 {
		match := choices[exact]
		copy(choices[1:exact+1], choices[:exact])
		choices[0] = match
	} else if customRuleType.MatchString(query) {
		choices = append(choices, ruleTypeChoice{name: query, custom: true})
	}
	return choices
}

func (f *addForm) openTypePicker() tea.Cmd {
	f.typePickerOpen, f.typePickerCursor = true, 0
	f.fields[addFieldType].SetValue("")
	return f.fields[addFieldType].Focus()
}

func (f *addForm) chooseType(name string) {
	f.typeName = name
	f.typeCursor = indexOfPreset(name)
	f.typePickerOpen = false
	if !f.isIPType() {
		f.noResolve = false
	}
	f.errMsg = ""
}

func (s State) moveTypePicker(delta int) State {
	form := &s.addForm
	choices := typeChoices(form.fields[addFieldType].Value(), s.availableTypes)
	form.typePickerCursor = max(0, min(form.typePickerCursor+delta, len(choices)-1))
	return s
}

func (s State) updateTypePicker(msg tea.KeyMsg) (State, tea.Cmd) {
	form := &s.addForm
	switch msg.String() {
	case "esc":
		form.typePickerOpen = false
	case "up":
		return s.moveTypePicker(-1), nil
	case "down":
		return s.moveTypePicker(1), nil
	case "enter":
		choices := typeChoices(form.fields[addFieldType].Value(), s.availableTypes)
		if len(choices) > 0 {
			form.chooseType(choices[min(form.typePickerCursor, len(choices)-1)].name)
		}
	default:
		input := form.fields[addFieldType]
		next, cmd := input.Update(msg)
		form.fields[addFieldType] = next
		if next.Value() != input.Value() {
			form.typePickerCursor = 0
		}
		return s, cmd
	}
	return s, nil
}

func typePickerDialog(state PageState) (dialog.Dialog, []ruleTypeChoice) {
	form := state.AddForm
	width := max(24, min(addFormModalWidth, state.Width-4))
	input := form.fields[addFieldType]
	input.Width = width - 5
	choices := typeChoices(input.Value(), state.AvailableTypes)
	count := min(len(choices), 8, max(1, state.Height-4))
	cursor := max(0, min(form.typePickerCursor, len(choices)-1))
	var start int
	var visible []ruleTypeChoice
	if count > 0 {
		start = max(0, cursor-count+1)
		visible = choices[start : start+count]
	}

	rows := []string{input.View()}
	for i, choice := range visible {
		label := choice.name
		if choice.custom {
			label = i18n.Tf("rules.type_picker_custom", choice.name)
		}
		rows = append(rows, applyFieldRowBackground("  "+label, start+i == cursor, width-4))
	}
	if len(visible) == 0 {
		rows = append(rows, common.MutedStyle().Render(i18n.T("rules.picker_empty")))
	}
	content := common.RenderTokyoPanel(i18n.T("rules.add_field_type"), strings.Join(rows, "\n"), width)
	return dialog.New(content, state.Width, state.Height).Below(rulesHeaderHeight + 1).Clipped(), visible
}
