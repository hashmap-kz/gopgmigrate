package migrate

import (
	"os"
	"path/filepath"
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
	createTestFile(t, tmpDir, "refresh.r.sql", "-- SQL content")

	// Run GetFiles
	files, err := GetFiles(tmpDir)
	if err != nil {
		t.Fatalf("GetFiles() failed: %v", err)
	}

	// Check that migrations were correctly detected
	if len(files.versioned) != 2 {
		t.Errorf("Expected 2 versioned migrations, got %d", len(files.versioned))
	}
	if len(files.repeatable) != 1 {
		t.Errorf("Expected 1 repeatable migration, got %d", len(files.repeatable))
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
		"refresh.r.sql",
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
	files := []migrationFile{
		{base: "00001-init.do.sql", path: "/migrations/00001-init.do.sql"},
		{base: "00002-users.do.sql", path: "/migrations/00002-users.do.sql"},
	}

	err := checkFilesAreUniqueByVersion(files)
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	// Introduce a duplicate version
	files = append(files, migrationFile{base: "00001-duplicate.do.sql", path: "/migrations/00001-duplicate.do.sql"})
	err = checkFilesAreUniqueByVersion(files)
	if err == nil {
		t.Errorf("Expected error due to duplicate version, but got nil")
	}
}

// Test checkVersionsAreSequential
func TestCheckVersionsAreSequential(t *testing.T) {
	files := []migrationFile{
		{base: "00001-init.do.sql", path: "/migrations/00001-init.do.sql"},
		{base: "00002-users.do.sql", path: "/migrations/00002-users.do.sql"},
		{base: "00003-roles.do.sql", path: "/migrations/00003-roles.do.sql"},
	}

	err := checkVersionsAreSequential(files)
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	// Introduce a gap in versions
	files = append(files, migrationFile{base: "00005-missing.do.sql", path: "/migrations/00005-missing.do.sql"})
	err = checkVersionsAreSequential(files)
	if err == nil {
		t.Errorf("Expected error due to non-sequential versions, but got nil")
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
