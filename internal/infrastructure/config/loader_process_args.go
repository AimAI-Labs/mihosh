package config

import (
	"bytes"
	"encoding/binary"
	"path/filepath"
	"strings"
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
