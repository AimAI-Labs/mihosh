package utils

import "fmt"

// CreateHyperlink 创建一个支持终端原生点击的 OSC 8 超链接字符串
func CreateHyperlink(url, text string) string {
	// \x1b]8;;{url}\x1b\ {text} \x1b]8;;\x1b\
	return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", url, text)
}
