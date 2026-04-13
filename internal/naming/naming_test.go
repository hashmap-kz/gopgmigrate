package naming

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseMigrationName_Valid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		revision int64
		base     string
		kind     MigrationKind
	}{
		{
			name:     "versioned transactional",
			input:    "0000001-create-users.up.sql",
			revision: 1,
			base:     "create-users",
			kind:     MigrationKindUp,
		},
		{
			name:     "repeatable transactional",
			input:    "0000002-refresh-user-stats.r.up.sql",
			revision: 2,
			base:     "refresh-user-stats",
			kind:     MigrationKindRUp,
		},
		{
			name:     "versioned non transactional",
			input:    "0000003-vacuum-big-table.notx.up.sql",
			revision: 3,
			base:     "vacuum-big-table",
			kind:     MigrationKindNotxUp,
		},
		{
			name:     "repeatable non transactional",
			input:    "0000004-refresh-heavy-view.rnotx.up.sql",
			revision: 4,
			base:     "refresh-heavy-view",
			kind:     MigrationKindRNotxUp,
		},
		{
			name:     "rollback",
			input:    "0000005-create-users.down.sql",
			revision: 5,
			base:     "create-users",
			kind:     MigrationKindDown,
		},
		{
			name:     "name with dots",
			input:    "0000006-create.users.table.up.sql",
			revision: 6,
			base:     "create.users.table",
			kind:     MigrationKindUp,
		},
		{
			name:     "name with spaces",
			input:    "0000007-create users table.up.sql",
			revision: 7,
			base:     "create users table",
			kind:     MigrationKindUp,
		},
		{
			name:     "name with unicode",
			input:    "0000008-создать-таблицу.up.sql",
			revision: 8,
			base:     "создать-таблицу",
			kind:     MigrationKindUp,
		},
		{
			name:     "name containing suffix like substring",
			input:    "0000009-feature.rnotx.upgrade.up.sql",
			revision: 9,
			base:     "feature.rnotx.upgrade",
			kind:     MigrationKindUp,
		},
		{
			name:     "name containing down substring",
			input:    "0000010-prepare-down-migration.up.sql",
			revision: 10,
			base:     "prepare-down-migration",
			kind:     MigrationKindUp,
		},
		{
			name:     "name containing sql substring",
			input:    "0000011-create-sql-helper.up.sql",
			revision: 11,
			base:     "create-sql-helper",
			kind:     MigrationKindUp,
		},
		{
			name:     "zero revision allowed",
			input:    "0000000-init.up.sql",
			revision: 0,
			base:     "init",
			kind:     MigrationKindUp,
		},
		{
			name:     "name with many dots and spaces",
			input:    "0000013-my file.v2.final.r.up.sql",
			revision: 13,
			base:     "my file.v2.final",
			kind:     MigrationKindRUp,
		},
		{
			name:     "name ending with reserved word fragment",
			input:    "0000014-create-users.tx.up.sql",
			revision: 14,
			base:     "create-users.tx",
			kind:     MigrationKindUp,
		},
		{
			name:     "name ending with repeatable fragment",
			input:    "0000015-create-users.repeatable.up.sql",
			revision: 15,
			base:     "create-users.repeatable",
			kind:     MigrationKindUp,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseMigrationName(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.revision, got.Revision)
			assert.Equal(t, tt.base, got.Name)
			assert.Equal(t, tt.kind, got.Kind)
		})
	}
}

func TestParseMigrationName_Invalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{name: "empty string", input: ""},
		{name: "missing revision", input: "create-users.up.sql"},
		{name: "missing hyphen separator", input: "0000001create-users.up.sql"},
		{name: "wrong revision width short", input: "000001-create-users.up.sql"},
		{name: "wrong revision width long", input: "00000001-create-users.up.sql"},
		{name: "non digit revision", input: "00000a1-create-users.up.sql"},
		{name: "negative revision style", input: "-000001-create-users.up.sql"},
		{name: "empty base name", input: "0000001-.up.sql"},
		{name: "slash in base name", input: "0000001-dir/create-users.up.sql"},
		{name: "path traversal like input", input: "0000001-../create-users.up.sql"},
		{name: "wrong forward suffix do", input: "0000001-create-users.do.sql"},
		{name: "wrong rollback suffix undo", input: "0000001-create-users.undo.sql"},
		{name: "wrong order up r", input: "0000001-create-users.up.r.sql"},
		{name: "wrong order up notx", input: "0000001-create-users.up.notx.sql"},
		{name: "missing sql suffix", input: "0000001-create-users.up"},
		{name: "extra suffix after sql", input: "0000001-create-users.up.sql.bak"},
		{name: "uppercase kind", input: "0000001-create-users.UP.sql"},
		{name: "uppercase extension", input: "0000001-create-users.up.SQL"},
		{name: "space after kind", input: "0000001-create-users.up.sql "},
		{name: "space before revision", input: " 0000001-create-users.up.sql"},
		{name: "only basename without kind", input: "0000001-create-users.sql"},
		{name: "name starts with dot repeatable", input: "0000001-.r.up.sql"},
		{name: "name starts with dot notx", input: "0000001-.notx.up.sql"},
		{name: "name starts with dot rnotx", input: "0000001-.rnotx.up.sql"},
		{name: "name starts with dot down", input: "0000001-.down.sql"},
		{name: "double dot before kind", input: "0000001-create-users..up.sql"},
		{name: "directory prefix should not match basename parser", input: "./0000001-create-users.up.sql"},
		{name: "absolute path should not match basename parser", input: "/tmp/0000001-create-users.up.sql"},
		{name: "name ends with dot before repeatable kind", input: "0000001-create-users..r.up.sql"},
		{name: "name ends with dot before notx kind", input: "0000001-create-users..notx.up.sql"},
		{name: "name contains double dot inside", input: "0000001-create..users.up.sql"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseMigrationName(tt.input)
			assert.Error(t, err)
		})
	}
}

