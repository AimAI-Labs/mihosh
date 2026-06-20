package sub

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	s.showMergeEditor = true
	assert.Equal(t, ModeMergeEditor, s.Mode())

	s.showMergeEditor = false
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
	assert.True(t, subFuzzy("GC", "Group C"))
	assert.True(t, subFuzzy("gc", "Group C")) // 大小写不敏感
	assert.False(t, subFuzzy("XYZ", "Group C"))
	assert.True(t, subFuzzy("", "anything"))
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

func TestHandleMergeLoaded_OnlyMatchingUID(t *testing.T) {
	editor := newMergeEditor("a1")
	s := State{mergeLoadingUID: "a1", mergeEditor: editor}

	// 不匹配的 UID 应被丢弃，不改变状态
	s = s.HandleMergeLoaded("b2", []byte("x"), nil)
	assert.False(t, s.mergeEditor.ready, "不匹配的 UID 不应回填")

	// 匹配的 UID 应回填（内容存入 pending，渲染时才 flush 到 textarea）
	s = s.HandleMergeLoaded("a1", []byte("mode: rule"), nil)
	assert.True(t, s.mergeEditor.ready)
	assert.Equal(t, "mode: rule", s.mergeEditor.pending)
	assert.False(t, s.mergeEditor.flushed, "未渲染时 pending 尚未 flush")
}

func TestHandleMergeLoaded_EditorNotOpened(t *testing.T) {
	// 编辑器未初始化（零值 mergeEditor，uid=""）时不应 panic
	s := State{mergeLoadingUID: "a1"}
	s = s.HandleMergeLoaded("a1", []byte("mode: rule"), nil)
	assert.False(t, s.mergeEditor.ready, "编辑器未打开时应忽略加载结果")
}
