package core

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/surge-downloader/surge/internal/download"
	"github.com/surge-downloader/surge/internal/engine/state"
	"github.com/surge-downloader/surge/internal/engine/types"
)

// setupTestEnv prepares a temporary directory for config/state isolation
func setupTestEnv(t *testing.T) (string, func()) {
	// Create temp dir
	dir, err := os.MkdirTemp("", "surge-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Set env vars to isolate config
	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	originalHome := os.Getenv("HOME")

	os.Setenv("XDG_CONFIG_HOME", dir)
	os.Setenv("HOME", dir)
	os.Setenv("SURGE_ALLOW_PRIVATE_IPS", "true")

	// Configure state DB path
	dbPath := filepath.Join(dir, "surge.db")

	// Close any existing DB connection first
	state.CloseDB()

	// Configure new DB path
	state.Configure(dbPath)

	cleanup := func() {
		state.CloseDB()
		os.RemoveAll(dir)
		os.Setenv("XDG_CONFIG_HOME", originalXDG)
		os.Setenv("HOME", originalHome)
	}

	return dir, cleanup
}

// createDummyFile creates a file of specific size with random/zero content
func createDummyFile(t *testing.T, path string, size int64) {
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create dummy file: %v", err)
	}
	defer f.Close()

	if err := f.Truncate(size); err != nil {
		t.Fatalf("Failed to truncate file: %v", err)
	}
}

func TestLocalDownloadService_Integration(t *testing.T) {
	// Setup environment
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// create a dummy file to serve
	fileName := "testfile.bin"
	filePath := filepath.Join(tempDir, fileName)
	createDummyFile(t, filePath, 10*1024*1024) // 10MB

	// Start local file server
	ts := httptest.NewServer(http.FileServer(http.Dir(tempDir)))
	defer ts.Close()

	fileURL := ts.URL + "/" + fileName

	// Initialize service
	progressCh := make(chan any, 100)
	pool := download.NewWorkerPool(progressCh, 5)
	service := NewLocalDownloadService(pool)

	// Ensure cleanup of service
	defer service.Shutdown()

	// 1. Add Download
	t.Log("Adding download...")
	id, err := service.Add(fileURL, tempDir, "", nil, nil)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if id == "" {
		t.Fatal("Add returned empty ID")
	}

	// 2. Poll for 'downloading' status
	t.Log("Waiting for download to start...")
	if err := waitForStatus(service, id, "downloading", 5*time.Second); err != nil {
		t.Fatalf("Download failed to start: %v", err)
	}

	// Verify properties
	// Wait for TotalSize to be populated (probe complete)
	timeout := time.After(5 * time.Second)
	var status *types.DownloadStatus

	for {
		s, err := service.GetStatus(id)
		if err != nil {
			t.Fatalf("GetStatus failed: %v", err)
		}
		status = s
		if status.TotalSize > 0 {
			break
		}
		select {
		case <-timeout:
			t.Fatalf("Timeout waiting for TotalSize > 0, got %d", status.TotalSize)
		case <-time.After(100 * time.Millisecond):
			continue
		}
	}

	if status.TotalSize != 10*1024*1024 {
		t.Errorf("Expected size 10MB, got %d", status.TotalSize)
	}

	// 3. Pause
	t.Log("Pausing download...")
	if err := service.Pause(id); err != nil {
		t.Fatalf("Pause failed: %v", err)
	}

	// Poll for 'paused' status
	// Note: It might transition through 'pausing' first
	if err := waitForStatus(service, id, "paused", 5*time.Second); err != nil {
		t.Fatalf("Download failed to pause: %v", err)
	}

	// 4. Resume
	t.Log("Resuming download...")
	if err := service.Resume(id); err != nil {
		t.Fatalf("Resume failed: %v", err)
	}

	// Poll for 'downloading' or 'completed' status
	// Resume might be fast enough to complete immediately depending on chunk size and network speed (loopback is fast)
	if err := waitForStatusAny(service, id, []string{"downloading", "completed"}, 5*time.Second); err != nil {
		t.Fatalf("Download failed to resume: %v", err)
	}

	// 5. Delete
	t.Log("Deleting download...")
	if err := service.Delete(id); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify removed from active list
	list, err := service.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	for _, item := range list {
		if item.ID == id && item.Status != "completed" {
			// Delete usually removes it, but if it completed before delete, it might be in history
			// But Delete calls RemoveFromMasterList, so it should be gone from history too.
			t.Errorf("Download %s still exists in list with status %s", id, item.Status)
		}
	}
}

// waitForStatus polls until status matches target or timeout
func waitForStatus(s *LocalDownloadService, id string, target string, timeout time.Duration) error {
	return waitForStatusAny(s, id, []string{target}, timeout)
}

func waitForStatusAny(s *LocalDownloadService, id string, targets []string, timeout time.Duration) error {
	start := time.Now()
	for time.Since(start) < timeout {
		status, err := s.GetStatus(id)
		if err == nil {
			for _, t := range targets {
				if status.Status == t {
					return nil
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Timeout, get final status for error message
	status, err := s.GetStatus(id)
	current := "unknown"
	if err == nil {
		current = status.Status
	}
	return fmt.Errorf("timeout waiting for status %v, current: %s", targets, current)
}
