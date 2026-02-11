package state

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestDBMigration(t *testing.T) {
	// Create temp directory for test
	tempDir, err := os.MkdirTemp("", "surge-test-migration-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	dbPath := filepath.Join(tempDir, "surge.db")

	// 1. Setup initial state: DB with old schema (missing new columns)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open DB for setup: %v", err)
	}

	// Create tables without the new columns
	query := `
	CREATE TABLE IF NOT EXISTS downloads (
		id TEXT PRIMARY KEY,
		url TEXT NOT NULL,
		dest_path TEXT NOT NULL,
		filename TEXT,
		status TEXT,
		total_size INTEGER,
		downloaded INTEGER,
		url_hash TEXT,
		created_at INTEGER,
		paused_at INTEGER,
		completed_at INTEGER,
		time_taken INTEGER
	);
	CREATE TABLE IF NOT EXISTS tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		download_id TEXT,
		offset INTEGER,
		length INTEGER,
		FOREIGN KEY(download_id) REFERENCES downloads(id) ON DELETE CASCADE
	);
	`
	if _, err := db.Exec(query); err != nil {
		t.Fatalf("Failed to create initial tables: %v", err)
	}
	db.Close()

	// 2. Configure and Init DB (Run Migration)
	// We need to reset the singleton state first
	dbMu.Lock()
	if db != nil {
		_ = db.Close()
		db = nil // Reset global db
	}
	configured = false
	dbMu.Unlock()

	Configure(dbPath)

	// GetDB calls initDB, which runs the migration
	d, err := GetDB()
	if err != nil {
		t.Fatalf("GetDB (migration) failed: %v", err)
	}

	// 3. Verify columns exist
	checkColumnExists(t, d, "downloads", "mirrors")
	checkColumnExists(t, d, "downloads", "chunk_bitmap")
	checkColumnExists(t, d, "downloads", "actual_chunk_size")

	// 4. Close and Re-open (Idempotency Check)
	CloseDB()

	// Re-open
	d2, err := GetDB()
	if err != nil {
		t.Fatalf("GetDB (re-open) failed: %v", err)
	}
	defer CloseDB()

	// Verify columns still exist and no error occurred
	checkColumnExists(t, d2, "downloads", "mirrors")
}

func checkColumnExists(t *testing.T, db *sql.DB, tableName, columnName string) {
	t.Helper()
	var count int
	query := fmt.Sprintf("SELECT count(*) FROM pragma_table_info('%s') WHERE name = ?", tableName)
	err := db.QueryRow(query, columnName).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to check column existence for %s.%s: %v", tableName, columnName, err)
	}
	if count == 0 {
		t.Errorf("Column %s.%s does not exist", tableName, columnName)
	}
}
