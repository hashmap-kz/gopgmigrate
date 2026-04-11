package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- ParseVersionDo ---

func TestParseVersionDo(t *testing.T) {
	tests := []struct {
		filename string
		expected int64
		hasError bool
	}{
		// versioned transactional
		{"0000001-users.do.sql", 1, false},
		{"0012345-roles.do.sql", 12345, false},
		{"0000000-init.do.sql", 0, false},
		{"0000123-test.do.sql", 123, false},

		// versioned non-transactional
		{"0000001-vacuum-users.notx.do.sql", 1, false},
		{"0000042-reindex.notx.do.sql", 42, false},

		// repeatable transactional
		{"0000003-fn-get-users.r.do.sql", 3, false},
		{"0000099-vw-active-users.r.do.sql", 99, false},

		// repeatable non-transactional
		{"0000007-fn-refresh.rnotx.do.sql", 7, false},

		// invalid — wrong digit count
		{"1-users.do.sql", -1, true},        // too short, not 7 digits
		{"000001-users.do.sql", -1, true},   // 6 digits
		{"00000001-users.do.sql", -1, true}, // 8 digits

		// invalid — other reasons
		{"users.do.sql", -1, true},             // missing version
		{"0000001_users.do.sql", -1, true},     // underscore separator
		{"0000001-users.up.sql", -1, true},     // wrong suffix
		{"0000001-users.sql", -1, true},        // missing type
		{"0000001-users.do.sql.bak", -1, true}, // extra suffix
		{"0000001-users.undo.sql", -1, true},   // undo is not do
		{"0000001-users.notx.sql", -1, true},   // missing .do. segment
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result, err := ParseVersionDo(tt.filename)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// --- ParseVersionUndo ---

func TestParseVersionUndo(t *testing.T) {
	tests := []struct {
		filename string
		expected int64
		hasError bool
	}{
		{"0000001-users.undo.sql", 1, false},
		{"0012345-roles.undo.sql", 12345, false},
		{"0000000-init.undo.sql", 0, false},
		{"0000123-test.undo.sql", 123, false},

		// invalid — wrong digit count
		{"1-users.undo.sql", -1, true},        // too short
		{"000001-users.undo.sql", -1, true},   // 6 digits
		{"00000001-users.undo.sql", -1, true}, // 8 digits

		// invalid — other reasons
		{"users.undo.sql", -1, true},             // missing version
		{"0000001_users.undo.sql", -1, true},     // underscore separator
		{"0000001-users.down.sql", -1, true},     // wrong suffix
		{"0000001-users.sql", -1, true},          // missing type
		{"0000001-users.undo.sql.bak", -1, true}, // extra suffix
		{"0000001-users.do.sql", -1, true},       // do is not undo
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result, err := ParseVersionUndo(tt.filename)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// --- doRegex ---

func TestDoRegex(t *testing.T) {
	tests := []struct {
		input   string
		matches bool
	}{
		// valid — all four do variants
		{"0000001-create-users.do.sql", true},
		{"0000001-vacuum-users.notx.do.sql", true},
		{"0000001-fn-get-users.r.do.sql", true},
		{"0000001-fn-refresh.rnotx.do.sql", true},

		// invalid — wrong digit count
		{"1-create-users.do.sql", false},        // too short
		{"000001-create-users.do.sql", false},   // 6 digits
		{"00000001-create-users.do.sql", false}, // 8 digits

		// invalid — other reasons
		{"0000001-users.undo.sql", false}, // undo direction
		{"0000001-users.up.sql", false},   // wrong suffix
		{"users.do.sql", false},           // missing version
		{"0000001-users.do.txt", false},   // wrong extension
		{"0000001-users.notx.sql", false}, // missing .do. segment
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.matches, doRegex.MatchString(tt.input))
		})
	}
}

// --- undoRegex ---

func TestUndoRegex(t *testing.T) {
	tests := []struct {
		input   string
		matches bool
	}{
		{"0000001-create-users.undo.sql", true},
		{"0000042-add-roles.undo.sql", true},

		// invalid — wrong digit count
		{"1-create-users.undo.sql", false},        // too short
		{"000001-create-users.undo.sql", false},   // 6 digits
		{"00000001-create-users.undo.sql", false}, // 8 digits

		// invalid — other reasons
		{"0000001-users.do.sql", false},   // do direction
		{"0000001-users.down.sql", false}, // wrong suffix
		{"users.undo.sql", false},         // missing version
		{"0000001-users.undo.txt", false}, // wrong extension
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.matches, undoRegex.MatchString(tt.input))
		})
	}
}

