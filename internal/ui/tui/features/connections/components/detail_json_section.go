package components

import (
	"encoding/json"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
)

func renderJSONDetailSection(conn *model.Connection, s detailStyles) ([]string, error) {
	jsonBytes, err := json.MarshalIndent(conn, "", "  ")
	if err != nil {
		return nil, err
	}

	lines := []string{
		s.SectionTitle.Render("─── JSON 详情 ───"),
		"",
	}

	for _, line := range splitLines(string(jsonBytes)) {
		lines = append(lines, s.JSON.Render(line))
	}

	return lines, nil
}

func splitLines(content string) []string {
	if content == "" {
		return []string{""}
	}

	var lines []string
	start := 0
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			lines = append(lines, content[start:i])
			start = i + 1
		}
	}
	lines = append(lines, content[start:])
	return lines
}
