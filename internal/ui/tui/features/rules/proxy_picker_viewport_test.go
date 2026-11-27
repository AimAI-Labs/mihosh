package rules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestProxyPickerKeepsSelectionVisibleInShortDialog(t *testing.T) {
	const width, height = 74, 16
	s := State{showAddForm: true}
	s.addForm = newAddForm("")
	s.addForm.showProxyPicker = true
	for i := 0; i < 25; i++ {
		s.addForm.proxyGroups = append(s.addForm.proxyGroups, fmt.Sprintf("policy-%02d", i))
	}
	for i := 0; i < 12; i++ {
		s.addForm.movePickerCursor(1)
	}
	page := s.ToPageState(width, height)
	layout := computeProxyPickerLayout(page, width, height)
	require.GreaterOrEqual(t, s.addForm.pickerCursor, layout.listHeight)
	want := s.addForm.pickerFiltered()[s.addForm.pickerCursor]
	text := strings.Split(ansi.Strip(buildProxyPickerModal(page, width, height)), "\n")
	require.Contains(t, strings.Join(text[layout.listStartY:layout.listStartY+layout.listHeight], "\n"), want)
	left, top, _, _ := ResolveProxyPickerBounds(page, width, height)
	name := s.addForm.pickerFiltered()[layout.visibleStart]
	x, y := left+2, top+layout.listStartY
	s, _ = s.HandleMouseLeft(x, y, width, height)
	s, _ = s.HandleMouseLeft(x, y, width, height)
	require.False(t, s.addForm.showProxyPicker)
	require.Equal(t, name, s.addForm.proxySelected)
}
