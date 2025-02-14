package migrate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseVersionDo(t *testing.T) {
	tests := []struct {
		filename string
		expected int64
		hasError bool
	}{
		{"00000000000001-users.do.sql", 1, false},
		{"00000000012345-roles.do.sql", 12345, false},
		{"00000000000000-init.do.sql", 0, false},
		{"00000000000123-test.do.sql", 123, false},

		{"0000000001234-users.do.sql", -1, true},      // Invalid: needs exactly 14 digits
		{"0000000000000-users.do.sql", -1, true},      // Invalid: only 13 digits
		{"00000000000001_users.do.sql", -1, true},     // Invalid: uses `_` instead of `-`
		{"00000000000001-users.up.sql", -1, true},     // Invalid: wrong suffix
		{"000000000-users.do.sql", -1, true},          // Invalid: missing version number
		{"00000000000001-users.sql", -1, true},        // Invalid: missing `.do.sql`
		{"00000000000001-users.do.sql.bak", -1, true}, // Invalid: additional suffix
	}

	for _, test := range tests {
		t.Run(test.filename, func(t *testing.T) {
			result, err := parseVersionDo(test.filename)

			if test.hasError {
				assert.Error(t, err, "Expected an error but got none")
			} else {
				assert.NoError(t, err, "Expected no error but got: %v", err)
				assert.Equal(t, test.expected, result, "Expected version %d but got %d", test.expected, result)
			}
		})
	}
}

func TestParseVersionUndo(t *testing.T) {
	tests := []struct {
		filename string
		expected int64
		hasError bool
	}{
		{"00000000000001-users.undo.sql", 1, false},
		{"00000000012345-roles.undo.sql", 12345, false},
		{"00000000000000-init.undo.sql", 0, false},
		{"00000000000123-test.undo.sql", 123, false},
		{"0000000001234-users.undo.sql", -1, true},      // Invalid: needs exactly 5 digits
		{"0000000000000-users.undo.sql", -1, true},      // Invalid: only 4 digits
		{"00000000000001_users.undo.sql", -1, true},     // Invalid: uses `_` instead of `-`
		{"00000000000001-users.down.sql", -1, true},     // Invalid: wrong suffix
		{"000000000-users.undo.sql", -1, true},          // Invalid: missing version number
		{"00000000000001-users.sql", -1, true},          // Invalid: missing `.undo.sql`
		{"00000000000001-users.undo.sql.bak", -1, true}, // Invalid: additional suffix
	}

	for _, test := range tests {
		t.Run(test.filename, func(t *testing.T) {
			result, err := parseVersionUndo(test.filename)

			if test.hasError {
				assert.Error(t, err, "Expected an error but got none")
			} else {
				assert.NoError(t, err, "Expected no error but got: %v", err)
				assert.Equal(t, test.expected, result, "Expected version %d but got %d", test.expected, result)
			}
		})
	}
}

func TestVersionedMigrationRegexDo(t *testing.T) {
	tests := []struct {
		input   string
		matches bool
		version string
		name    string
		migType string
	}{
		{"00000000000003-users.do.sql", true, "00000000000003", "users", "do"},
		{"00000000000004-fn_list_users.r.sql", true, "00000000000004", "fn_list_users", "r"},
		{"000000000123-invalid.sql", false, "", "", ""},
		{"00000000000006-wrong.do.txt", false, "", "", ""},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			matches := versionedMigrationRegexDo.FindStringSubmatch(test.input)
			assert.Equal(t, test.matches, matches != nil)

			if test.matches {
				assert.Equal(t, test.version, matches[1])
				assert.Equal(t, test.name, matches[2])
				assert.Equal(t, test.migType, matches[3])
			}
		})
	}
}

func TestVersionedMigrationRegexUndo(t *testing.T) {
	tests := []struct {
		input   string
		matches bool
		version string
		name    string
		migType string
	}{
		{"00000000000003-users.undo.sql", true, "00000000000003", "users", "undo"},
		{"00000000000004-fn_list_users.undo.sql", true, "00000000000004", "fn_list_users", "undo"},
		{"00000000000005-users.do.sql", false, "", "", ""},
		{"00000000000006-wrong.undo.txt", false, "", "", ""},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			matches := versionedMigrationRegexUndo.FindStringSubmatch(test.input)
			assert.Equal(t, test.matches, matches != nil)

			if test.matches {
				assert.Equal(t, test.version, matches[1])
				assert.Equal(t, test.name, matches[2])
				assert.Equal(t, test.migType, matches[3])
			}
		})
	}
}

func TestRepeatableMigrationRegexDo(t *testing.T) {
	tests := []struct {
		input   string
		matches bool
		version string
		name    string
		migType string
	}{
		{"00000000000004-fn_list_users.r.sql", true, "00000000000004", "fn_list_users", "r"},
		{"00000000000005-users.do.sql", false, "", "", ""},
		{"00000000000007-invalid.txt", false, "", "", ""},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			matches := repeatableMigrationRegexDo.FindStringSubmatch(test.input)
			assert.Equal(t, test.matches, matches != nil)

			if test.matches {
				assert.Equal(t, test.version, matches[1])
				assert.Equal(t, test.name, matches[2])
				assert.Equal(t, test.migType, matches[3])
			}
		})
	}
}

func TestVersionedMigrationRegexNtx(t *testing.T) {
	tests := []struct {
		input   string
		matches bool
		version string
		name    string
		migType string
	}{
		{"00000000000003-vacuum-users.ntx.do.sql", true, "00000000000003", "vacuum-users", "do"},
		{"00000000000004-fn_alter_system_1.ntx.r.sql", true, "00000000000004", "fn_alter_system_1", "r"},
		{"00000000000005-users.do.sql", false, "", "", ""},
		{"00000000000006-invalid.ntx.txt", false, "", "", ""},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			matches := versionedMigrationRegexNtx.FindStringSubmatch(test.input)
			assert.Equal(t, test.matches, matches != nil)

			if test.matches {
				assert.Equal(t, test.version, matches[1])
				assert.Equal(t, test.name, matches[2])
				assert.Equal(t, test.migType, matches[3])
			}
		})
	}
}

func TestPostgresqlSchemaTablePathRegex(t *testing.T) {
	tests := []struct {
		input   string
		matches bool
	}{
		{"myschema.table", true},
		{"m$yschema1.m$table", true},
		{"public.users", true},
		{"my_schema.table$", true},
		{"sche$ma.table_123$456", true},

		{"123schema.table", false}, // Invalid because schema name starts with a number
		{"myschema..table", false}, // Invalid format
		{"myschema-table", false},  // Invalid: should use dot separator
		{"_schema.$table", false},  // Cannot start with $
		{"schema.123table", false}, // Table name must start with letter or _
		{"schema..table", false},   // Missing schema
		{".table", false},          // Missing schema
		{"schema.", false},         // Missing table
		{"123schema.table", false}, // Schema must start with letter or _
		{"schema.table$@", false},  // Invalid character
		{"schema.reallylongtablename111111111111$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$", false}, // Too long
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			matches := postgresqlSchemaTablePathRegex.MatchString(test.input)
			assert.Equal(t, test.matches, matches)
		})
	}
}
