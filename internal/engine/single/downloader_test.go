package single

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/surge-downloader/surge/internal/engine/types"
	"github.com/surge-downloader/surge/internal/testutil"
)

func init() {
	// Allow private IPs for testing
	_ = os.Setenv("SURGE_ALLOW_PRIVATE_IPS", "true")
}

func TestNewSingleDownloader(t *testing.T) {
	dl := NewSingleDownloader("test-id", nil, nil, types.DefaultRuntimeConfig())
	if dl.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got %s", dl.ID)
	}
}

func TestSingleDownloader_Download_Success(t *testing.T) {
	// Setup mock server
	server := testutil.NewMockServerT(t, testutil.WithFileSize(1024), testutil.WithRandomData(true))
	defer server.Close()

	// Setup temp output file
	tmpDir, cleanup, err := testutil.TempDir("single-dl-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer cleanup()

	outputPath := filepath.Join(tmpDir, "output.bin")

	// Setup downloader
	progressCh := make(chan any, 10)
	state := types.NewProgressState("test-id", 1024)
	dl := NewSingleDownloader("test-id", progressCh, state, types.DefaultRuntimeConfig())

	// Run download
	err = dl.Download(context.Background(), server.URL(), outputPath, 1024, "output.bin", false)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	// Verify file size
	if err := testutil.VerifyFileSize(outputPath, 1024); err != nil {
		t.Errorf("File verification failed: %v", err)
	}

	// Note: dl.Download does NOT set state.Done. The WorkerPool does.
	if state.Downloaded.Load() != 1024 {
		t.Errorf("Expected 1024 downloaded bytes, got %d", state.Downloaded.Load())
	}
}

func TestSingleDownloader_Download_Cancellation(t *testing.T) {
	// Setup slow mock server
	server := testutil.NewMockServerT(t,
		testutil.WithFileSize(1024*1024),
		testutil.WithByteLatency(10*time.Microsecond), // Ensure it takes some time
	)
	defer server.Close()

	tmpDir, cleanup, err := testutil.TempDir("single-dl-cancel")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer cleanup()
	outputPath := filepath.Join(tmpDir, "cancel.bin")

	progressCh := make(chan any, 10)
	state := types.NewProgressState("test-id", 1024*1024)
	dl := NewSingleDownloader("test-id", progressCh, state, types.DefaultRuntimeConfig())

	ctx, cancel := context.WithCancel(context.Background())

	// Start download in goroutine
	errCh := make(chan error)
	go func() {
		errCh <- dl.Download(ctx, server.URL(), outputPath, 1024*1024, "cancel.bin", false)
	}()

	// Wait a bit then cancel
	time.Sleep(10 * time.Millisecond)
	cancel()

	// Check result
	err = <-errCh
	// Accepts nil (completed before cancel) or context.Canceled (wrapped or direct)
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled or nil, got: %v", err)
	}
}

func TestSingleDownloader_Download_ProgressTracking(t *testing.T) {
	// Setup server
	size := int64(100 * 1024)
	server := testutil.NewMockServerT(t, testutil.WithFileSize(size))
	defer server.Close()

	tmpDir, cleanup, err := testutil.TempDir("single-dl-progress")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer cleanup()
	outputPath := filepath.Join(tmpDir, "progress.bin")

	progressCh := make(chan any, 100)
	state := types.NewProgressState("test-id", size)
	dl := NewSingleDownloader("test-id", progressCh, state, types.DefaultRuntimeConfig())

	err = dl.Download(context.Background(), server.URL(), outputPath, size, "progress.bin", false)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	// Verify state updates
	if state.Downloaded.Load() != size {
		t.Errorf("Expected %d bytes downloaded, got %d", size, state.Downloaded.Load())
	}
}

func TestSingleDownloader_Download_ServerError(t *testing.T) {
	// Setup failing server
	server := testutil.NewMockServerT(t, testutil.WithFailOnNthRequest(1))
	defer server.Close()

	tmpDir, cleanup, err := testutil.TempDir("single-dl-error")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer cleanup()
	outputPath := filepath.Join(tmpDir, "error.bin")

	dl := NewSingleDownloader("test-id", nil, nil, types.DefaultRuntimeConfig())

	// Should fail immediately
	err = dl.Download(context.Background(), server.URL(), outputPath, 1024, "error.bin", false)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

func TestSingleDownloader_Download_WithLatency(t *testing.T) {
	// Setup slow server
	server := testutil.NewMockServerT(t,
		testutil.WithFileSize(1024),
		testutil.WithLatency(2*time.Millisecond),
	)
	defer server.Close()

	tmpDir, cleanup, err := testutil.TempDir("single-dl-latency")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer cleanup()
	outputPath := filepath.Join(tmpDir, "latency.bin")

	dl := NewSingleDownloader("test-id", nil, nil, types.DefaultRuntimeConfig())

	start := time.Now()
	err = dl.Download(context.Background(), server.URL(), outputPath, 1024, "latency.bin", false)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}
	duration := time.Since(start)

	if duration < 2*time.Millisecond {
		t.Errorf("Download too fast, expected at least 2ms latency, took %v", duration)
	}
}

