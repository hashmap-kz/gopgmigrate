//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"testing"
)

// MigrationDir is a temporary directory holding SQL migration files for one test.
type MigrationDir struct {
	Root string
}

func NewMigrationDir(t *testing.T) *MigrationDir {
	t.Helper()
	return &MigrationDir{Root: t.TempDir()}
}

// Add writes a SQL file at the given relative path.
// Calling Add with the same filename twice overwrites the first write —
// this is intentional for repeatable migration tests that need to change file content.
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

func removeFile(dir, filename string) error {
	return os.Remove(filepath.Join(dir, filename))
}
