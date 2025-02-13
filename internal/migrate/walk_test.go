package migrate

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// Test GetFiles function
func TestGetFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create valid versioned migration files
	validFiles := []string{
		"00001-init.do.sql",
		"00002-users.do.sql",
	}
	for _, f := range validFiles {
		createTestFile(t, tmpDir, f, "-- SQL content")
	}

	// Create a valid repeatable migration file
	createTestFile(t, tmpDir, "00003-refresh.r.sql", "-- SQL content")

	// Run GetFiles
	files, err := GetFiles(tmpDir, map[string]*regexp.Regexp{})
	if err != nil {
		t.Fatalf("GetFiles() failed: %v", err)
	}

	// Check that migrations were correctly detected
	if len(files) != 3 {
		t.Errorf("Expected 3 versioned migrations, got %d", len(files))
	}
}

// Test checkMigrationDirectoryDoesNotContainStrayFiles
func TestCheckMigrationDirectoryDoesNotContainStrayFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid migration file
	createTestFile(t, tmpDir, "00001-init.do.sql", "-- SQL content")

	// Create a stray file
	createTestFile(t, tmpDir, "random.txt", "stray content")

	err := checkMigrationDirectoryDoesNotContainStrayFiles(tmpDir)
	if err == nil {
		t.Errorf("Expected an error due to stray files, but got nil")
	}
}

// Test getAllStrayFiles
func TestGetAllStrayFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create valid migration files
	validFiles := []string{
		"00001-init.do.sql",
		"00002-refresh.r.sql",
	}
	for _, f := range validFiles {
		createTestFile(t, tmpDir, f, "-- SQL content")
	}

	// Create stray files
	strayFiles := []string{
		"random.txt",
		"config.yaml",
	}
	for _, f := range strayFiles {
		createTestFile(t, tmpDir, f, "stray content")
	}

	foundStrays, err := getAllStrayFiles(tmpDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(foundStrays) != len(strayFiles) {
		t.Errorf("Expected %d stray files, got %d", len(strayFiles), len(foundStrays))
	}
}

// Test checkFilesAreUniqueByVersion
func TestCheckFilesAreUniqueByVersion(t *testing.T) {
	files := []MigrationFile{
		{Base: "00001-init.do.sql", Path: "/migrations/00001-init.do.sql", Vers: 1},
		{Base: "00002-users.do.sql", Path: "/migrations/00002-users.do.sql", Vers: 2},
	}

	err := checkFilesAreUniqueByVersion(files)
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	// Introduce a duplicate version
	files = append(files, MigrationFile{Base: "00001-duplicate.do.sql", Path: "/migrations/00001-duplicate.do.sql", Vers: 1})
	err = checkFilesAreUniqueByVersion(files)
	if err == nil {
		t.Errorf("Expected error due to duplicate version, but got nil")
	}
}

// Helper function to create test files
func createTestFile(t *testing.T, dir, filename, content string) {
	t.Helper()
	path := filepath.Join(dir, filename)
	err := os.WriteFile(path, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file %s: %v", filename, err)
	}
}
