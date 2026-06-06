package connections

import (
	"strings"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections/components"
	tea "github.com/charmbracelet/bubbletea"
)

func TestConnsHandleMouseLeft_DoubleClickConnectionEntersDetail(t *testing.T) {
	state := State{
		Connections: &model.ConnectionsResponse{
			Connections: []model.Connection{
				{
					ID: "conn-1",
					Metadata: model.Metadata{
						Host:          "example.com",
						DestinationIP: "1.1.1.1",
					},
				},
			},
		},
		connViewMode: ConnViewActive,
	}

	const width, height = 120, 30
	x, y, ok := findConnMousePoint(state, width, height, MouseTargetConnection, 0)
	if !ok {
		t.Fatalf("failed to locate first connection row mouse point")
	}

	next, cmd := state.HandleMouseLeft(x, y, width, height, nil, 3000)
	if cmd != nil {
		t.Fatalf("expected nil cmd on first click, got non-nil")
	}
	if next.connDetailMode {
		t.Fatalf("expected detail mode disabled on first click")
	}

	next, cmd = next.HandleMouseLeft(x, y, width, height, nil, 3000)
	if cmd == nil {
		t.Fatalf("expected non-nil cmd on connection double click")
	}
	if !next.connDetailMode {
		t.Fatalf("expected detail mode enabled after double click")
	}
	if next.connDetailSnapshot == nil || next.connDetailSnapshot.ID != "conn-1" {
		t.Fatalf("expected detail snapshot for conn-1, got %#v", next.connDetailSnapshot)
	}
}

func TestConnsHandleMouseLeft_ClickHistoryTabSwitchesView(t *testing.T) {
	state := State{
		Connections:   &model.ConnectionsResponse{},
		connViewMode:  ConnViewActive,
		selectedConn:  5,
		connScrollTop: 3,
	}

	const width, height = 120, 30
	x, y, ok := findConnMousePoint(state, width, height, MouseTargetViewHistory, -1)
	if !ok {
		t.Fatalf("failed to locate history tab mouse point")
	}

	next, cmd := state.HandleMouseLeft(x, y, width, height, nil, 3000)
	if cmd != nil {
		t.Fatalf("expected nil cmd when switching view tab")
	}
	if next.connViewMode != ConnViewHistory {
		t.Fatalf("expected history view mode, got %d", next.connViewMode)
	}
	if next.selectedConn != 0 || next.connScrollTop != 0 {
		t.Fatalf("expected selection reset after tab switch, got selected=%d scrollTop=%d", next.selectedConn, next.connScrollTop)
	}
}

func TestConnsHandleMouseLeft_ClickActiveTabSwitchesView(t *testing.T) {
	state := State{
		Connections:  &model.ConnectionsResponse{},
		connViewMode: ConnViewHistory,
	}
	state.appendClosed(model.Connection{ID: "closed-1", Metadata: model.Metadata{Host: "closed.example"}})

	const width, height = 120, 30
	x, y, ok := findConnMousePoint(state, width, height, MouseTargetViewActive, -1)
	if !ok {
		t.Fatalf("failed to locate active tab mouse point")
	}

	next, cmd := state.HandleMouseLeft(x, y, width, height, nil, 3000)
	if cmd != nil {
		t.Fatalf("expected nil cmd when switching view tab")
	}
	if next.connViewMode != ConnViewActive {
		t.Fatalf("expected active view mode, got %d", next.connViewMode)
	}
}

