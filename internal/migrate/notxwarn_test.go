package migrate

import (
	"testing"
)

func TestCheckFileForWarnings(t *testing.T) {
	tests := []struct {
		name         string
		sqlContent   string
		expectedWarn []string
	}{
		{"Detect COPY FROM STDIN", "COPY users FROM STDIN;", []string{"Warning: Detected CopyFromStdin pattern"}},
		{"Detect CREATE DATABASE", "CREATE DATABASE testdb;", []string{"Warning: Detected CreateDatabaseTablespaceSubscription pattern"}},
		{"Detect ALTER SYSTEM", "ALTER SYSTEM SET work_mem = '64MB';", []string{"Warning: Detected AlterSystem pattern"}},
		{"Detect CREATE INDEX CONCURRENTLY", "CREATE INDEX CONCURRENTLY idx_users ON users (name);", []string{"Warning: Detected CreateIndexConcurrently pattern"}},
		{"Detect REINDEX", "REINDEX DATABASE mydb;", []string{"Warning: Detected Reindex pattern"}},
		{"Detect VACUUM", "VACUUM;", []string{"Warning: Detected Vacuum pattern"}},
		{"Detect VACUUM", "vacuum;", []string{"Warning: Detected Vacuum pattern"}},
		{"Detect DISCARD ALL", "DISCARD ALL;", []string{"Warning: Detected DiscardAll pattern"}},
		{"Detect ALTER TYPE ADD VALUE", "ALTER TYPE my_enum ADD VALUE 'new_value';", []string{"Warning: Detected AlterTypeAddValue pattern"}},
		{"Detect multiple patterns", "vacuum; create database newdb; alter system set log_statement = 'all';", []string{
			"Warning: Detected Vacuum pattern",
			"Warning: Detected CreateDatabaseTablespaceSubscription pattern",
			"Warning: Detected AlterSystem pattern",
		}},
		{"Safe SQL with no warnings", "SELECT * FROM users WHERE age > 18;", []string{}}, // No matches
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			warnings := checkThatFileIsPossibleShouldNotUseTx(test.sqlContent)
			if len(warnings) != len(test.expectedWarn) {
				t.Errorf("Expected %d warnings, got %d. Warnings: %v", len(test.expectedWarn), len(warnings), warnings)
			}
			for _, expected := range test.expectedWarn {
				if !contains(warnings, expected) {
					t.Errorf("Expected warning: %q, but not found in %v", expected, warnings)
				}
			}
		})
	}
}

func contains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
