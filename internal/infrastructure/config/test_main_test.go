package config

import (
	"os"
	"testing"
)

// Ignore a developer service root; individual tests may explicitly set an isolated root.
func TestMain(m *testing.M) { _ = os.Unsetenv("MIHOSH_CONFIG_DIR"); os.Exit(m.Run()) }
