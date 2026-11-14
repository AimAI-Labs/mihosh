package utils

import (
	"fmt"
	"time"
)

// FormatBytes 格式化字节数为人类可读格式
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// MaskSecret 掩码敏感信息
func MaskSecret(secret string) string {
	if secret == "" {
		return "<未设置>"
	}
	if len(secret) <= 6 {
		return "****"
	}
	return secret[:3] + "****" + secret[len(secret)-3:]
}

// FormatDuration 格式化连接时长
func FormatDuration(startStr string) string {
	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		return "-"
	}

	duration := time.Since(start)
	if duration < time.Minute {
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	}
	if duration < time.Hour {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	}
	return fmt.Sprintf("%dh%dm", int(duration.Hours()), int(duration.Minutes())%60)
}
