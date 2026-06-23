package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadWriteMihomoYAML(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Test 1: Write to non-existent file
	err := WriteMihomoField(configPath, "allow-lan", true)
	if err != nil {
		t.Fatalf("Failed to write to new file: %v", err)
	}
	res, err := ReadMihomoYAML(configPath)
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}
	if res["allow-lan"] != true {
		t.Errorf("Expected allow-lan=true, got %v", res["allow-lan"])
	}

	// Test 2: Write different type
	err = WriteMihomoField(configPath, "mixed-port", 7890)
	if err != nil {
		t.Fatalf("Failed to write int: %v", err)
	}

	// Test 3: Write float type (generic type test)
	err = WriteMihomoField(configPath, "test-float", 1.5)
	if err != nil {
		t.Fatalf("Failed to write float: %v", err)
	}

	// Test 4: Preserve comments
	initialYAML := []byte("# Comment\nexternal-controller: '127.0.0.1:9090'\nallow-lan: false\n")
	if err := os.WriteFile(configPath, initialYAML, 0644); err != nil {
		t.Fatalf("Failed to setup comment test: %v", err)
	}
	if err := WriteMihomoField(configPath, "allow-lan", true); err != nil {
		t.Fatalf("Failed to write existing field: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read back: %v", err)
	}
	if !strings.Contains(string(data), "# Comment") {
		t.Errorf("Comments were not preserved")
	}
	if !strings.Contains(string(data), "allow-lan: true") {
		t.Errorf("Value was not updated")
	}
}
