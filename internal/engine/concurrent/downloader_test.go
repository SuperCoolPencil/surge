package concurrent

import (
	"context"
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

func TestConcurrentDownloader_Download_Success(t *testing.T) {
	tmpDir, cleanup := initTestState(t)
	defer cleanup()

	// Setup mock server
	size := int64(5 * 1024 * 1024) // 5MB
	server := testutil.NewMockServerT(t, testutil.WithFileSize(size), testutil.WithRandomData(true))
	defer server.Close()

	outputPath := filepath.Join(tmpDir, "output.bin")

	// Downloader
	progressCh := make(chan any, 100)
	state := types.NewProgressState("test-id", size)
	dl := NewConcurrentDownloader("test-id", progressCh, state, types.DefaultRuntimeConfig())

	// Run
	err := dl.Download(context.Background(), server.URL(), nil, nil, outputPath, size, false)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	// Verify
	if err := testutil.VerifyFileSize(outputPath, size); err != nil {
		t.Errorf("File verification failed: %v", err)
	}
	// Note: dl.Download does NOT set state.Done. The WorkerPool does.
}

func TestConcurrentDownloader_Download_SmallFile(t *testing.T) {
	tmpDir, cleanup := initTestState(t)
	defer cleanup()

	// Should fallback to single connection or just handle it
	size := int64(100 * 1024) // 100KB
	server := testutil.NewMockServerT(t, testutil.WithFileSize(size))
	defer server.Close()

	outputPath := filepath.Join(tmpDir, "small.bin")

	state := types.NewProgressState("test-id", size)
	dl := NewConcurrentDownloader("test-id", nil, state, types.DefaultRuntimeConfig())
	err := dl.Download(context.Background(), server.URL(), nil, nil, outputPath, size, false)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	if err := testutil.VerifyFileSize(outputPath, size); err != nil {
		t.Errorf("File verification failed: %v", err)
	}
}

func TestConcurrentDownloader_Download_Cancellation(t *testing.T) {
	tmpDir, cleanup := initTestState(t)
	defer cleanup()

	// Large file with latency to ensure we can cancel in middle
	size := int64(10 * 1024 * 1024)
	server := testutil.NewMockServerT(t, testutil.WithFileSize(size), testutil.WithLatency(10*time.Millisecond))
	defer server.Close()

	outputPath := filepath.Join(tmpDir, "cancel.bin")

	ctx, cancel := context.WithCancel(context.Background())
	state := types.NewProgressState("test-id", size)
	dl := NewConcurrentDownloader("test-id", nil, state, types.DefaultRuntimeConfig())

	errCh := make(chan error)
	go func() {
		errCh <- dl.Download(ctx, server.URL(), nil, nil, outputPath, size, false)
	}()

	time.Sleep(200 * time.Millisecond) // Wait for start
	cancel()

	err := <-errCh
	// Download returns nil on clean cancellation
	if err != nil && err != context.Canceled {
		t.Errorf("Expected nil or context.Canceled, got: %v", err)
	}
}

func TestConcurrentDownloader_Download_ProgressTracking(t *testing.T) {
	tmpDir, cleanup := initTestState(t)
	defer cleanup()

	size := int64(2 * 1024 * 1024)
	server := testutil.NewMockServerT(t, testutil.WithFileSize(size))
	defer server.Close()

	outputPath := filepath.Join(tmpDir, "progress.bin")

	progressCh := make(chan any, 100)
	state := types.NewProgressState("test-id", size)
	dl := NewConcurrentDownloader("test-id", progressCh, state, types.DefaultRuntimeConfig())

	err := dl.Download(context.Background(), server.URL(), nil, nil, outputPath, size, false)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	if state.Downloaded.Load() != size {
		t.Errorf("Expected %d bytes downloaded, got %d", size, state.Downloaded.Load())
	}
}

func TestConcurrentDownloader_Download_ServerError(t *testing.T) {
	tmpDir, cleanup := initTestState(t)
	defer cleanup()

	// Server fails ALWAYS
	server := testutil.NewMockServerT(t, testutil.WithHandler(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "simulated error", http.StatusInternalServerError)
	}))
	defer server.Close()

	outputPath := filepath.Join(tmpDir, "error.bin")

	// Use timeout to prevent hanging on retries
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Provide dummy state to prevent panic
	state := types.NewProgressState("test-id", 1024*1024)
	dl := NewConcurrentDownloader("test-id", nil, state, types.DefaultRuntimeConfig())

	// Should fail eventually (retries exhausted or timeout)
	err := dl.Download(ctx, server.URL(), nil, nil, outputPath, 1024*1024, false)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

func TestConcurrentDownloader_Resume(t *testing.T) {
	tmpDir, cleanup := initTestState(t)
	defer cleanup()

	size := int64(5 * 1024 * 1024)
	server := testutil.NewMockServerT(t, testutil.WithFileSize(size), testutil.WithRandomData(true))
	defer server.Close()

	outputPath := filepath.Join(tmpDir, "resume.bin")

	// Create partial download (first 2MB downloaded)
	partialSize := int64(2 * 1024 * 1024)
	if _, err := testutil.CreateSurgeFile(tmpDir, "resume.bin", size, partialSize); err != nil {
		t.Fatalf("Failed to create partial file: %v", err)
	}

	// Prepare resume state
	state := types.NewProgressState("test-id", size)
	state.Downloaded.Store(partialSize)

	progressCh := make(chan any, 100)
	dl := NewConcurrentDownloader("test-id", progressCh, state, types.DefaultRuntimeConfig())

	err := dl.Download(context.Background(), server.URL(), nil, nil, outputPath, size, false)
	if err != nil {
		t.Fatalf("Resume failed: %v", err)
	}

	if err := testutil.VerifyFileSize(outputPath, size); err != nil {
		t.Errorf("File verification failed: %v", err)
	}
}

func TestConcurrentDownloader_Mirrors(t *testing.T) {
	tmpDir, cleanup := initTestState(t)
	defer cleanup()

	// Setup primary and mirror servers
	size := int64(1024 * 1024)
	primary := testutil.NewMockServerT(t, testutil.WithFileSize(size))
	defer primary.Close()

	mirror1 := testutil.NewMockServerT(t, testutil.WithFileSize(size))
	defer mirror1.Close()

	outputPath := filepath.Join(tmpDir, "mirrors.bin")

	state := types.NewProgressState("test-id", size)
	dl := NewConcurrentDownloader("test-id", nil, state, types.DefaultRuntimeConfig())

	// Pass mirror URLs
	mirrors := []string{mirror1.URL()}
	err := dl.Download(context.Background(), primary.URL(), mirrors, mirrors, outputPath, size, false)
	if err != nil {
		t.Fatalf("Download with mirrors failed: %v", err)
	}
}
