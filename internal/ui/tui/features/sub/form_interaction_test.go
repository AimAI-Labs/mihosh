package sub

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestEditFormSizesMatchRenderedInputsAndMouseFields(t *testing.T) {
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	for _, language := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(language)
		for _, width := range []int{24, 40, 80, 120} {
			for _, height := range []int{8, 12, 24} {
				s := State{}.ApplySubs([]profile.Profile{{UID: "test", Name: strings.Repeat("长名称", 30), Source: profile.SubSource{Kind: profile.SourceRemote, URL: "https://example.org/" + strings.Repeat("long-url/", 30)}}}, "")
				s, _ = s.Update(messages.PageResizeMsg{Width: width, Height: height})
				s, _ = s.openEditForm()
				_, fieldWidth, sourceHeight := formDimensions(width, height)
				sourceWidth := fieldWidth
				if button := s.editForm.copyButton(fieldWidth); button != "" {
					sourceWidth -= lipgloss.Width(button) + 1
				}
				if s.editForm.nameField.Width != fieldWidth || s.editForm.srcField.Width() != sourceWidth || s.editForm.srcField.Height() != sourceHeight {
					t.Fatal("input dimensions differ from layout")
				}
				modal := ansi.Strip(buildEditFormModal(s.ToPageState(width, height), width, height))
				if lipgloss.Height(modal) > height-1 {
					t.Fatalf("%s %dx%d: modal exceeds available height:\n%s", language, width, height, modal)
				}
				for _, row := range strings.Split(modal, "\n") {
					if lipgloss.Width(row) > width {
						t.Fatalf("modal exceeds width: %q", row)
					}
				}
				view := RenderSubPage(s.ToPageState(width, height))
				if lipgloss.Height(view) > height {
					t.Fatal("form page exceeds terminal height")
				}
				for _, row := range strings.Split(view, "\n") {
					if lipgloss.Width(row) > width {
						t.Fatal("form page exceeds terminal width")
					}
				}
				left, top, _, _ := resolveFormBounds(s.ToPageState(width, height), width, height)
				s, _ = s.Update(messages.PageMouseClickMsg{X: left + 2 + formLabelWidth() + 1, Y: top + 3, Width: width, Height: height})
				if !s.editForm.isSrcField() || !s.editForm.srcField.Focused() {
					t.Fatal("source click did not focus its visible input")
				}
				before := s.editForm.srcField.LineInfo()
				s, _ = s.Update(tea.KeyMsg{Type: tea.KeyUp})
				if !s.editForm.isSrcField() || s.editForm.srcField.LineInfo() == before {
					t.Fatal("up key left the wrapped source instead of moving its cursor")
				}
				s, _ = s.Update(messages.PageMouseScrollMsg{Up: true, X: left + 3, Y: top + 3, Width: width, Height: height})
				if s.selected != 0 || !s.showEditForm || !s.editForm.isSrcField() {
					t.Fatal("wheel escaped the source input")
				}
				s, _ = s.Update(messages.PageMouseClickMsg{X: 0, Y: height - 1, Width: width, Height: height})
				if s.showEditForm || s.editUID != "" {
					t.Fatal("outside close retained editing state")
				}
			}
		}
	}
}

func TestEditSourceCopyHitMatchesRenderedButton(t *testing.T) {
	i18n.Init()
	const width, height = 80, 24
	s := State{}.ApplySubs([]profile.Profile{{UID: "test", Name: "profile", Source: profile.SubSource{Kind: profile.SourceRemote, URL: "https://example.org/profile"}}}, "")
	s, _ = s.openEditForm()
	s.editForm.copySource = true
	s, _ = s.Update(messages.PageResizeMsg{Width: width, Height: height})
	page := s.ToPageState(width, height)
	left, top, _, _ := resolveFormBounds(page, width, height)
	row := strings.Split(ansi.Strip(buildEditFormModal(page, width, height)), "\n")[3]
	label := i18n.T("sub.copy_source")
	before, _, found := strings.Cut(row, label)
	if !found {
		t.Fatalf("source row is missing copy button: %q", row)
	}
	s, cmd := s.HandleMouseLeft(left+lipgloss.Width(before), top+3, width, height)
	if cmd == nil || !s.showEditForm || s.editUID != "test" {
		t.Fatal("clicking Copy should dispatch a copy command and leave the editor open")
	}
}

func TestSourceKindSelectionIsHonoredWithoutBreakingAutomaticDetection(t *testing.T) {
	f := newAddForm()
	f.srcField.SetValue("https://example.org/sub")
	if f.buildSource().Kind != profile.SourceRemote {
		t.Fatal("automatic URL detection failed")
	}
	f.toggleKind()
	f.nameField.SetValue("test")
	if ok, key := f.validate(); ok || key != "sub.add_err_kind" {
		t.Fatal("mismatched type was allowed to reach source normalization")
	}
	if source := f.buildSource(); source.Kind != profile.SourceLocal || source.Path == "" || source.URL != "" {
		t.Fatal("explicit source type was ignored")
	}
	f = newAddForm()
	f.srcField.SetValue("/tmp/profile.yaml")
	if f.buildSource().Kind != profile.SourceLocal {
		t.Fatal("automatic local path detection regressed")
	}
}

