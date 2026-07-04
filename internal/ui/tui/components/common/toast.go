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
			Foreground(Bright()).
			Background(Success()).
			Padding(0, 1)
	case ToastError:
		icon = "✗"
		style = lipgloss.NewStyle().
			Foreground(Bright()).
			Background(Danger()).
			Padding(0, 1)
	case ToastWarning:
		icon = "⚠"
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(Warning()).
			Padding(0, 1)
	case ToastInfo:
		icon = "ℹ"
		style = lipgloss.NewStyle().
			Foreground(Bright()).
			Background(Info()).
			Padding(0, 1)
	default:
		icon = "•"
		style = lipgloss.NewStyle().
			Foreground(Bright()).
			Background(Gray()).
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
	toasts       []Toast
	lastShown    map[string]time.Time // 记录每条消息最后显示的时间
	throttleTime time.Duration        // 限流时间间隔
}

// NewToastManager 创建 Toast 管理器
func NewToastManager() *ToastManager {
	return &ToastManager{
		lastShown:    make(map[string]time.Time),
		throttleTime: 2 * time.Second, // 默认限流 2 秒
	}
}

// SetThrottleTime 设置限流时间
func (m *ToastManager) SetThrottleTime(duration time.Duration) {
	m.throttleTime = duration
}

// shouldShowToast 检查是否应该显示该消息（限流检查）
func (m *ToastManager) shouldShowToast(msg string) bool {
	lastTime, exists := m.lastShown[msg]
	if !exists {
		return true
	}
	return time.Since(lastTime) >= m.throttleTime
}

// recordToast 记录消息已显示
func (m *ToastManager) recordToast(msg string) {
	m.lastShown[msg] = time.Now()
}

// Add 添加 Toast（带限流）
func (m *ToastManager) Add(msg string, toastType ToastType, duration time.Duration) {
	// 限流检查：如果相同消息在限流时间内已显示过，则忽略
	if !m.shouldShowToast(msg) {
		return
	}

	// 记录该消息已显示
	m.recordToast(msg)

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

// CleanExpired 清理过期的 Toast 和限流记录
func (m *ToastManager) CleanExpired() {
	var valid []Toast
	for _, t := range m.toasts {
		if !t.IsExpired() {
			valid = append(valid, t)
		}
	}
	m.toasts = valid

	// 清理过期的限流记录（保留最近 10 秒的记录）
	now := time.Now()
	for msg, lastTime := range m.lastShown {
		if now.Sub(lastTime) > 10*time.Second {
			delete(m.lastShown, msg)
		}
	}
}
