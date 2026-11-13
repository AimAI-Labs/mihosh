// Package dialog provides placement and composition for page-owned dialogs.
package dialog

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Dialog places rendered content within a viewport. It owns no interaction state.
type Dialog struct {
	content       string
	width, height int
	top           int
	clip          bool
}

// New creates a horizontally and vertically centered dialog.
func New(content string, width, height int) Dialog {
	return Dialog{content: content, width: width, height: height, top: -1}
}

// Below places a dialog at top, moving it up if necessary to fit the viewport.
func (d Dialog) Below(top int) Dialog {
	d.top = max(0, top)
	return d
}

// Clipped limits the dialog to the viewport, for scrollable forms and help.
func (d Dialog) Clipped() Dialog {
	d.clip = true
	return d
}

// Bounds returns page coordinates with exclusive right and bottom edges.
func (d Dialog) Bounds() (left, top, right, bottom int) {
	if d.width <= 0 || d.height <= 0 {
		return 0, 0, 0, 0
	}
	width, height := lipgloss.Width(d.content), lipgloss.Height(d.content)
	if d.clip {
		width, height = min(width, d.width), min(height, d.height)
	}
	left = max(0, (d.width-width)/2)
	top = max(0, (d.height-height)/2)
	if d.top >= 0 {
		top = max(0, min(d.top, d.height-height))
	}
	return left, top, left + width, top + height
}

// View places a dialog on an empty viewport, including full-page detail views.
func (d Dialog) View() string {
	if d.width <= 0 || d.height <= 0 {
		return ""
	}
	if d.clip {
		lines := strings.Split(d.content, "\n")
		lines = lines[:min(len(lines), d.height)]
		for i := range lines {
			lines[i] = ansi.Truncate(lines[i], d.width, "")
		}
		d.content = strings.Join(lines, "\n")
	}
	if d.top < 0 {
		return lipgloss.Place(d.width, d.height, lipgloss.Center, lipgloss.Center, d.content)
	}
	_, top, _, _ := d.Bounds()
	return lipgloss.Place(d.width, d.height, lipgloss.Center, lipgloss.Top, strings.Repeat("\n", top)+d.content)
}

// Overlay inserts a dialog over base, optionally dimming the background.
func (d Dialog) Overlay(base string, dim bool) string {
	if d.width <= 0 || d.height <= 0 {
		return ""
	}
	lines := strings.Split(base, "\n")
	for len(lines) < d.height {
		lines = append(lines, "")
	}
	lines = lines[:d.height]
	if dim {
		faint := lipgloss.NewStyle().Faint(true)
		for i := range lines {
			lines[i] = faint.Render(lines[i])
		}
	}
	left, top, right, _ := d.Bounds()
	for i, line := range strings.Split(d.content, "\n") {
		y := top + i
		if y >= d.height {
			break
		}
		prefix := ansi.Cut(lines[y], 0, left)
		prefix += strings.Repeat(" ", max(0, left-lipgloss.Width(prefix)))
		suffix := ansi.Cut(lines[y], right, d.width)
		if d.clip {
			line = ansi.Truncate(line, right-left, "")
		}
		lines[y] = prefix + line + suffix
		if d.clip {
			lines[y] = ansi.Truncate(lines[y], d.width, "")
		}
	}
	return strings.Join(lines, "\n")
}