func TestEditFormPersistsSelectedSourceWithoutLosingURLOrPath(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	if err := config.Save(&config.DefaultConfig); err != nil {
		t.Fatal(err)
	}
	svc := service.NewProfileService(nil)
	p, err := svc.AddProfile("old", profile.SubSource{Kind: profile.SourceRemote, URL: "https://example.org/sub"})
	if err != nil {
		t.Fatal(err)
	}
	s := NewState(svc).ApplySubs([]profile.Profile{p}, "")
	s, _ = s.openEditForm()
	s.editForm.toggleKind()
	s, cmd := s.handleEditFormUpdate(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil || !s.showEditForm || s.editForm.errMsg != i18n.T("sub.add_err_kind") {
		t.Fatal("invalid type/source pair was submitted")
	}
	s.editForm.srcField.SetValue("/tmp/local.yaml")
	s.editForm.nameField.SetValue("renamed")
	s, cmd = s.handleEditFormUpdate(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || s.showEditForm {
		t.Fatal("valid edit was not submitted")
	}
	if _, ok := cmd().(messages.SubEditDoneMsg); !ok {
		t.Fatal("edit command failed")
	}
	subs, _, err := svc.ListProfiles()
	if err != nil || len(subs) != 1 || subs[0].Name != "renamed" || subs[0].Source.Kind != profile.SourceLocal || subs[0].Source.Path != "/tmp/local.yaml" {
		t.Fatal("selected source was lost during persistence", subs, err)
	}
}

func TestEditFormFetchOptionsPersistAndPreserveUsageForSameURL(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	i18n.Init()
	cfg := config.DefaultConfig
	cfg.Subs = []profile.Profile{{
		UID:  "u1",
		Name: "RemoteSub",
		Source: profile.SubSource{
			Kind: profile.SourceRemote,
			URL:  "https://example.org/sub.yaml",
		},
		Usage: &profile.SubscriptionUsage{
			UsedBytes:  1024,
			TotalBytes: 4096,
			ExpiresAt:  1900000000,
		},
	}}
	if err := config.Save(&cfg); err != nil {
		t.Fatal(err)
	}

	svc := service.NewProfileService(nil)
	s := NewState(svc).ApplySubs(cfg.Subs, "")
	s, _ = s.Update(messages.PageResizeMsg{Width: 100, Height: 24})
	s, _ = s.openEditForm()

	// Tab to Proxy (3) -> toggle On
	for range 3 {
		s, _ = s.handleEditFormUpdate(tea.KeyMsg{Type: tea.KeyTab})
	}
	if !s.editForm.isProxyField() {
		t.Fatalf("expected focus on proxy field, got %d", s.editForm.fieldCursor)
	}
	s, _ = s.handleEditFormUpdate(tea.KeyMsg{Type: tea.KeySpace})

	// Tab to UA (4) -> enter custom UA
	s, _ = s.handleEditFormUpdate(tea.KeyMsg{Type: tea.KeyTab})
	if !s.editForm.isUAField() {
		t.Fatalf("expected focus on UA field, got %d", s.editForm.fieldCursor)
	}
	s.editForm.uaField.SetValue("MyCustomUA/2.0")

	// Tab to SkipTLS (5) -> toggle On
	s, _ = s.handleEditFormUpdate(tea.KeyMsg{Type: tea.KeyTab})
	if !s.editForm.isSkipTLSField() {
		t.Fatalf("expected focus on SkipTLS field, got %d", s.editForm.fieldCursor)
	}
	s, _ = s.handleEditFormUpdate(tea.KeyMsg{Type: tea.KeyRight})

	// Tab to Interval (6) -> select 30m -> 1h
	s, _ = s.handleEditFormUpdate(tea.KeyMsg{Type: tea.KeyTab})
	if !s.editForm.isIntervalField() {
		t.Fatalf("expected focus on Interval field, got %d", s.editForm.fieldCursor)
	}
	s, _ = s.handleEditFormUpdate(tea.KeyMsg{Type: tea.KeyRight})
	s, _ = s.handleEditFormUpdate(tea.KeyMsg{Type: tea.KeyRight})

	// Submit via Enter
	s, cmd := s.handleEditFormUpdate(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || s.showEditForm {
		t.Fatal("expected edit form to submit cleanly")
	}
	if _, ok := cmd().(messages.SubEditDoneMsg); !ok {
		t.Fatal("expected SubEditDoneMsg")
	}

	subs, _, err := svc.ListProfiles()
	if err != nil || len(subs) != 1 {
		t.Fatalf("failed to reload subs: %v", err)
	}
	got := subs[0]
	if !got.Source.UseProxy || got.Source.UserAgent != "MyCustomUA/2.0" || !got.Source.InsecureSkipVerify || got.Source.UpdateInterval != 60 {
		t.Fatalf("fetch options were not persisted: %+v", got.Source)
	}
	if got.Usage == nil || got.Usage.UsedBytes != 1024 || got.Usage.TotalBytes != 4096 || got.Usage.ExpiresAt != 1900000000 {
		t.Fatalf("existing subscription usage was lost when editing fetch options: %+v", got.Usage)
	}

	// Press 'U' to trigger batch update all remote subscriptions
	s = s.ApplySubs(subs, "")
	s, batchCmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("U")})
	if !s.BatchUpdating() || batchCmd == nil {
		t.Fatal("expected U shortcut to start batch remote update")
	}
}
