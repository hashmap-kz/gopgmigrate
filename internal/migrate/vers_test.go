package migrate

import (
	"testing"
)

func TestParseVersionDo(t *testing.T) {
	tests := []struct {
		filename string
		expected int64
		hasError bool
	}{
		{"00001-users.do.sql", 1, false},
		{"12345-roles.do.sql", 12345, false},
		{"00000-init.do.sql", 0, false},
		{"00123-test.do.sql", 123, false},

		{"1234-users.do.sql", -1, true},      // Invalid: needs exactly 5 digits
		{"0000-users.do.sql", -1, true},      // Invalid: only 4 digits
		{"00001_users.do.sql", -1, true},     // Invalid: uses `_` instead of `-`
		{"00001-users.up.sql", -1, true},     // Invalid: wrong suffix
		{"users.do.sql", -1, true},           // Invalid: missing version number
		{"00001-users.sql", -1, true},        // Invalid: missing `.do.sql`
		{"00001-users.do.sql.bak", -1, true}, // Invalid: additional suffix
	}

	for _, test := range tests {
		result, err := parseVersionDo(test.filename)
		if (err != nil) != test.hasError {
			t.Errorf("parseVersionDo(%q) unexpected error status: got %v, want error: %v", test.filename, err, test.hasError)
		}
		if result != test.expected {
			t.Errorf("parseVersionDo(%q) = %d, want %d", test.filename, result, test.expected)
		}
	}
}

func TestParseVersionUndo(t *testing.T) {
	tests := []struct {
		filename string
		expected int64
		hasError bool
	}{
		{"00001-users.undo.sql", 1, false},
		{"12345-roles.undo.sql", 12345, false},
		{"00000-init.undo.sql", 0, false},
		{"00123-test.undo.sql", 123, false},

		{"1234-users.undo.sql", -1, true},      // Invalid: needs exactly 5 digits
		{"0000-users.undo.sql", -1, true},      // Invalid: only 4 digits
		{"00001_users.undo.sql", -1, true},     // Invalid: uses `_` instead of `-`
		{"00001-users.down.sql", -1, true},     // Invalid: wrong suffix
		{"users.undo.sql", -1, true},           // Invalid: missing version number
		{"00001-users.sql", -1, true},          // Invalid: missing `.undo.sql`
		{"00001-users.undo.sql.bak", -1, true}, // Invalid: additional suffix
	}

	for _, test := range tests {
		result, err := parseVersionUndo(test.filename)
		if (err != nil) != test.hasError {
			t.Errorf("parseVersionUndo(%q) unexpected error status: got %v, want error: %v", test.filename, err, test.hasError)
		}
		if result != test.expected {
			t.Errorf("parseVersionUndo(%q) = %d, want %d", test.filename, result, test.expected)
		}
	}
}

func TestRepeatableMigrationRegex(t *testing.T) {
	tests := []struct {
		filename string
		matches  bool
	}{
		{"users.r.sql", true},
		{"init.r.sql", true},
		{"backup_v1.r.sql", true},
		{"my-migration.r.sql", true},
		{"123.r.sql", true},

		{"migration.R.SQL", false},     // Case sensitive
		{"migration.sql", false},       // Missing `.r.`
		{"migration.rsql", false},      // Missing dot before `sql`
		{"migration.r.sql.bak", false}, // Extra suffix
		{"migration.rs.sql", false},    // Extra character
		{".r.sql", false},              // Missing migration name
	}

	for _, test := range tests {
		matches := repeatableMigrationRegex.MatchString(test.filename)
		if matches != test.matches {
			t.Errorf("repeatableMigrationRegex.MatchString(%q) = %v, want %v", test.filename, matches, test.matches)
		}
	}
}
