package config

import (
	"bytes"
	"encoding/binary"
	"path/filepath"
	"strings"
	"unicode"
)

// parseDarwinProcessArguments decodes argc, the saved exec path, padding and
// exactly argc NUL-terminated arguments from kern.procargs2.
func parseDarwinProcessArguments(raw []byte) []string {
	if len(raw) < 4 {
		return nil
	}
	count := int(binary.NativeEndian.Uint32(raw[:4]))
	_, args, ok := bytes.Cut(raw[4:], []byte{0})
	args = bytes.TrimLeft(args, "\x00")
	if !ok || count <= 0 || count > len(args) {
		return nil
	}
	result := make([]string, 0, count)
	for range count {
		var arg []byte
		arg, args, ok = bytes.Cut(args, []byte{0})
		if !ok {
			return nil
		}
		result = append(result, string(arg))
	}
	return result
}

func isMihomoExecutable(executable string) bool {
	return strings.Contains(filepath.Base(executable), "mihomo")
}

// splitSystemdCommand handles quoted ExecStart arguments when a live process
// is unavailable. Running processes are read with their original argv.
func splitSystemdCommand(line string) []string {
	var args []string
	var field strings.Builder
	var quote rune
	escaped := false
	for _, char := range line {
		switch {
		case escaped:
			field.WriteRune(char)
			escaped = false
		case char == '\\' && quote != '\'':
			escaped = true
		case quote != 0:
			if char == quote {
				quote = 0
			} else {
				field.WriteRune(char)
			}
		case char == '"' || char == '\'':
			quote = char
		case unicode.IsSpace(char):
			if field.Len() != 0 {
				args = append(args, field.String())
				field.Reset()
			}
		default:
			field.WriteRune(char)
		}
	}
	if quote != 0 || escaped {
		return nil
	}
	if field.Len() != 0 {
		args = append(args, field.String())
	}
	return args
}
