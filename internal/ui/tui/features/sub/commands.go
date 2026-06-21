package sub

// commands.go — 把 ProfileService 的纯方法包装为 tea.Cmd。
//
// 保持 service 层不依赖 bubbletea（与 ProxyService/ConfigService 一致），
// 所有 tea.Cmd 在此封装，返回 messages.Sub* 消息。

import (
	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
)

// FetchSubs 从配置加载订阅列表与激活 UID。
func FetchSubs(svc *service.ProfileService) tea.Cmd {
	return func() tea.Msg {
		if svc == nil {
			return messages.SubsLoadedMsg{Subs: []profile.Profile{}, Active: ""}
		}
		subs, active, err := svc.ListProfiles()
		if err != nil {
			return messages.ErrMsg{Err: err}
		}
		return messages.SubsLoadedMsg{Subs: subs, Active: active}
	}
}

// AddSubCmd 新增订阅。
func AddSubCmd(svc *service.ProfileService, name string, src profile.SubSource) tea.Cmd {
	return func() tea.Msg {
		if svc == nil {
			return messages.SubAddErrorMsg{Err: errNoService}
		}
		p, err := svc.AddProfile(name, src)
		if err != nil {
			return messages.SubAddErrorMsg{Err: err}
		}
		return messages.SubAddDoneMsg{UID: p.UID}
	}
}

// EditSubCmd 编辑订阅元数据（名称 + 来源）。
func EditSubCmd(svc *service.ProfileService, uid, name string, src profile.SubSource) tea.Cmd {
	return func() tea.Msg {
		if svc == nil {
			return messages.SubEditErrorMsg{UID: uid, Err: errNoService}
		}
		if err := svc.EditProfile(uid, name, src); err != nil {
			return messages.SubEditErrorMsg{UID: uid, Err: err}
		}
		return messages.SubEditDoneMsg{UID: uid}
	}
}

// DeleteSubCmd 删除订阅。
func DeleteSubCmd(svc *service.ProfileService, uid string) tea.Cmd {
	return func() tea.Msg {
		if svc == nil {
			return messages.SubDeleteErrorMsg{Err: errNoService}
		}
		if err := svc.DeleteProfile(uid); err != nil {
			return messages.SubDeleteErrorMsg{Err: err}
		}
		return messages.SubDeletedMsg{UID: uid}
	}
}

// UpdateSubCmd 拉取/更新某订阅的原始配置。
func UpdateSubCmd(svc *service.ProfileService, uid string) tea.Cmd {
	return func() tea.Msg {
		if svc == nil {
			return messages.SubFetchErrorMsg{UID: uid, Err: errNoService}
		}
		if err := svc.FetchProfile(uid); err != nil {
			return messages.SubFetchErrorMsg{UID: uid, Err: err}
		}
		return messages.SubFetchDoneMsg{UID: uid}
	}
}

// ActivateSubCmd 激活订阅（生成 → 备份 → 写入 → 重载）。
func ActivateSubCmd(svc *service.ProfileService, uid string) tea.Cmd {
	return func() tea.Msg {
		if svc == nil {
			return messages.SubActivatedMsg{UID: uid, Err: errNoService}
		}
		res, err := svc.Activate(uid)
		if err != nil {
			return messages.SubActivatedMsg{UID: uid, Err: err}
		}
		return messages.SubActivatedMsg{
			UID:        uid,
			BackupName: res.BackupName,
			MergeErr:   res.MergeErr,
		}
	}
}

// LoadMergeCmd 读取某订阅的 merge.yaml 内容（用于编辑器载入）。
func LoadMergeCmd(svc *service.ProfileService, uid string) tea.Cmd {
	return func() tea.Msg {
		if svc == nil {
			return MergeLoadedMsg{UID: uid, Err: errNoService}
		}
		data, err := svc.LoadMerge(uid)
		return MergeLoadedMsg{UID: uid, Data: data, Err: err}
	}
}

// SaveMergeCmd 保存 merge.yaml 内容。
func SaveMergeCmd(svc *service.ProfileService, uid string, content []byte) tea.Cmd {
	return func() tea.Msg {
		if svc == nil {
			return messages.SubMergeSaveErrorMsg{UID: uid, Err: errNoService}
		}
		if err := svc.SaveMerge(uid, content); err != nil {
			return messages.SubMergeSaveErrorMsg{UID: uid, Err: err}
		}
		return messages.SubMergeSavedMsg{UID: uid}
	}
}

// MergeLoadedMsg 内部消息：merge 内容已读取（由 sub 页与主 Model 消费，
// 不进全局 events.go 以保持载荷内聚）。
type MergeLoadedMsg struct {
	UID  string
	Data []byte
	Err  error
}

var errNoService = errNoServiceErr{}

type errNoServiceErr struct{}

func (errNoServiceErr) Error() string { return "订阅服务未初始化" }