func TestConnsHandleMouseLeft_DoubleClickHistoryConnectionEntersDetail(t *testing.T) {
	state := State{
		Connections:  &model.ConnectionsResponse{},
		connViewMode: ConnViewHistory,
	}
	state.appendClosed(model.Connection{
		ID: "closed-1",
		Metadata: model.Metadata{
			Host:          "closed.example.com",
			DestinationIP: "8.8.8.8",
		},
	})

	const width, height = 120, 30
	x, y, ok := findConnMousePoint(state, width, height, MouseTargetConnection, 0)
	if !ok {
		t.Fatalf("failed to locate first history connection row mouse point")
	}

	next, cmd := state.HandleMouseLeft(x, y, width, height, nil, 3000)
	if cmd != nil {
		t.Fatalf("expected nil cmd on first history click, got non-nil")
	}
	if next.connDetailMode {
		t.Fatalf("expected detail mode disabled on first history click")
	}

	next, cmd = next.HandleMouseLeft(x, y, width, height, nil, 3000)
	if cmd == nil {
		t.Fatalf("expected non-nil cmd on history connection double click")
	}
	if !next.connDetailMode {
		t.Fatalf("expected detail mode enabled after history double click")
	}
	if next.connDetailSnapshot == nil || next.connDetailSnapshot.ID != "closed-1" {
		t.Fatalf("expected detail snapshot for closed-1, got %#v", next.connDetailSnapshot)
	}
}

func TestConnsHandleMouseLeft_DoubleClickSiteTestTriggersSiteProbe(t *testing.T) {
	state := State{
		Connections:  &model.ConnectionsResponse{},
		connViewMode: ConnViewTraffic,
		siteTests: []model.SiteTest{
			{Name: "A", URL: "http://a.test"},
			{Name: "B", URL: "http://b.test"},
			{Name: "C", URL: "http://c.test"},
			{Name: "D", URL: "http://d.test"},
		},
	}

	const width, height = 120, 30
	x, y, ok := findConnMousePoint(state, width, height, MouseTargetSiteTest, 1)
	if !ok {
		t.Fatalf("failed to locate second site-test card mouse point")
	}

	next, cmd := state.HandleMouseLeft(x, y, width, height, nil, 3000)
	if cmd != nil {
		t.Fatalf("expected nil cmd on first site click, got non-nil")
	}
	if next.selectedSiteTest != 1 {
		t.Fatalf("expected selectedSiteTest=1, got %d", next.selectedSiteTest)
	}
	if next.siteTests[1].Testing {
		t.Fatalf("expected selected site not testing on first click")
	}

	next, cmd = next.HandleMouseLeft(x, y, width, height, nil, 3000)
	if cmd == nil {
		t.Fatalf("expected non-nil cmd on site card double click")
	}
	if !next.siteTests[1].Testing {
		t.Fatalf("expected selected site testing state true after double click")
	}
}

func TestConnsHandleMouseLeft_DoubleClickChartOpensTopNModal(t *testing.T) {
	state := State{
		Connections: &model.ConnectionsResponse{
			Connections: []model.Connection{
				{ID: "c1", Upload: 10, Download: 100, Metadata: model.Metadata{Process: "app-1"}},
				{ID: "c2", Upload: 20, Download: 200, Metadata: model.Metadata{Process: "app-2"}},
				{ID: "c3", Upload: 30, Download: 300, Metadata: model.Metadata{Process: "app-3"}},
				{ID: "c4", Upload: 40, Download: 400, Metadata: model.Metadata{Process: "app-4"}},
				{ID: "c5", Upload: 50, Download: 500, Metadata: model.Metadata{Process: "app-5"}},
				{ID: "c6", Upload: 60, Download: 600, Metadata: model.Metadata{Process: "app-6"}},
				{ID: "c7", Upload: 70, Download: 700, Metadata: model.Metadata{Process: "app-7"}},
			},
		},
		connViewMode: ConnViewTraffic,
	}

	chart := model.NewChartData(60)
	for i := 0; i < 10; i++ {
		chart.AddSpeedData(int64(100+i*10), int64(80+i*10))
		chart.AddMemoryData(int64(1024 * 1024))
		chart.AddConnCountData(7)
	}

	const width, height = 120, 30
	before := state.ToPageState(chart, width, height)
	if got := len(before.TopNItems); got != 5 {
		t.Fatalf("expected default top5 items, got %d", got)
	}
	if before.TopNModalMode {
		t.Fatalf("expected topN modal closed by default")
	}

	// 图表区域位于模式切换栏之后，通过 findConnMousePoint 定位实际坐标。
	chartX, chartY, ok := findConnMousePoint(state, width, height, MouseTargetChart, -1)
	if !ok {
		// chart 需要 chartData，使用带 chartData 的 pageState 查找
		pageStateWithChart := state.ToPageState(chart, width, height)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				hit := ResolveMouseHit(pageStateWithChart, x, y)
				if hit.Target == MouseTargetChart {
					chartX, chartY, ok = x, y, true
					break
				}
			}
			if ok {
				break
			}
		}
	}
	if !ok {
		t.Fatalf("failed to locate chart area mouse point")
	}
	next, cmd := state.HandleMouseLeft(chartX, chartY, width, height, chart, 3000)
	if cmd != nil {
		t.Fatalf("expected nil cmd on first chart click, got non-nil")
	}
	next, cmd = next.HandleMouseLeft(chartX, chartY, width, height, chart, 3000)
	if cmd != nil {
		t.Fatalf("expected nil cmd on chart double click, got non-nil")
	}

	after := next.ToPageState(chart, width, height)
	if !after.TopNModalMode {
		t.Fatalf("expected topN modal opened after chart double click")
	}
	if got := len(after.TopNItems); got != 5 {
		t.Fatalf("expected page topN keep top5, got %d items", got)
	}
	if got := len(after.TopNModalItems); got != 7 {
		t.Fatalf("expected full ranking in modal after chart double click, got %d items", got)
	}
}

