package concurrent

import (
	"testing"

	"github.com/surge-downloader/surge/internal/engine/types"
)

// TestSequentialDownload_RespectsConcurrency verifies that when SequentialDownload is true,
// we limit the number of connections to 1 to ensure strict sequential behavior.
func TestSequentialDownload_RespectsConcurrency(t *testing.T) {
	// Setup RuntimeConfig with SequentialDownload = true
	runtime := &types.RuntimeConfig{
		SequentialDownload:    true,
		MaxConnectionsPerHost: 8,
		MinChunkSize:          1024 * 1024, // 1MB
	}

	d := &ConcurrentDownloader{Runtime: runtime}

	// Large file size that would normally trigger multiple connections
	// With 100MB and MinChunk 1MB, it would normally spawn max connections (8)
	fileSize := int64(100 * 1024 * 1024) // 100MB

	// Check initial connections
	conns := d.getInitialConnections(fileSize)

	// We expect 1 connection for strict sequential download
	if conns != 1 {
		t.Errorf("Expected 1 connection for SequentialDownload, got %d", conns)
	}

	// Verify that if SequentialDownload is FALSE, we get multiple connections
	runtime.SequentialDownload = false
	connsParallel := d.getInitialConnections(fileSize)
	if connsParallel <= 1 {
		t.Errorf("Expected > 1 connection for Parallel download, got %d", connsParallel)
	}
}

// TestSequentialDownload_RespectsChunkSize verifies that chunk size logic
// still respects the MinChunkSize for sequential downloads.
func TestSequentialDownload_RespectsChunkSize(t *testing.T) {
	minChunk := int64(2 * 1024 * 1024) // 2MB
	runtime := &types.RuntimeConfig{
		SequentialDownload: true,
		MinChunkSize:       minChunk,
	}

	d := &ConcurrentDownloader{Runtime: runtime}
	fileSize := int64(100 * 1024 * 1024)

	// Determine chunk size (numConns is ignored in sequential logic but passed anyway)
	chunkSize := d.determineChunkSize(fileSize, 8)

	// Should match minChunk (aligned)
	if chunkSize != minChunk {
		t.Errorf("Expected chunk size %d, got %d", minChunk, chunkSize)
	}
}
