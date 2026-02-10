package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/surge-downloader/surge/internal/engine/types"
)

func BenchmarkSaveState(b *testing.B) {
	// Setup DB
	tempDir, err := os.MkdirTemp("", "surge-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "surge.db")

	// Ensure clean state
	dbMu.Lock()
	if db != nil {
		_ = db.Close()
		db = nil
	}
	configured = false
	dbMu.Unlock()

	Configure(dbPath)
	if err := initDB(); err != nil {
		b.Fatalf("Failed to init DB: %v", err)
	}
	defer CloseDB()

	// Prepare data
	url := "https://example.com/bench.zip"
	dest := filepath.Join(tempDir, "bench.zip")
	id := uuid.New().String()

	// Create a state with many tasks
	numTasks := 1000
	tasks := make([]types.Task, numTasks)
	for i := 0; i < numTasks; i++ {
		tasks[i] = types.Task{
			Offset: int64(i * 1024),
			Length: 1024,
		}
	}

	state := &types.DownloadState{
		ID:         id,
		URL:        url,
		DestPath:   dest,
		TotalSize:  int64(numTasks * 1024),
		Downloaded: 0,
		Tasks:      tasks,
		Filename:   "bench.zip",
	}

	// Initial save
	if err := SaveState(url, dest, state); err != nil {
		b.Fatalf("Initial SaveState failed: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Simulate slight change: remove one task, add one task?
		// Or just save same state (common case where only metadata might change or few tasks change)
		// Let's modify one task to simulate progress
		if len(state.Tasks) > 0 {
			state.Tasks[0].Length += 1
		}

		if err := SaveState(url, dest, state); err != nil {
			b.Fatalf("SaveState failed: %v", err)
		}

		// Restore
		if len(state.Tasks) > 0 {
			state.Tasks[0].Length -= 1
		}
	}
}