func TestConnsHandleMouseLeft_DoubleClickTopNOpensTopNModal(t *testing.T) {
	state := State{
		Connections: &model.ConnectionsResponse{
			Connections: []model.Connection{
				{ID: "c1", Upload: 10, Download: 100, Metadata: model.Metadata{Process: "app-1"}},
				{ID: "c2", Upload: 20, Download: 200, Metadata: model.Metadata{Process: "app-2"}},
			},
		},
		connViewMode: ConnViewTraffic,
	}

	const width, height = 120, 30
	// 找到 Top N 区域的坐标
	x, y, ok := findConnMousePoint(state, width, height, MouseTargetTopN, -1)
	if !ok {
		t.Fatalf("failed to locate TopN section mouse point")
	}

	next, cmd := state.HandleMouseLeft(x, y, width, height, nil, 3000)
	if cmd != nil {
		t.Fatalf("expected nil cmd on first topN click")
	}
	next, cmd = next.HandleMouseLeft(x, y, width, height, nil, 3000)
	if cmd != nil {
		t.Fatalf("expected nil cmd on topN double click")
	}

	after := next.ToPageState(nil, width, height)
	if !after.TopNModalMode {
		t.Fatalf("expected topN modal opened after TopN double click")
	}
}

func TestConnsHandleMouseLeft_DoubleClickTopNModalItemEntersDetail(t *testing.T) {
	state := State{
		topNModalMode: true,
		Connections: &model.ConnectionsResponse{
			Connections: []model.Connection{
				{
					ID: "conn-top-1",
					Metadata: model.Metadata{
						Host: "top.example.com",
					},
					Download: 1000,
				},
			},
		},
	}

	const width, height = 120, 30
	// 找到 Top N 弹窗中第一项的坐标
	x, y, ok := findConnMousePoint(state, width, height, MouseTargetTopNModalItem, 0)
	if !ok {
		t.Fatalf("failed to locate TopN modal item mouse point")
	}

	next, cmd := state.HandleMouseLeft(x, y, width, height, nil, 3000)
	if cmd != nil {
		t.Fatalf("expected nil cmd on first click")
	}
	if !next.topNModalMode {
		t.Fatalf("expected topN modal still open after first click")
	}

	next, cmd = next.HandleMouseLeft(x, y, width, height, nil, 3000)
	if cmd == nil {
		t.Fatalf("expected non-nil cmd on double click (fetch ip info)")
	}
	if !next.topNModalMode {
		t.Fatalf("expected topN modal retained after double click to allow returning")
	}
	if !next.connDetailMode {
		t.Fatalf("expected detail mode enabled after double click")
	}
	if next.connDetailSnapshot == nil || next.connDetailSnapshot.ID != "conn-top-1" {
		t.Fatalf("expected detail snapshot for conn-top-1, got %#v", next.connDetailSnapshot)
	}

	// 模拟双击详情页面区域以关闭它
	// 双击 (10, 10)
	next, cmd = next.HandleMouseLeft(10, 10, width, height, nil, 3000)
	next, cmd = next.HandleMouseLeft(10, 10, width, height, nil, 3000)
	if next.connDetailMode {
		t.Fatalf("expected detail mode closed after double click")
	}
	if !next.topNModalMode {
		t.Fatalf("expected topN modal still active after closing detail")
	}
}

