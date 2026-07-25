package tui

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
)

func TestModelShowAndHideGuideModal(t *testing.T) {
	m := NewModel(nil, "http://www.gstatic.com/generate_204", 5000)
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
