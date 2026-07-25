package guide

import (
	"testing"
)

func TestGuideStateActivateAndNavigation(t *testing.T) {
	var s State
	if s.Active {
		t.Errorf("initial state should be inactive")
	}

	s = s.Activate("http://127.0.0.1:9090", "******", "connection refused")
	if !s.Active {
		t.Errorf("state should be active after Activate")
	}
	if s.Endpoint != "http://127.0.0.1:9090" {
		t.Errorf("unexpected endpoint: %s", s.Endpoint)
	}
	if s.FocusedButton != 0 {
		t.Errorf("default focused button should be 0")
	}

	s = s.NextButton()
	if s.FocusedButton != 1 {
		t.Errorf("expected focused button 1, got %d", s.FocusedButton)
	}

	s = s.NextButton()
	if s.FocusedButton != 2 {
		t.Errorf("expected focused button 2, got %d", s.FocusedButton)
	}

	s = s.NextButton()
	if s.FocusedButton != 0 {
		t.Errorf("expected wrap around to 0, got %d", s.FocusedButton)
	}

	s = s.PrevButton()
	if s.FocusedButton != 2 {
		t.Errorf("expected prev wrap around to 2, got %d", s.FocusedButton)
	}

	s = s.Dismiss()
	if s.Active {
		t.Errorf("state should be inactive after Dismiss")
	}
}