func TestConnsHandleMouseLeft_ClickOutsideTopNModalCloses(t *testing.T) {
	state := State{
		topNModalMode: true,
		Connections: &model.ConnectionsResponse{
			Connections: []model.Connection{
				{ID: "c1", Upload: 10, Download: 100, Metadata: model.Metadata{Process: "app-1"}},
				{ID: "c2", Upload: 20, Download: 200, Metadata: model.Metadata{Process: "app-2"}},
				{ID: "c3", Upload: 30, Download: 300, Metadata: model.Metadata{Process: "app-3"}},
				{ID: "c4", Upload: 40, Download: 400, Metadata: model.Metadata{Process: "app-4"}},
				{ID: "c5", Upload: 50, Download: 500, Metadata: model.Metadata{Process: "app-5"}},
				{ID: "c6", Upload: 60, Download: 600, Metadata: model.Metadata{Process: "app-6"}},
			},
		},
	}

	const width, height = 120, 30
	items := state.CalculateTopN(0, 5*time.Minute)
	left, top, right, bottom := components.ResolveTopNModalBounds(items, width, height, 0)
	if right <= left || bottom <= top {
		t.Fatalf("invalid topN modal bounds: left=%d top=%d right=%d bottom=%d", left, top, right, bottom)
	}

	clickX := 0
	clickY := 0
	if left > 0 {
		clickX = left - 1
		clickY = top
	}

	next, cmd := state.HandleMouseLeft(clickX, clickY, width, height, nil, 3000)
	if cmd != nil {
		t.Fatalf("expected nil cmd when closing topN modal by outside click")
	}
	if next.topNModalMode {
		t.Fatalf("expected topN modal closed by outside click")
	}
}

func TestConnsHandleMouseLeft_ClickTabClosesDetail(t *testing.T) {
	state := State{
		connDetailMode: true,
		connDetailSnapshot: &model.Connection{
			ID: "conn-1",
			Metadata: model.Metadata{
				Host:          "example.com",
				DestinationIP: "1.1.1.1",
			},
		},
		connIPInfo:            &model.IPInfo{Country: "TW"},
		connDetailLeftScroll:  2,
		connDetailRightScroll: 3,
		connDetailFocusPanel:  1,
	}

	const width, height = 120, 30
	// 找到模式切换栏的某个按钮（例如历史记录按钮）
	x, y, ok := findConnMousePoint(state, width, height, MouseTargetViewHistory, -1)
	if !ok {
		t.Fatalf("failed to locate history tab mouse point")
	}
	
	next, cmd := state.HandleMouseLeft(x, y, width, height, nil, 3000)
	if cmd != nil {
		t.Fatalf("expected nil cmd when closing detail by tab click")
	}
	if next.connDetailMode {
		t.Fatalf("expected detail mode closed by tab click")
	}
	if next.connDetailSnapshot != nil {
		t.Fatalf("expected detail snapshot cleared after close")
	}
	if next.connIPInfo != nil {
		t.Fatalf("expected ip info cleared after close")
	}
	if next.connDetailLeftScroll != 0 || next.connDetailRightScroll != 0 {
		t.Fatalf("expected detail scroll reset after close, got left=%d right=%d", next.connDetailLeftScroll, next.connDetailRightScroll)
	}
	if next.connDetailFocusPanel != 0 {
		t.Fatalf("expected detail focus reset after close, got %d", next.connDetailFocusPanel)
	}
}

