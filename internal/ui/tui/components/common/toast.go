package common

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ToastType Toast 类型
type ToastType int

const (
	ToastSuccess ToastType = iota
	ToastError
	ToastWarning
	ToastInfo
)

// Toast 系统消息
type Toast struct {
	Message   string
	Type      ToastType
	CreatedAt time.Time
	Duration  time.Duration
}

// NewToast 创建新的 Toast
func NewToast(msg string, toastType ToastType, duration time.Duration) Toast {
	return Toast{
		Message:   msg,
		Type:      toastType,
		CreatedAt: time.Now(),
		Duration:  duration,
	}
}

// IsExpired 检查 Toast 是否已过期
func (t Toast) IsExpired() bool {
	return time.Since(t.CreatedAt) > t.Duration
}

// RenderToast 渲染单个 Toast
func RenderToast(toast Toast) string {
	var icon string
	var style lipgloss.Style

	switch toast.Type {
	case ToastSuccess:
		icon = "✓"
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#2ECC71")).
			Padding(0, 1)
	case ToastError:
		icon = "✗"
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#E74C3C")).
			Padding(0, 1)
	case ToastWarning:
		icon = "⚠"
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#FFD700")).
			Padding(0, 1)
	case ToastInfo:
		icon = "ℹ"
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#3498DB")).
			Padding(0, 1)
	default:
		icon = "•"
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#95A5A6")).
			Padding(0, 1)
	}

	content := icon + " " + toast.Message
	return style.Render(content)
}

// RenderToasts 渲染多个 Toast（堆叠显示在右上角）
func RenderToasts(toasts []Toast, width int) string {
	if len(toasts) == 0 {
		return ""
	}

	var validToasts []Toast
	for _, t := range toasts {
		if !t.IsExpired() {
			validToasts = append(validToasts, t)
		}
	}

	if len(validToasts) == 0 {
		return ""
	}

	// 最多显示 3 个 Toast
	if len(validToasts) > 3 {
		validToasts = validToasts[len(validToasts)-3:]
	}

	var lines []string
	for _, t := range validToasts {
		toastStr := RenderToast(t)
		// 右对齐
		toastWidth := lipgloss.Width(toastStr)
		padding := width - toastWidth
		if padding < 0 {
			padding = 0
		}
		lines = append(lines, strings.Repeat(" ", padding)+toastStr)
	}

	return strings.Join(lines, "\n")
}

// ToastManager Toast 管理器
type ToastManager struct {
	toasts []Toast
}

// NewToastManager 创建 Toast 管理器
func NewToastManager() *ToastManager {
	return &ToastManager{}
}

// Add 添加 Toast
func (m *ToastManager) Add(msg string, toastType ToastType, duration time.Duration) {
	m.toasts = append(m.toasts, NewToast(msg, toastType, duration))
}

// Success 添加成功 Toast
func (m *ToastManager) Success(msg string) {
	m.Add(msg, ToastSuccess, 2*time.Second)
}

// Error 添加错误 Toast
func (m *ToastManager) Error(msg string) {
	m.Add(msg, ToastError, 3*time.Second)
}

// Warning 添加警告 Toast
func (m *ToastManager) Warning(msg string) {
	m.Add(msg, ToastWarning, 2500*time.Millisecond)
}

// Info 添加信息 Toast
func (m *ToastManager) Info(msg string) {
	m.Add(msg, ToastInfo, 2*time.Second)
}

// Render 渲染所有 Toast
func (m *ToastManager) Render(width int) string {
	return RenderToasts(m.toasts, width)
}

// CleanExpired 清理过期的 Toast
func (m *ToastManager) CleanExpired() {
	var valid []Toast
	for _, t := range m.toasts {
		if !t.IsExpired() {
			valid = append(valid, t)
		}
	}
	m.toasts = valid
}
