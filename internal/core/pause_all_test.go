package core

import (
	"testing"

	"github.com/surge-downloader/surge/internal/download"
)

func TestLocalDownloadService_PauseAll(t *testing.T) {
	// Setup
	progressCh := make(chan any, 100)
	pool := download.NewWorkerPool(progressCh, 5)
	service := NewLocalDownloadService(pool)

	// Test 1: No downloads
	if err := service.PauseAll(); err != nil {
		t.Errorf("PauseAll failed with no downloads: %v", err)
	}
}