func TestSingleDownloader_Download_ContentIntegrity(t *testing.T) {
	// Setup server with random data
	size := int64(64 * 1024)
	server := testutil.NewMockServerT(t, testutil.WithFileSize(size), testutil.WithRandomData(true))
	defer server.Close()

	// Download raw content to compare
	resp, err := http.Get(server.URL())
	if err != nil {
		t.Fatalf("Failed to fetch reference content: %v", err)
	}
	defer resp.Body.Close()
	expectedData, _ := io.ReadAll(resp.Body)

	tmpDir, cleanup, err := testutil.TempDir("single-dl-integrity")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer cleanup()
	outputPath := filepath.Join(tmpDir, "integrity.bin")

	dl := NewSingleDownloader("test-id", nil, nil, types.DefaultRuntimeConfig())

	err = dl.Download(context.Background(), server.URL(), outputPath, size, "integrity.bin", false)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	// Compare file content
	actualData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if len(actualData) != len(expectedData) {
		t.Errorf("Size mismatch: expected %d, got %d", len(expectedData), len(actualData))
	} else {
		for i := range actualData {
			if actualData[i] != expectedData[i] {
				t.Errorf("Content mismatch at byte %d", i)
				break
			}
		}
	}
}

func TestSingleDownloader_StreamingServer(t *testing.T) {
	// Simulate very large file stream (10MB)
	size := int64(10 * 1024 * 1024)
	server := testutil.NewStreamingMockServerT(t, size)
	defer server.Close()

	tmpDir, cleanup, err := testutil.TempDir("single-dl-stream")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer cleanup()
	outputPath := filepath.Join(tmpDir, "stream.bin")

	dl := NewSingleDownloader("test-id", nil, nil, types.DefaultRuntimeConfig())

	// We don't need to download the whole thing, just start it
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = dl.Download(ctx, server.URL(), outputPath, size, "stream.bin", false)
	// It should fail with timeout/cancel, but crucially NOT with protocol error
	if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("Streaming download failed: %v", err)
	}
}

func TestSingleDownloader_FailAfterBytes(t *testing.T) {
	// Server fails connection after 50KB
	failAt := int64(50 * 1024)
	server := testutil.NewMockServerT(t,
		testutil.WithFileSize(100*1024),
		testutil.WithFailAfterBytes(failAt),
	)
	defer server.Close()

	tmpDir, cleanup, err := testutil.TempDir("single-dl-fail")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer cleanup()
	outputPath := filepath.Join(tmpDir, "fail.bin")

	dl := NewSingleDownloader("test-id", nil, nil, types.DefaultRuntimeConfig())

	// Should return error
	err = dl.Download(context.Background(), server.URL(), outputPath, 100*1024, "fail.bin", false)
	if err == nil {
		t.Fatal("Expected error due to connection drop, got nil")
	}

	// Verify partial file
	info, statErr := os.Stat(outputPath + types.IncompleteSuffix)
	if statErr == nil {
		if info.Size() == 0 {
			t.Errorf("Expected at least 50KB served before failure, got 0")
		}
	}
}

func TestSingleDownloader_NilState(t *testing.T) {
	// Ensure it doesn't panic with nil state
	server := testutil.NewMockServerT(t, testutil.WithFileSize(1024))
	defer server.Close()

	tmpDir, cleanup, err := testutil.TempDir("single-dl-nil")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer cleanup()
	outputPath := filepath.Join(tmpDir, "nil.bin")

	dl := NewSingleDownloader("test-id", nil, nil, types.DefaultRuntimeConfig())

	err = dl.Download(context.Background(), server.URL(), outputPath, 1024, "nil.bin", false)
	if err != nil {
		t.Fatalf("Download with nil state failed: %v", err)
	}
}
