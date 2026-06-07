package model

var (
	// Version 应用程序版本 (由 -ldflags 注入)
	Version = "dev"
	// Commit 提交哈希 (由 -ldflags 注入)
	Commit = "unknown"
	// Date 构建时间 (由 -ldflags 注入)
	Date = "unknown"
)
