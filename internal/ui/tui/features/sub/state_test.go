package sub

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRuneKey 构造单字符按键消息（测试辅助）。
func newRuneKey(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// sampleSubs 构造一组测试用订阅。
func sampleSubs() []profile.Profile {
	return []profile.Profile{
		{UID: "a1", Name: "订阅A", Source: profile.SubSource{Kind: profile.SourceRemote, URL: "https://a.io/sub"}},
		{UID: "b2", Name: "订阅B", Source: profile.SubSource{Kind: profile.SourceLocal, Path: "/etc/mihomo/b.yaml"}},
		{UID: "c3", Name: "Group C", Source: profile.SubSource{Kind: profile.SourceRemote, URL: "https://c.io/sub"}},
	}
}

func TestApplySubs_BuildsFilterCache(t *testing.T) {
	s := State{}.ApplySubs(sampleSubs(), "a1")

	assert.Len(t, s.filteredIdx, 3, "无过滤时应包含全部订阅")
	assert.Equal(t, "a1", s.activeUID)
}

func TestApplySubs_EmptyResetsFilter(t *testing.T) {
	s := State{
		filteredIdx: []int{0, 1, 2},
		filter:      "x",
	}.ApplySubs(nil, "")
	assert.Empty(t, s.filteredIdx)
	assert.Empty(t, s.activeUID)
}

func TestState_QueryingAndMode(t *testing.T) {
	s := State{}
	assert.False(t, s.Querying())
	assert.Equal(t, ModeNormal, s.Mode())

	s.filterMode = true
	assert.True(t, s.Querying())
	assert.Equal(t, ModeSearch, s.Mode())

	s.filterMode = false
	s.showAddForm = true
	assert.Equal(t, ModeAddForm, s.Mode())

	s.showAddForm = false
	s.showDeleteConf = true
	assert.Equal(t, ModeDeleteConf, s.Mode())
}

func TestFilter_Substring(t *testing.T) {
	s := State{filterEngine: FilterSubstring}.ApplySubs(sampleSubs(), "")
	s.filter = "订阅"
	s.updateFiltered()

	var got []string
	for _, idx := range s.filteredIdx {
		got = append(got, s.subs[idx].Name)
	}
	assert.Equal(t, []string{"订阅A", "订阅B"}, got)
}

func TestFilter_Fuzzy(t *testing.T) {
	s := State{filterEngine: FilterFuzzy}.ApplySubs(sampleSubs(), "")
	// "GC" 应模糊匹配 "Group C"（按顺序子序列）
	s.filter = "GC"
	s.updateFiltered()
	require.Len(t, s.filteredIdx, 1)
	assert.Equal(t, "c3", s.subs[s.filteredIdx[0]].UID)
}

func TestFilter_Regex(t *testing.T) {
	s := State{filterEngine: FilterRegex}.ApplySubs(sampleSubs(), "")
	s.filter = `^订阅`
	s.updateFiltered()
	require.Len(t, s.filteredIdx, 2)
}

func TestFilter_RemoteKind(t *testing.T) {
	// 过滤 "remote" 应匹配所有远程订阅（hay 含 Kind.String）
	s := State{filterEngine: FilterSubstring}.ApplySubs(sampleSubs(), "")
	s.filter = "remote"
	s.updateFiltered()
	require.Len(t, s.filteredIdx, 2, "应有 2 个 remote")
	for _, idx := range s.filteredIdx {
		assert.Equal(t, profile.SourceRemote, s.subs[idx].Source.Kind)
	}
}

func TestFilter_EmptyShowsAll(t *testing.T) {
	s := State{}.ApplySubs(sampleSubs(), "")
	s.filter = "zzz"
	s.updateFiltered()
	require.Len(t, s.filteredIdx, 0)
	// 清空过滤恢复全部
	s.filter = ""
	s.updateFiltered()
	require.Len(t, s.filteredIdx, 3)
}

func TestSubFuzzy(t *testing.T) {
	assert.True(t, common.FuzzyMatch("GC", "Group C"))
	assert.True(t, common.FuzzyMatch("gc", "Group C")) // 大小写不敏感
	assert.False(t, common.FuzzyMatch("XYZ", "Group C"))
	assert.True(t, common.FuzzyMatch("", "anything"))
}

func TestClampScroll(t *testing.T) {
	s := State{}.ApplySubs(sampleSubs(), "")
	s.selected = 2
	s.clampScroll()
	// 选中第 3 项（索引 2）应在可视范围内
	assert.GreaterOrEqual(t, s.selected, s.scrollTop)
}

func TestActivateSelected_NoSelection(t *testing.T) {
	s := State{}.ApplySubs([]profile.Profile{}, "")
	_, cmd := s.activateSelected(nil)
	assert.Nil(t, cmd, "空列表激活应无命令")
}

func TestOpenDeleteConfirm(t *testing.T) {
	s := State{}.ApplySubs(sampleSubs(), "")
	s.selected = 1
	s, _ = s.openDeleteConfirm()
	require.True(t, s.showDeleteConf)
	assert.Equal(t, "b2", s.deleteUID)
}

func TestOpenDeleteConfirm_EmptyList(t *testing.T) {
	s := State{}.ApplySubs(nil, "")
	s, _ = s.openDeleteConfirm()
	assert.False(t, s.showDeleteConf)
}

func TestUpdate_MKeyOpensMergeEditor(t *testing.T) {
	// 按 m 应返回非 nil 命令（tea.ExecProcess 启动外部编辑器）。
	s := State{}.ApplySubs(sampleSubs(), "")
	s.selected = 0
	_, cmd := s.Update(newRuneKey("m"), nil)
	require.NotNil(t, cmd, "m 应发出打开外部编辑器的命令")
}

func TestOpenMergeExternalEditor_NoSelection(t *testing.T) {
	// 无论列表是否为空，都可以打开全局覆写配置。
	s := State{}.ApplySubs(nil, "")
	_, cmd := s.openMergeExternalEditor(nil)
	assert.NotNil(t, cmd, "空列表也应该发出编辑器命令")
}

func TestOpenEditForm_PrefillsCurrentSub(t *testing.T) {
	s := State{}.ApplySubs(sampleSubs(), "")
	s.selected = 1 // 选中 b2（local）

	s, _ = s.openEditForm(nil)
	require.True(t, s.showEditForm)
	assert.Equal(t, "b2", s.editUID)
	// 名称与来源应预填
	assert.Equal(t, "订阅B", s.editForm.nameField.Value())
	assert.Equal(t, "/etc/mihomo/b.yaml", s.editForm.srcField.Value())
	assert.False(t, s.editForm.kindRemote, "local 订阅应标记为非 remote")
}

func TestOpenEditForm_EmptyList(t *testing.T) {
	s := State{}.ApplySubs(nil, "")
	s, _ = s.openEditForm(nil)
	assert.False(t, s.showEditForm, "空列表不应打开编辑表单")
}

func TestUpdate_EKeyOpensEditForm(t *testing.T) {
	// 按 e 应打开编辑表单
	s := State{}.ApplySubs(sampleSubs(), "")
	s.selected = 0
	s, _ = s.Update(newRuneKey("e"), nil)
	assert.True(t, s.showEditForm, "e 应触发编辑表单")
	assert.Equal(t, "a1", s.editUID)
}

func TestHandleEditFormSubmit_DispatchesEditCmd(t *testing.T) {
	s := State{}.ApplySubs(sampleSubs(), "")
	s.selected = 1
	s, _ = s.openEditForm(nil)

	// 修改名称后按 Enter 提交
	form := s.editForm
	form.nameField.SetValue("新名称")
	form.srcField.SetValue("https://new.io/sub")
	s.editForm = form

	s, cmd := s.handleEditFormUpdate(tea.KeyMsg{Type: tea.KeyEnter}, nil)
	require.NotNil(t, cmd)
	assert.False(t, s.showEditForm, "提交后表单应关闭")
	assert.Empty(t, s.editUID)
}

func TestHandleEditFormCancel_ClosesForm(t *testing.T) {
	s := State{}.ApplySubs(sampleSubs(), "")
	s.selected = 0
	s, _ = s.openEditForm(nil)

	s, cmd := s.handleEditFormUpdate(tea.KeyMsg{Type: tea.KeyEsc}, nil)
	assert.Nil(t, cmd)
	assert.False(t, s.showEditForm, "Esc 应关闭编辑表单")
	assert.Empty(t, s.editUID)
}
