package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/surge-downloader/surge/internal/config"
)

func TestReadActivePort(t *testing.T) {
	// Setup temp environment
	tempDir := t.TempDir()

	// Set both HOME and XDG_CONFIG_HOME to cover different OS paths in config.GetSurgeDir
	// We set APPDATA too for Windows.
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("HOME", tempDir)
	t.Setenv("APPDATA", tempDir)

	// In the real code config.GetSurgeDir might rely on these env vars.
	// Let's verify what config.GetSurgeDir() returns to be sure.
	surgeDir := config.GetSurgeDir()

	// Create the directory if it doesn't exist (simulating app startup or config.EnsureDirs)
	if err := os.MkdirAll(surgeDir, 0755); err != nil {
		t.Fatalf("Failed to create surge dir: %v", err)
	}

	portFile := filepath.Join(surgeDir, "port")

	// Case 1: No port file
	// Ensure file doesn't exist
	_ = os.Remove(portFile)
	if port := readActivePort(); port != 0 {
		t.Errorf("Case 1: Expected 0 when port file missing, got %d", port)
	}

	// Case 2: valid port file
	expectedPort := 12345
	if err := os.WriteFile(portFile, []byte(fmt.Sprintf("%d", expectedPort)), 0644); err != nil {
		t.Fatalf("Failed to write port file: %v", err)
	}

	if port := readActivePort(); port != expectedPort {
		t.Errorf("Case 2: Expected %d, got %d", expectedPort, port)
	}

	// Case 3: invalid port file (garbage)
	if err := os.WriteFile(portFile, []byte("garbage"), 0644); err != nil {
		t.Fatalf("Failed to write port file: %v", err)
	}

	if port := readActivePort(); port != 0 {
		t.Errorf("Case 3: Expected 0 when port file is garbage, got %d", port)
	}

	// Case 4: empty port file
	if err := os.WriteFile(portFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to write port file: %v", err)
	}

	if port := readActivePort(); port != 0 {
		t.Errorf("Case 4: Expected 0 when port file is empty, got %d", port)
	}
}
