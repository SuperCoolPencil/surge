package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/surge-downloader/surge/internal/config"
	"github.com/surge-downloader/surge/internal/core"
	"github.com/surge-downloader/surge/internal/download"
	"github.com/surge-downloader/surge/internal/engine/types"
)

func TestHandleDownload_DuplicateDetection(t *testing.T) {
	// Setup temporary directory for mocking XDG_CONFIG_HOME and settings
	tempDir, err := os.MkdirTemp("", "surge-test-duplicate")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Mock XDG_CONFIG_HOME
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	// Ensure surge directories exist (important for config loading)
	surgeDir := filepath.Join(tempDir, "surge")
	if err := os.MkdirAll(surgeDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create settings with WarnOnDuplicate = true
	settings := config.DefaultSettings()
	settings.General.WarnOnDuplicate = true
	settings.General.ExtensionPrompt = false // ensure this doesn't interfere
	if err := config.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}

	// Initialize GlobalPool
	// We need to make sure GlobalPool is reset after test to avoid polluting other tests
	oldGlobalPool := GlobalPool
	defer func() { GlobalPool = oldGlobalPool }()

	GlobalPool = download.NewWorkerPool(nil, 1)

	// Channel to signal server to stop hanging
	stopServer := make(chan struct{})

	// Start a test server that hangs to keep the download active in the pool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-stopServer
	}))
	defer ts.Close() // This will block until handler returns, so we must close stopServer first
	defer close(stopServer) // This runs before ts.Close()

	duplicateURL := ts.URL

	// Add a dummy active download to the pool
	// Initialize State to avoid panic in TUIDownload when worker processes it
	dummyState := types.NewProgressState("existing-id", 0)
	GlobalPool.Add(types.DownloadConfig{
		ID:       "existing-id",
		URL:      duplicateURL,
		Filename: "duplicate.zip",
		State:    dummyState,
	})

	// Wait for the download to become active (move from queued to downloads)
	active := false
	for i := 0; i < 50; i++ {
		if GlobalPool.HasDownload(duplicateURL) {
			active = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !active {
		t.Fatalf("Download %s failed to become active in time", duplicateURL)
	}

	// Test Case 1: Duplicate Download in Headless Mode (serverProgram == nil)
	reqBody := DownloadRequest{
		URL: duplicateURL,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/download", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	// Initialize Service (Local)
	svc := core.NewLocalDownloadServiceWithInput(GlobalPool, nil)

	// Call handleDownload
	handleDownload(w, req, tempDir, svc)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected StatusConflict (409), got %d. Body: %s", w.Code, w.Body.String())
	}

    var resp map[string]string
    if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
        t.Errorf("Failed to parse JSON response: %v", err)
    }

    if resp["status"] != "error" {
        t.Errorf("Expected status 'error', got '%s'", resp["status"])
    }

	// Test Case 2: New Download (Not Duplicate)
    newURL := "http://example.com/new.zip"
    reqBodyNew := DownloadRequest{
		URL: newURL,
	}
	bodyNew, _ := json.Marshal(reqBodyNew)
	reqNew := httptest.NewRequest("POST", "/download", bytes.NewBuffer(bodyNew))
	wNew := httptest.NewRecorder()

	handleDownload(wNew, reqNew, tempDir, svc)

    if wNew.Code != http.StatusOK {
		t.Errorf("Expected StatusOK (200), got %d. Body: %s", wNew.Code, wNew.Body.String())
	}

    if err := json.Unmarshal(wNew.Body.Bytes(), &resp); err != nil {
        t.Errorf("Failed to parse JSON response: %v", err)
    }

    if resp["status"] != "queued" {
        t.Errorf("Expected status 'queued', got '%s'", resp["status"])
    }
}
