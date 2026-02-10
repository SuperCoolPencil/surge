package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/surge-downloader/surge/internal/config"
	"github.com/surge-downloader/surge/internal/core"
	"github.com/surge-downloader/surge/internal/download"
)

func TestHandleDownload_Vulnerability_Repro(t *testing.T) {
	// Setup temporary directory for mocking XDG_CONFIG_HOME
	tempDir, err := os.MkdirTemp("", "surge-test-home")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create surge config directory
	surgeConfigDir := filepath.Join(tempDir, "surge")
	if err := os.MkdirAll(surgeConfigDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Setup default download directory
	defaultDownloadDir := filepath.Join(tempDir, "Downloads")
	if err := os.MkdirAll(defaultDownloadDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Setup sensitive directory (to be written to)
	sensitiveDir := filepath.Join(tempDir, "sensitive")
	if err := os.MkdirAll(sensitiveDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a temporary settings file
	settings := config.DefaultSettings()
	settings.General.DefaultDownloadDir = defaultDownloadDir

	if err := config.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}

	// Initialize GlobalPool (required by handleDownload)
	GlobalPool = download.NewWorkerPool(nil, 1)

	// Vulnerability: Attempt to download to sensitiveDir using absolute path
	// This should be BLOCKED but currently SUCCEEDS.
	targetPath := sensitiveDir // Directory, because req.Path is directory

	request := DownloadRequest{
		URL:                  "http://example.com/sensitive_file",
		Path:                 targetPath,
		RelativeToDefaultDir: false, // Don't use default dir prefix
		Filename:             "exploit.txt",
	}

	body, _ := json.Marshal(request)
	req := httptest.NewRequest("POST", "/download", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	svc := core.NewLocalDownloadService(GlobalPool)

	// Call handleDownload
	handleDownload(w, req, defaultDownloadDir, svc)

	// Current behavior (SECURE):
	// It should fail with 403 Forbidden because targetPath is outside defaultDownloadDir.

	if w.Code == http.StatusForbidden {
		t.Logf("Request correctly blocked with code %d", w.Code)
	} else {
		t.Errorf("Request should have been blocked with 403, got %d: %s", w.Code, w.Body.String())
	}

	// Check that it was NOT queued
	configs := GlobalPool.GetAll()
	for _, cfg := range configs {
		if cfg.URL == request.URL {
			t.Errorf("Download should NOT be queued, but found in pool with path: %s", cfg.OutputPath)
		}
	}
}
