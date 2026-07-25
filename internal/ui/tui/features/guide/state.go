package guide

// State 封装引导弹窗状态
type State struct {
	Active        bool
	Endpoint      string
	SecretMasked  string
	ErrMessage    string
	FocusedButton int // 0: 前往配置, 1: 重试连接, 2: 暂不配置
}

// Activate 开启并初始化引导弹窗状态
func (s State) Activate(endpoint, secretMasked, errMsg string) State {
	s.Active = true
	s.Endpoint = endpoint
	s.SecretMasked = secretMasked
	s.ErrMessage = errMsg
	s.FocusedButton = 0
	return s
}

// Dismiss 关闭引导弹窗
func (s State) Dismiss() State {
	s.Active = false
	return s
}

// NextButton 聚焦下一个操作按钮
func (s State) NextButton() State {
	s.FocusedButton = (s.FocusedButton + 1) % 3
	return s
}

// PrevButton 聚焦上一个操作按钮
func (s State) PrevButton() State {
	s.FocusedButton = (s.FocusedButton + 2) % 3
	return s
}