func TestConnsHandleMouseLeft_DoubleClickDetailClosesModal(t *testing.T) {
	state := State{
		connDetailMode: true,
		connDetailSnapshot: &model.Connection{
			ID: "conn-1",
			Metadata: model.Metadata{
				Host:          "example.com",
				DestinationIP: "1.1.1.1",
			},
		},
	}

	const width, height = 120, 30
	// 点击详情区域（在模式切换栏下方），y = 10
	clickX := 10
	clickY := 10

	next, cmd := state.HandleMouseLeft(clickX, clickY, width, height, nil, 3000)
	if cmd != nil {
		t.Fatalf("expected nil cmd when clicking inside detail")
	}
	if !next.connDetailMode {
		t.Fatalf("expected detail mode keep open on single click")
	}
	if next.connDetailSnapshot == nil || next.connDetailSnapshot.ID != "conn-1" {
		t.Fatalf("expected detail snapshot retained, got %#v", next.connDetailSnapshot)
	}

	// 双击退出
	next, cmd = next.HandleMouseLeft(clickX, clickY, width, height, nil, 3000)
	if cmd != nil {
		t.Fatalf("expected nil cmd on double click")
	}
	if next.connDetailMode {
		t.Fatalf("expected detail mode close on double click")
	}
}

func TestConnsDetailInputTakesPriorityOverTopNModal(t *testing.T) {
	state := State{
		topNModalMode: true,
		Connections: &model.ConnectionsResponse{
			Connections: []model.Connection{
				{ID: "conn-1", Download: 100, Metadata: model.Metadata{Host: "example.com"}},
			},
		},
		connDetailMode: true,
		connDetailSnapshot: &model.Connection{
			ID: "conn-1",
			Metadata: model.Metadata{
				Host:        "example.com",
				ProcessPath: strings.Repeat("C:/very/long/path/", 20),
			},
			Chains: []string{"a", "b", "c", "d", "e", "f", "g", "h"},
		},
		connDetailFocusPanel:    1,
		connDetailJSONLineCount: 40,
	}

	next, cmd := state.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, nil, 3000)
	if cmd != nil {
		t.Fatalf("expected nil cmd while scrolling detail, got non-nil")
	}
	if next.connDetailRightScroll != 1 {
		t.Fatalf("expected right detail scroll to advance, got %d", next.connDetailRightScroll)
	}
	if next.topNModalScroll != 0 {
		t.Fatalf("expected hidden topN modal scroll unchanged, got %d", next.topNModalScroll)
	}

	next = next.HandleMouseScroll(false, 90, 10, 120, 30)
	if next.connDetailRightScroll != 2 {
		t.Fatalf("expected mouse wheel to advance right detail scroll, got %d", next.connDetailRightScroll)
	}
	if next.topNModalScroll != 0 {
		t.Fatalf("expected hidden topN modal scroll unchanged after mouse wheel, got %d", next.topNModalScroll)
	}
}

func TestConnsChartDoubleClickThreshold_Allows500msGap(t *testing.T) {
	var state State
	base := time.Unix(1000, 0)

	if state.isMouseDoubleClickWithThreshold(MouseTargetChart, 0, base, connsChartDoubleClickMax) {
		t.Fatalf("first click should not be treated as double click")
	}

	second := base.Add(500 * time.Millisecond)
	if !state.isMouseDoubleClickWithThreshold(MouseTargetChart, 0, second, connsChartDoubleClickMax) {
		t.Fatalf("expected chart second click within 500ms to be treated as double click")
	}
}

func findConnMousePoint(
	state State,
	width int,
	height int,
	target MouseTarget,
	index int,
) (int, int, bool) {
	pageState := state.ToPageState(nil, width, height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			hit := ResolveMouseHit(pageState, x, y)
			if hit.Target != target {
				if (target == MouseTargetChart || target == MouseTargetTopN) && (hit.Target == MouseTargetChart || hit.Target == MouseTargetTopN) {
					// chart and topN are overlapping or related
				} else {
					continue
				}
			}
			if index >= 0 && hit.Index != index {
				continue
			}
			return x, y, true
		}
	}
	return 0, 0, false
}
