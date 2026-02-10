package core

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
	"os"

	"github.com/surge-downloader/surge/internal/download"
	"github.com/surge-downloader/surge/internal/engine/state"
	"github.com/surge-downloader/surge/internal/testutil"
)

func TestLocalDownloadService_PauseAll(t *testing.T) {
	// Setup State DB
	tmpDir := t.TempDir()
	t.Logf("Using temp dir: %s", tmpDir)
	state.Configure(filepath.Join(tmpDir, "surge.db"))

	// Setup Mock Server
	server := testutil.NewMockServerT(t, testutil.WithFileSize(10*1024*1024), testutil.WithLatency(10*time.Millisecond)) // 10MB file, slow
	defer server.Close()

	// Setup Service
	progressCh := make(chan any, 100)
	// Create a pool with 2 workers so we can test active + queued
	pool := download.NewWorkerPool(progressCh, 2)
	service := NewLocalDownloadService(pool)
	defer func() { _ = service.Shutdown() }()

	// Test 1: No downloads
	if err := service.PauseAll(); err != nil {
		t.Errorf("PauseAll failed with no downloads: %v", err)
	}

	// Test 2: Active and Queued downloads
	var ids []string
	for i := 0; i < 4; i++ {
		distinctURL := fmt.Sprintf("%s?id=%d", server.URL(), i)
		// Use tmpDir for downloads
		id, err := service.Add(distinctURL, tmpDir, fmt.Sprintf("file%d.bin", i), nil, nil)
		if err != nil {
			t.Fatalf("Failed to add download: %v", err)
		}
		ids = append(ids, id)
	}

	// Wait for 2 downloads to start
	timeout := time.After(5 * time.Second)
	started := 0

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// Wait until we have 2 downloading and 2 queued
	loop:
	for {
		select {
		case <-timeout:
			t.Fatalf("Timed out waiting for downloads to start")
		case <-ticker.C:
			downloading := 0
			queued := 0
			for _, id := range ids {
				status, err := service.GetStatus(id)
				if err == nil && status != nil {
					if status.Status == "downloading" {
						downloading++
					} else if status.Status == "queued" {
						queued++
					}
				}
			}
			if downloading == 2 && queued == 2 {
				started = downloading
				break loop
			}
		}
	}

	t.Logf("Downloads started: %d active, %d queued", started, 4-started)

	// Pause All
	if err := service.PauseAll(); err != nil {
		t.Errorf("PauseAll failed: %v", err)
	}

	// Verify all become paused
	timeout = time.After(5 * time.Second)

	for {
		select {
		case <-timeout:
			// Print statuses for debugging
			for _, id := range ids {
				status, err := service.GetStatus(id)
				s := "error"
				if err == nil && status != nil {
					s = status.Status
				} else if err != nil {
					s = err.Error()
				}
				t.Logf("ID %s: %s", id, s)
			}
			t.Fatalf("Timed out waiting for all downloads to pause")
		case <-ticker.C:
			paused := 0
			for _, id := range ids {
				status, err := service.GetStatus(id)
				if err == nil && status != nil && status.Status == "paused" {
					paused++
				}
			}
			if paused == 4 {
				t.Logf("All 4 downloads paused successfully")

				// Cleanup check: check if files are in tmpDir
				files, _ := os.ReadDir(tmpDir)
				t.Logf("Files in tmpDir: %d", len(files))
				for _, f := range files {
					t.Logf("  %s", f.Name())
				}

				return
			}
		}
	}
}
