package download

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestUniqueFilePath_ManyConflicts_ExceedsLimit(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "surge-test-limit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create 150 conflicting files
	// file.txt
	// file(1).txt ... file(149).txt

	basePath := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(basePath, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	for i := 1; i < 150; i++ {
		path := filepath.Join(tmpDir, fmt.Sprintf("file(%d).txt", i))
		if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// We expect file(150).txt
	result := uniqueFilePath(basePath)
	expected := filepath.Join(tmpDir, "file(150).txt")

	if result != expected {
		t.Errorf("uniqueFilePath() = %v, want %v", result, expected)
	}
}