// --- IsRepeatable ---

func TestIsRepeatable(t *testing.T) {
	repeatable := []string{
		"0000001-fn-get-users.r.do.sql",
		"0000002-fn-refresh.rnotx.do.sql",
	}
	notRepeatable := []string{
		"0000001-create-users.do.sql",
		"0000001-vacuum.notx.do.sql",
		"0000001-create-users.undo.sql",
	}

	for _, base := range repeatable {
		t.Run("repeatable/"+base, func(t *testing.T) {
			assert.True(t, IsRepeatable(MigrationFile{Base: base}))
		})
	}
	for _, base := range notRepeatable {
		t.Run("not-repeatable/"+base, func(t *testing.T) {
			assert.False(t, IsRepeatable(MigrationFile{Base: base}))
		})
	}
}

// --- IsNonTransactional ---

func TestIsNonTransactional(t *testing.T) {
	notx := []string{
		"0000001-vacuum-users.notx.do.sql",
		"0000002-fn-refresh.rnotx.do.sql",
	}
	tx := []string{
		"0000001-create-users.do.sql",
		"0000001-fn-get-users.r.do.sql",
		"0000001-create-users.undo.sql",
	}

	for _, base := range notx {
		t.Run("notx/"+base, func(t *testing.T) {
			assert.True(t, IsNonTransactional(MigrationFile{Base: base}))
		})
	}
	for _, base := range tx {
		t.Run("tx/"+base, func(t *testing.T) {
			assert.False(t, IsNonTransactional(MigrationFile{Base: base}))
		})
	}
}

// --- IsTransactional ---

func TestIsTransactional(t *testing.T) {
	assert.True(t, IsTransactional(MigrationFile{Base: "0000001-create-users.do.sql"}))
	assert.True(t, IsTransactional(MigrationFile{Base: "0000001-fn-get-users.r.do.sql"}))
	assert.False(t, IsTransactional(MigrationFile{Base: "0000001-vacuum.notx.do.sql"}))
	assert.False(t, IsTransactional(MigrationFile{Base: "0000001-fn-refresh.rnotx.do.sql"}))
}

// --- IsDoFile / IsUndoFile ---

func TestIsDoFile(t *testing.T) {
	assert.True(t, IsDoFile("0000001-create-users.do.sql"))
	assert.True(t, IsDoFile("0000001-vacuum.notx.do.sql"))
	assert.True(t, IsDoFile("0000001-fn-get-users.r.do.sql"))
	assert.True(t, IsDoFile("0000001-fn-refresh.rnotx.do.sql"))
	assert.False(t, IsDoFile("0000001-create-users.undo.sql"))
	assert.False(t, IsDoFile("not-a-migration.sql"))
	assert.False(t, IsDoFile("1-create-users.do.sql")) // wrong digit count
}

func TestIsUndoFile(t *testing.T) {
	assert.True(t, IsUndoFile("0000001-create-users.undo.sql"))
	assert.False(t, IsUndoFile("0000001-create-users.do.sql"))
	assert.False(t, IsUndoFile("not-a-migration.sql"))
	assert.False(t, IsUndoFile("1-create-users.undo.sql")) // wrong digit count
}

// --- IsSchemaTablePath ---

func TestIsSchemaTablePath(t *testing.T) {
	valid := []string{
		"public.users",
		"myschema.table",
		"my_schema.table_123",
		"m$yschema1.m$table",
	}
	invalid := []string{
		"users",           // missing schema
		"",                // empty
		"a.b.c",           // three parts
		"123schema.table", // schema starts with digit
		"schema.123table", // table starts with digit
		"schema.",         // missing table
		".table",          // missing schema
		"schema.table$@",  // invalid character
		"schema.reallylongtablename111111111111$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$", // too long
	}

	for _, s := range valid {
		t.Run("valid/"+s, func(t *testing.T) {
			assert.True(t, IsSchemaTablePath(s))
		})
	}
	for _, s := range invalid {
		t.Run("invalid/"+s, func(t *testing.T) {
			assert.False(t, IsSchemaTablePath(s))
		})
	}
}
