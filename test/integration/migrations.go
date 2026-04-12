//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"testing"
)

// MigrationDir is a temporary directory that holds SQL migration files
// for one test. It is cleaned up automatically when the test ends.
type MigrationDir struct {
	Root string
}

// NewMigrationDir creates a fresh temporary directory for the test.
func NewMigrationDir(t *testing.T) *MigrationDir {
	t.Helper()
	return &MigrationDir{Root: t.TempDir()}
}

// Add writes a SQL file at the given relative path inside the directory.
// Intermediate subdirectories are created as needed.
// Calling Add with the same filename twice overwrites the first file —
// this is intentional for repeatable migration tests that need to change
// file content between runs.
func (m *MigrationDir) Add(t *testing.T, filename, content string) {
	t.Helper()
	path := filepath.Join(m.Root, filename)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", filename, err)
	}
}

// Sub returns the absolute path to a subdirectory inside the migration dir.
func (m *MigrationDir) Sub(subdir string) string {
	return filepath.Join(m.Root, subdir)
}
