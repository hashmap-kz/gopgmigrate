//go:build integration

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
// Calling Add with the same filename twice overwrites the first write -
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

// ManifestEntry describes one entry in the migrations list.
type ManifestEntry struct {
	Files       []string // paths relative to MigrationDir.Root
	Mode        string   // "", "atomic", "no-tx", "repeatable"
	Description string
}

// WriteManifest writes manifest.yaml to the root dir and returns its absolute path.
// Calling WriteManifest again overwrites the previous manifest - useful for tests
// that need to evolve the manifest between runs.
func (m *MigrationDir) WriteManifest(t *testing.T, table string, entries []ManifestEntry) string {
	t.Helper()
	var sb strings.Builder
	sb.WriteString("manifest:\n")
	if table != "" {
		fmt.Fprintf(&sb, "  table: %s\n", table)
	}
	sb.WriteString("migrations:\n")
	for _, e := range entries {
		sb.WriteString("  - files:\n")
		for _, f := range e.Files {
			fmt.Fprintf(&sb, "      - %s\n", f)
		}
		if e.Mode != "" {
			fmt.Fprintf(&sb, "    mode: %s\n", e.Mode)
		}
		if e.Description != "" {
			fmt.Fprintf(&sb, "    description: %q\n", e.Description)
		}
	}
	path := filepath.Join(m.Root, "manifest.yaml")
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return path
}
