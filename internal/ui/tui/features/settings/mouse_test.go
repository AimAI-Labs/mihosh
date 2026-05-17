package settings

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
)

func TestHandleMouseLeft_SingleClickSelectsSetting(t *testing.T) {
	state := State{}
	cfg := &config.Config{
		APIAddress:   "http://127.0.0.1:9090",
		Secret:       "abc",
		TestURL:      "http://example.com",
		Timeout:      5000,
		ProxyAddress: "http://127.0.0.1:7890",
	}

	next := state.HandleMouseLeft(5, cfg)
	if next.selectedSetting != 1 {
		t.Fatalf("expected selectedSetting=1, got %d", next.selectedSetting)
	}
	if next.editMode {
		t.Fatalf("expected editMode=false on single click")
	}
}

func TestHandleMouseLeft_DoubleClickEntersEditMode(t *testing.T) {
	state := State{}
	cfg := &config.Config{
		Timeout: 7000,
	}

	const timeoutRowY = 7 // timeout index=3, offset=4
	next := state.HandleMouseLeft(timeoutRowY, cfg)
	if next.editMode {
		t.Fatalf("expected editMode=false on first click")
	}

	next = next.HandleMouseLeft(timeoutRowY, cfg)
	if !next.editMode {
		t.Fatalf("expected editMode=true after double click")
	}
	if next.editValue != "7000" {
		t.Fatalf("expected editValue=7000, got %q", next.editValue)
	}
	if next.editCursor != 4 {
		t.Fatalf("expected editCursor=4, got %d", next.editCursor)
	}
}
