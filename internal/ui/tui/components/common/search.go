package common

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type SearchEngine int

const (
	SearchSubstring SearchEngine = iota
	SearchRegex
	SearchFuzzy
)

type SearchMatchMode int

const (
	MatchWholeSubstring SearchMatchMode = iota
	MatchAllKeywords
)

// SearchQuery owns query editing and matching, leaving activation and cancellation to the page.
type SearchQuery struct {
	Text   string
	Engine SearchEngine
}

// Edit reports a query change, so the page can rebuild its index and selection once.
func (q *SearchQuery) Edit(msg tea.KeyMsg) bool {
	switch {
	case msg.Type == tea.KeyCtrlR:
		q.toggle(SearchRegex)
	case msg.Type == tea.KeyCtrlF:
		q.toggle(SearchFuzzy)
	case key.Matches(msg, Keys.Backspace):
		if q.Text == "" {
			return false
		}
		runes := []rune(q.Text)
		q.Text = string(runes[:len(runes)-1])
	default:
		if msg.Paste && len(msg.Runes) > 0 {
			added := false
			for _, r := range msg.Runes {
				if r >= 32 {
					q.Text += string(r)
					added = true
				}
			}
			return added
		}
		input := []rune(msg.String())
		if len(input) != 1 || input[0] < 32 {
			return false
		}
		q.Text += msg.String()
	}
	return true
}

func (q *SearchQuery) toggle(engine SearchEngine) {
	if q.Engine == engine {
		q.Engine = SearchSubstring
	} else {
		q.Engine = engine
	}
}

// Matcher compiles once for a filtering pass, including invalid-regex handling.
func (q SearchQuery) Matcher(mode SearchMatchMode) func(string) bool {
	if q.Text == "" {
		return func(string) bool { return true }
	}
	switch q.Engine {
	case SearchRegex:
		re, err := regexp.Compile("(?i)" + q.Text)
		return func(text string) bool { return err == nil && re.MatchString(text) }
	case SearchFuzzy:
		return func(text string) bool { return FuzzyMatch(q.Text, text) }
	default:
		keywords := []string{strings.ToLower(q.Text)}
		if mode == MatchAllKeywords {
			keywords = strings.Fields(keywords[0])
		}
		return func(text string) bool {
			text = strings.ToLower(text)
			for _, word := range keywords {
				if !strings.Contains(text, word) {
					return false
				}
			}
			return true
		}
	}
}
