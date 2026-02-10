package utils_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/surge-downloader/surge/internal/engine/types"
	"github.com/surge-downloader/surge/internal/testutil"
	"github.com/surge-downloader/surge/internal/utils"
)

func TestCopyFile(t *testing.T) {
	tmpDir, cleanup, err := testutil.TempDir("surge-copy-test")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	// Create source file
	srcPath, err := testutil.CreateTestFile(tmpDir, "src.bin", 1024, true)
	if err != nil {
		t.Fatal(err)
	}

	dstPath := filepath.Join(tmpDir, "dst.bin")

	err = utils.CopyFile(srcPath, dstPath)
	if err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}

	// Verify destination exists
	if !testutil.FileExists(dstPath) {
		t.Error("Destination file should exist")
	}

	// Verify sizes match
	srcInfo, _ := os.Stat(srcPath)
	dstInfo, _ := os.Stat(dstPath)
	if srcInfo.Size() != dstInfo.Size() {
		t.Error("File sizes don't match")
	}

	// Verify contents match
	match, err := testutil.CompareFiles(srcPath, dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if !match {
		t.Error("File contents don't match")
	}
}

func TestCopyFile_SourceNotExists(t *testing.T) {
	tmpDir, cleanup, _ := testutil.TempDir("surge-copy-test")
	defer cleanup()

	err := utils.CopyFile(filepath.Join(tmpDir, "nonexistent.bin"), filepath.Join(tmpDir, "dst.bin"))
	if err == nil {
		t.Error("Expected error for nonexistent source")
	}
}

func TestCopyFile_InvalidDestination(t *testing.T) {
	tmpDir, cleanup, _ := testutil.TempDir("surge-copy-test")
	defer cleanup()

	srcPath, _ := testutil.CreateTestFile(tmpDir, "src.bin", 100, false)

	// Try to copy to an invalid path (non-existent directory)
	err := utils.CopyFile(srcPath, filepath.Join(tmpDir, "nonexistent", "subdir", "dst.bin"))
	if err == nil {
		t.Error("Expected error for invalid destination")
	}
}

func TestCopyFile_EmptyFile(t *testing.T) {
	tmpDir, cleanup, _ := testutil.TempDir("surge-copy-test")
	defer cleanup()

	srcPath, _ := testutil.CreateTestFile(tmpDir, "empty.bin", 0, false)
	dstPath := filepath.Join(tmpDir, "empty_copy.bin")

	err := utils.CopyFile(srcPath, dstPath)
	if err != nil {
		t.Fatalf("CopyFile failed for empty file: %v", err)
	}

	if err := testutil.VerifyFileSize(dstPath, 0); err != nil {
		t.Error(err)
	}
}

func TestCopyFile_LargeFile(t *testing.T) {
	tmpDir, cleanup, _ := testutil.TempDir("surge-copy-test")
	defer cleanup()

	size := int64(5 * types.MB)
	srcPath, _ := testutil.CreateTestFile(tmpDir, "large.bin", size, false)
	dstPath := filepath.Join(tmpDir, "large_copy.bin")

	err := utils.CopyFile(srcPath, dstPath)
	if err != nil {
		t.Fatalf("CopyFile failed for large file: %v", err)
	}

	if err := testutil.VerifyFileSize(dstPath, size); err != nil {
		t.Error(err)
	}
}

func TestCopyFile_ContentVerification(t *testing.T) {
	tmpDir, cleanup, _ := testutil.TempDir("surge-copy-content")
	defer cleanup()

	size := int64(128 * types.KB)
	srcPath, _ := testutil.CreateTestFile(tmpDir, "random.bin", size, true) // Random data
	dstPath := filepath.Join(tmpDir, "random_copy.bin")

	err := utils.CopyFile(srcPath, dstPath)
	if err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}

	match, err := testutil.CompareFiles(srcPath, dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if !match {
		t.Error("Copied file content doesn't match source")
	}
}
