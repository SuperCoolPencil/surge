package concurrent

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/surge-downloader/surge/internal/engine/types"
	"github.com/surge-downloader/surge/internal/testutil"
)

// TestConcurrentDownloader_HttpError_404 verifies that the downloader fails fast on 404 errors.
// Currently, it is expected to hang/retry until timeout if the bug exists.
func TestConcurrentDownloader_HttpError_404(t *testing.T) {
	tmpDir, cleanup := initTestState(t)
	defer cleanup()

	fileSize := int64(1024)
	server := testutil.NewMockServerT(t,
		testutil.WithFileSize(fileSize),
		testutil.WithHandler(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Not Found", http.StatusNotFound)
		}),
	)
	defer server.Close()

	destPath := filepath.Join(tmpDir, "error_test.bin")
	state := types.NewProgressState("error-test", fileSize)
	runtime := &types.RuntimeConfig{
		MaxConnectionsPerHost: 1,
		MaxTaskRetries:        2, // Low retries to fail fast
	}

	downloader := NewConcurrentDownloader("error-id", nil, state, runtime)

	// Set a timeout that is long enough for a few retries but short enough to detect an infinite loop.
	// If it retries forever, it will hit this timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	err := downloader.Download(ctx, server.URL(), nil, nil, destPath, fileSize, false)
	duration := time.Since(start)

	if err == nil {
		t.Fatal("Expected download failure, got success")
	}

	// If the error is context deadline exceeded, it means it hung (bug present).
	if err == context.DeadlineExceeded {
		t.Fatalf("Download hung/retried indefinitely on 404 (bug reproduced)")
	}

	// If we fixed it, it should return a non-timeout error quickly.
	if duration > 1500*time.Millisecond {
		t.Logf("Warning: Download took %v, might be retrying too much", duration)
	}

	t.Logf("Download failed correctly with: %v", err)
}

func TestConcurrentDownloader_HttpError_403(t *testing.T) {
	tmpDir, cleanup := initTestState(t)
	defer cleanup()

	fileSize := int64(1024)
	server := testutil.NewMockServerT(t,
		testutil.WithFileSize(fileSize),
		testutil.WithHandler(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Forbidden", http.StatusForbidden)
		}),
	)
	defer server.Close()

	destPath := filepath.Join(tmpDir, "error_test_403.bin")
	state := types.NewProgressState("error-test-403", fileSize)
	runtime := &types.RuntimeConfig{
		MaxConnectionsPerHost: 1,
		MaxTaskRetries:        2,
	}

	downloader := NewConcurrentDownloader("error-id-403", nil, state, runtime)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := downloader.Download(ctx, server.URL(), nil, nil, destPath, fileSize, false)

	if err == nil {
		t.Fatal("Expected download failure, got success")
	}

	if err == context.DeadlineExceeded {
		t.Fatalf("Download hung/retried indefinitely on 403 (bug reproduced)")
	}

	t.Logf("Download failed correctly with: %v", err)
}
