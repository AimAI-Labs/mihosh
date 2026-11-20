package profile

import (
	"bytes"
	"strconv"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// readableYAMLUnicode restores printable Unicode escaped by the YAML encoder.
// Only double-quoted YAML scalars interpret these escapes.
func readableYAMLUnicode(data []byte) []byte {
	if !bytes.Contains(data, []byte(`\U`)) && !bytes.Contains(data, []byte(`\u`)) && !bytes.Contains(data, []byte(`\x`)) {
		return data
	}
	var doc yaml.Node
	if yaml.Unmarshal(data, &doc) != nil {
		return data
	}
	lineOffsets := []int{0}
	for i, b := range data {
		if b == '\n' {
			lineOffsets = append(lineOffsets, i+1)
		}
	}
	var out bytes.Buffer
	copied := 0
	var visit func(*yaml.Node)
	visit = func(node *yaml.Node) {
		if node.Kind == yaml.ScalarNode && node.Style&yaml.DoubleQuotedStyle != 0 && node.Line > 0 && node.Line <= len(lineOffsets) {
			pos := lineOffsets[node.Line-1]
			for col := 1; col < node.Column && pos < len(data); col++ {
				_, width := utf8.DecodeRune(data[pos:])
				pos += width
			}
			// A node position can include an anchor or explicit tag before the quote.
			for pos < len(data) && (data[pos] == '&' || data[pos] == '!') {
				end := bytes.IndexAny(data[pos:], " \t\r\n")
				if end < 0 {
					return
				}
				pos += end
				for pos < len(data) && (data[pos] == ' ' || data[pos] == '\t') {
					pos++
				}
			}
			if pos < len(data) && data[pos] == '"' {
				for i := pos + 1; i+1 < len(data) && data[i] != '"'; i++ {
					if data[i] != '\\' {
						continue
					}
					width := unicodeEscapeWidth(data[i+1])
					if width != 0 && i+2+width <= len(data) {
						code, err := strconv.ParseUint(string(data[i+2:i+2+width]), 16, 32)
						r := rune(code)
						if err == nil && r >= 0xa0 && utf8.ValidRune(r) && unicode.IsGraphic(r) && r != '\u2028' && r != '\u2029' {
							out.Write(data[copied:i])
							out.WriteRune(r)
							copied = i + 2 + width
							i = copied - 1
							continue
						}
					}
					i++
				}
			}
		}
		for _, child := range node.Content {
			visit(child)
		}
	}
	visit(&doc)
	if copied == 0 {
		return data
	}
	out.Write(data[copied:])
	return out.Bytes()
}

func unicodeEscapeWidth(kind byte) int {
	switch kind {
	case 'x':
		return 2
	case 'u':
		return 4
	case 'U':
		return 8
	}
	return 0
}
