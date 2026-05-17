package common

import (
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/bubbles/key"
)

// KeyMap 快捷键定义
type KeyMap struct {
	Up        key.Binding
	Down      key.Binding
	Left      key.Binding
	Right     key.Binding
	Enter     key.Binding
	Test      key.Binding
	TestAll   key.Binding
	Quit      key.Binding
	Refresh   key.Binding
	NextPage  key.Binding
	PrevPage  key.Binding
	Page1     key.Binding
	Page2     key.Binding
	Page3     key.Binding
	Page4     key.Binding
	Page5     key.Binding
	Escape    key.Binding
	Save      key.Binding
	Backspace key.Binding
	Delete    key.Binding
	Home      key.Binding
	End       key.Binding
	Clear        key.Binding
	LogLevelDown key.Binding
	LogLevelUp   key.Binding
}

var Keys = KeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "上"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "下"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "左"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "右"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "确认"),
	),
	Test: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "测速"),
	),
	TestAll: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "全测"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "刷新"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "退出"),
	),
	NextPage: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "下一页"),
	),
	PrevPage: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "上一页"),
	),
	Page1: key.NewBinding(
		key.WithKeys("1"),
		key.WithHelp("1", "节点"),
	),
	Page2: key.NewBinding(
		key.WithKeys("2"),
		key.WithHelp("2", "连接"),
	),
	Page3: key.NewBinding(
		key.WithKeys("3"),
		key.WithHelp("3", "日志"),
	),
	Page4: key.NewBinding(
		key.WithKeys("4"),
		key.WithHelp("4", "规则"),
	),
	Page5: key.NewBinding(
		key.WithKeys("5"),
		key.WithHelp("5", "设置"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "取消"),
	),
	Save: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "保存"),
	),
	Backspace: key.NewBinding(
		key.WithKeys("backspace", "ctrl+h"),
		key.WithHelp("backspace", "删除"),
	),
	Delete: key.NewBinding(
		key.WithKeys("delete"),
		key.WithHelp("delete", "删除后字符"),
	),
	Home: key.NewBinding(
		key.WithKeys("home", "ctrl+a"),
		key.WithHelp("home", "行首"),
	),
	End: key.NewBinding(
		key.WithKeys("end", "ctrl+e"),
		key.WithHelp("end", "行尾"),
	),
	Clear: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "清空"),
	),
	LogLevelDown: key.NewBinding(
		key.WithKeys("["),
		key.WithHelp("[", "级别-"),
	),
	LogLevelUp: key.NewBinding(
		key.WithKeys("]"),
		key.WithHelp("]", "级别+"),
	),
}

// InitKeyBindings initializes the help descriptions for key bindings using i18n.
// This must be called after i18n.Init().
func InitKeyBindings() {
	Keys.Up.SetHelp("↑/k", i18n.T("key.up"))
	Keys.Down.SetHelp("↓/j", i18n.T("key.down"))
	Keys.Left.SetHelp("←/h", i18n.T("key.left"))
	Keys.Right.SetHelp("→/l", i18n.T("key.right"))
	Keys.Enter.SetHelp("enter", i18n.T("key.enter"))
	Keys.Test.SetHelp("t", i18n.T("key.test"))
	Keys.TestAll.SetHelp("a", i18n.T("key.testall"))
	Keys.Refresh.SetHelp("r", i18n.T("key.refresh"))
	Keys.Quit.SetHelp("q", i18n.T("key.quit"))
	Keys.NextPage.SetHelp("tab", i18n.T("key.nextpage"))
	Keys.PrevPage.SetHelp("shift+tab", i18n.T("key.prevpage"))
	Keys.Page1.SetHelp("1", i18n.T("key.nodes"))
	Keys.Page2.SetHelp("2", i18n.T("key.connections"))
	Keys.Page3.SetHelp("3", i18n.T("key.logs"))
	Keys.Page4.SetHelp("4", i18n.T("key.rules"))
	Keys.Page5.SetHelp("5", i18n.T("key.settings"))
	Keys.Escape.SetHelp("esc", i18n.T("key.escape"))
	Keys.Save.SetHelp("s", i18n.T("key.save"))
	Keys.Backspace.SetHelp("backspace", i18n.T("key.backspace"))
	Keys.Delete.SetHelp("delete", i18n.T("key.delete"))
	Keys.Home.SetHelp("home", i18n.T("key.home"))
	Keys.End.SetHelp("end", i18n.T("key.end"))
	Keys.Clear.SetHelp("c", i18n.T("key.clear"))
	Keys.LogLevelDown.SetHelp("[", i18n.T("key.logleveldown"))
	Keys.LogLevelUp.SetHelp("]", i18n.T("key.loglevelup"))
}
