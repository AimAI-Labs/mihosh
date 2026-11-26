package tui

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/bootstrap"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
)

func newTestModel(client *service.Client, testURL string, timeout int) Model {
	app := bootstrap.NewAppWithClient(client)
	app.Config.TestURL, app.Config.Timeout = testURL, timeout
	return NewModel(app)
}

func TestModelShowAndHideGuideModal(t *testing.T) {
	m := newTestModel(nil, "http://www.gstatic.com/generate_204", 5000)
	if m.guideState.Active {
		t.Errorf("expected initial guideState.Active to be false")
	}

	// Dispatch ShowGuideModalMsg
	newModel, _ := m.Update(messages.ShowGuideModalMsg{
		Endpoint:     "http://127.0.0.1:9090",
		SecretMasked: "******",
		ErrMessage:   "connection refused",
	})
	m = newModel.(Model)

	if !m.guideState.Active {
		t.Errorf("expected guideState.Active to be true after ShowGuideModalMsg")
	}
	if m.guideState.Endpoint != "http://127.0.0.1:9090" {
		t.Errorf("expected endpoint http://127.0.0.1:9090, got %s", m.guideState.Endpoint)
	}

	// Dispatch HideGuideModalMsg
	newModel, _ = m.Update(messages.HideGuideModalMsg{})
	m = newModel.(Model)

	if m.guideState.Active {
		t.Errorf("expected guideState.Active to be false after HideGuideModalMsg")
	}
}