func TestMigrationRegex_KindCoverage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		kind  MigrationKind
	}{
		{"0000001-a.up.sql", MigrationKindUp},
		{"0000001-a.r.up.sql", MigrationKindRUp},
		{"0000001-a.notx.up.sql", MigrationKindNotxUp},
		{"0000001-a.rnotx.up.sql", MigrationKindRNotxUp},
		{"0000001-a.down.sql", MigrationKindDown},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(string(tt.kind), func(t *testing.T) {
			t.Parallel()

			got, err := ParseMigrationName(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.kind, got.Kind)
		})
	}
}

func TestMigrationRegex_NameGreedinessIsSafe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
		kind     MigrationKind
	}{
		{
			input:    "0000001-my.r.up.migration.up.sql",
			expected: "my.r.up.migration",
			kind:     MigrationKindUp,
		},
		{
			input:    "0000002-my.notx.up.migration.r.up.sql",
			expected: "my.notx.up.migration",
			kind:     MigrationKindRUp,
		},
		{
			input:    "0000003-my.down.migration.notx.up.sql",
			expected: "my.down.migration",
			kind:     MigrationKindNotxUp,
		},
		{
			input:    "0000004-my.rnotx.up.migration.rnotx.up.sql",
			expected: "my.rnotx.up.migration",
			kind:     MigrationKindRNotxUp,
		},
		{
			input:    "0000005-my.down.script.down.sql",
			expected: "my.down.script",
			kind:     MigrationKindDown,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got, err := ParseMigrationName(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got.Name)
			assert.Equal(t, tt.kind, got.Kind)
		})
	}
}

