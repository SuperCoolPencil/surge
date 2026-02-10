package cmd

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/surge-downloader/surge/internal/config"
	"github.com/surge-downloader/surge/internal/core"
	"github.com/surge-downloader/surge/internal/download"
	"github.com/surge-downloader/surge/internal/utils"
)

// TestSecurity_Fix_SensitiveDataInLogs ensures that sensitive data (like tokens in URLs) is not logged.
func TestSecurity_Fix_SensitiveDataInLogs(t *testing.T) {
	// Create a temp directory for logs
	tmpDir, err := os.MkdirTemp("", "surge-security-test-logs")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Configure debug to use this directory
	originalDir := config.GetLogsDir()
	utils.ConfigureDebug(tmpDir)
	defer utils.ConfigureDebug(originalDir)

	// Initialize GlobalPool if needed
	GlobalPool = download.NewWorkerPool(nil, 1)

	sensitiveURL := "http://example.com/secret?token=SENSITIVE_DATA"
	reqBody := DownloadRequest{
		URL:  sensitiveURL,
		Path: "test",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/download", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	svc := core.NewLocalDownloadService(GlobalPool)

	// Call handleDownload
	handleDownload(w, req, ".", svc)

	// Check logs
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	foundSensitiveData := false
	logFilesCount := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "debug-") {
			logFilesCount++
			content, err := os.ReadFile(filepath.Join(tmpDir, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(content), "SENSITIVE_DATA") {
				foundSensitiveData = true
				break
			}
		}
	}

	if logFilesCount == 0 {
		t.Fatal("No log files created, ConfigureDebug failed to redirect logs.")
	}

	if foundSensitiveData {
		t.Log("Regression Test Failed: Sensitive data found in logs.")
		t.Fail()
	} else {
		t.Log("Sensitive data NOT found in logs.")
	}
}
