package download

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/surge-downloader/surge/internal/engine/state"
)

func TestMain(m *testing.M) {
	// Enable private IP access for tests
	os.Setenv("SURGE_ALLOW_PRIVATE_IPS", "true")

	// Setup temporary directory for state DB
	tmpDir, err := os.MkdirTemp("", "surge-download-test-state")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	// Configure state DB
	dbPath := filepath.Join(tmpDir, "surge.db")
	state.Configure(dbPath)

	os.Exit(m.Run())
}