func TestParseVersionUp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected int64
		hasError bool
	}{
		{name: "versioned tx", input: "0000001-users.up.sql", expected: 1},
		{name: "repeatable tx", input: "0000002-users.r.up.sql", expected: 2},
		{name: "versioned notx", input: "0000003-users.notx.up.sql", expected: 3},
		{name: "repeatable notx", input: "0000004-users.rnotx.up.sql", expected: 4},
		{name: "down file", input: "0000005-users.down.sql", hasError: true},
		{name: "invalid", input: "users.up.sql", hasError: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseVersionUp(tt.input)
			if tt.hasError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestParseVersionDown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected int64
		hasError bool
	}{
		{name: "down file", input: "0000005-users.down.sql", expected: 5},
		{name: "up file", input: "0000001-users.up.sql", hasError: true},
		{name: "invalid", input: "users.down.sql", hasError: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseVersionDown(tt.input)
			if tt.hasError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestHelpers(t *testing.T) {
	t.Parallel()

	t.Run("is tx", func(t *testing.T) {
		t.Parallel()

		assert.True(t, IsTx(&MigrationFile{Base: "0000001-users.up.sql"}))
		assert.True(t, IsTx(&MigrationFile{Base: "0000002-users.r.up.sql"}))
		assert.False(t, IsTx(&MigrationFile{Base: "0000003-users.notx.up.sql"}))
		assert.False(t, IsTx(&MigrationFile{Base: "0000004-users.rnotx.up.sql"}))
		assert.True(t, IsTx(&MigrationFile{Base: "0000005-users.down.sql"}))
		assert.False(t, IsTx(&MigrationFile{Base: "broken.sql"}))
	})

	t.Run("is repeatable", func(t *testing.T) {
		t.Parallel()

		assert.False(t, IsRepeatable(&MigrationFile{Base: "0000001-users.up.sql"}))
		assert.True(t, IsRepeatable(&MigrationFile{Base: "0000002-users.r.up.sql"}))
		assert.False(t, IsRepeatable(&MigrationFile{Base: "0000003-users.notx.up.sql"}))
		assert.True(t, IsRepeatable(&MigrationFile{Base: "0000004-users.rnotx.up.sql"}))
		assert.False(t, IsRepeatable(&MigrationFile{Base: "0000005-users.down.sql"}))
		assert.False(t, IsRepeatable(&MigrationFile{Base: "broken.sql"}))
	})

	t.Run("is versioned", func(t *testing.T) {
		t.Parallel()

		assert.True(t, IsVersioned("0000001-users.up.sql"))
		assert.True(t, IsVersioned("0000002-users.r.up.sql"))
		assert.True(t, IsVersioned("0000003-users.notx.up.sql"))
		assert.True(t, IsVersioned("0000004-users.rnotx.up.sql"))
		assert.False(t, IsVersioned("0000005-users.down.sql"))
		assert.False(t, IsVersioned("broken.sql"))
	})

	t.Run("is up", func(t *testing.T) {
		t.Parallel()

		assert.True(t, IsUp("0000001-users.up.sql"))
		assert.False(t, IsUp("0000002-users.r.up.sql"))
		assert.False(t, IsUp("0000003-users.notx.up.sql"))
		assert.False(t, IsUp("0000004-users.rnotx.up.sql"))
		assert.False(t, IsUp("0000005-users.down.sql"))
		assert.False(t, IsUp("broken.sql"))
	})

	t.Run("is repeatable up", func(t *testing.T) {
		t.Parallel()

		assert.False(t, IsRepeatableUp("0000001-users.up.sql"))
		assert.True(t, IsRepeatableUp("0000002-users.r.up.sql"))
		assert.False(t, IsRepeatableUp("0000003-users.notx.up.sql"))
		assert.False(t, IsRepeatableUp("0000004-users.rnotx.up.sql"))
		assert.False(t, IsRepeatableUp("0000005-users.down.sql"))
		assert.False(t, IsRepeatableUp("broken.sql"))
	})

	t.Run("is non transactional up", func(t *testing.T) {
		t.Parallel()

		assert.False(t, IsNonTransactionalUp("0000001-users.up.sql"))
		assert.False(t, IsNonTransactionalUp("0000002-users.r.up.sql"))
		assert.True(t, IsNonTransactionalUp("0000003-users.notx.up.sql"))
		assert.False(t, IsNonTransactionalUp("0000004-users.rnotx.up.sql"))
		assert.False(t, IsNonTransactionalUp("0000005-users.down.sql"))
		assert.False(t, IsNonTransactionalUp("broken.sql"))
	})

	t.Run("is repeatable non transactional up", func(t *testing.T) {
		t.Parallel()

		assert.False(t, IsRepeatableNonTransactionalUp("0000001-users.up.sql"))
		assert.False(t, IsRepeatableNonTransactionalUp("0000002-users.r.up.sql"))
		assert.False(t, IsRepeatableNonTransactionalUp("0000003-users.notx.up.sql"))
		assert.True(t, IsRepeatableNonTransactionalUp("0000004-users.rnotx.up.sql"))
		assert.False(t, IsRepeatableNonTransactionalUp("0000005-users.down.sql"))
		assert.False(t, IsRepeatableNonTransactionalUp("broken.sql"))
	})

	t.Run("is down", func(t *testing.T) {
		t.Parallel()

		assert.False(t, IsDown("0000001-users.up.sql"))
		assert.False(t, IsDown("0000002-users.r.up.sql"))
		assert.False(t, IsDown("0000003-users.notx.up.sql"))
		assert.False(t, IsDown("0000004-users.rnotx.up.sql"))
		assert.True(t, IsDown("0000005-users.down.sql"))
		assert.False(t, IsDown("broken.sql"))
	})

	t.Run("is non transaction", func(t *testing.T) {
		t.Parallel()

		assert.False(t, IsNonTransaction("0000001-users.up.sql"))
		assert.False(t, IsNonTransaction("0000002-users.r.up.sql"))
		assert.True(t, IsNonTransaction("0000003-users.notx.up.sql"))
		assert.True(t, IsNonTransaction("0000004-users.rnotx.up.sql"))
		assert.False(t, IsNonTransaction("0000005-users.down.sql"))
		assert.False(t, IsNonTransaction("broken.sql"))
	})
}

func TestPostgresqlSchemaTablePathRegex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		matches bool
	}{
		{"myschema.table", true},
		{"m$yschema1.m$table", true},
		{"public.users", true},
		{"my_schema.table$", true},
		{"sche$ma.table_123$456", true},

		{"123schema.table", false},
		{"myschema..table", false},
		{"myschema-table", false},
		{"_schema.$table", false},
		{"schema.123table", false},
		{"schema..table", false},
		{".table", false},
		{"schema.", false},
		{"123schema.table", false},
		{"schema.table$@", false},
		{"schema.reallylongtablename111111111111$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$", false},
	}

	for _, test := range tests {
		test := test
		t.Run(test.input, func(t *testing.T) {
			t.Parallel()

			matches := postgresqlSchemaTablePathRegex.MatchString(test.input)
			assert.Equal(t, test.matches, matches)
		})
	}
}

func TestParseVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected int64
		hasError bool
	}{
		{name: "up", input: "0000001-users.up.sql", expected: 1},
		{name: "repeatable up", input: "0000002-users.r.up.sql", expected: 2},
		{name: "notx up", input: "0000003-users.notx.up.sql", expected: 3},
		{name: "repeatable notx up", input: "0000004-users.rnotx.up.sql", expected: 4},
		{name: "down", input: "0000005-users.down.sql", expected: 5},
		{name: "invalid", input: "users.sql", hasError: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseVersion(tt.input)
			if tt.hasError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}
