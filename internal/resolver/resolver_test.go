package resolver

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gopgmigrate/internal/naming"
)

func TestGetFiles(t *testing.T) {
	t.Parallel()

	t.Run("loads valid migration files and sorts by base name", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		writeTestFile(t, dir, "0000002-users.r.up.sql", "select 2;")
		writeTestFile(t, dir, "0000001-init.up.sql", "select 1;")
		writeTestFile(t, dir, "0000003-vacuum.notx.up.sql", "vacuum;")
		writeTestFile(t, dir, "0000004-users.down.sql", "drop table users;")

		files, err := GetFiles(dir, naming.MigrationRegex(), nil)
		require.NoError(t, err)

		require.Len(t, files, 4)
		assert.Equal(t, "0000001-init.up.sql", files[0].Base)
		assert.Equal(t, "0000002-users.r.up.sql", files[1].Base)
		assert.Equal(t, "0000003-vacuum.notx.up.sql", files[2].Base)
		assert.Equal(t, "0000004-users.down.sql", files[3].Base)

		assert.EqualValues(t, 1, files[0].Vers)
		assert.EqualValues(t, 2, files[1].Vers)
		assert.EqualValues(t, 3, files[2].Vers)
		assert.EqualValues(t, 4, files[3].Vers)

		assert.NotEmpty(t, files[0].Hash)
		assert.Equal(t, []byte("select 1;"), files[0].Data)
	})

	t.Run("filters only files matching provided regex", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		writeTestFile(t, dir, "0000001-init.up.sql", "select 1;")
		writeTestFile(t, dir, "0000002-users.r.up.sql", "select 2;")
		writeTestFile(t, dir, "0000003-vacuum.notx.up.sql", "vacuum;")
		writeTestFile(t, dir, "0000004-users.down.sql", "drop table users;")

		files, err := GetFiles(dir, naming.MigrationRegex(), nil)
		require.NoError(t, err)
		require.Len(t, files, 4)

		downOnly := regexp.MustCompile(`^(\d{7})-([^/]+?)\.(down)\.sql$`)
		files, err = GetFiles(dir, downOnly, nil)
		require.NoError(t, err)

		require.Len(t, files, 1)
		assert.Equal(t, "0000004-users.down.sql", files[0].Base)
		assert.EqualValues(t, 4, files[0].Vers)
	})

	t.Run("returns error when directory contains stray file", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		writeTestFile(t, dir, "0000001-init.up.sql", "select 1;")
		writeTestFile(t, dir, "notes.txt", "hello")

		files, err := GetFiles(dir, naming.MigrationRegex(), nil)
		require.Error(t, err)
		assert.Nil(t, files)
		assert.Contains(t, err.Error(), "stray files are not allowed")
	})

	t.Run("returns error when invalid sql filename exists", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		writeTestFile(t, dir, "0000001-init.up.sql", "select 1;")
		writeTestFile(t, dir, "00001-bad.do.sql", "old naming")

		files, err := GetFiles(dir, naming.MigrationRegex(), nil)
		require.Error(t, err)
		assert.Nil(t, files)
		assert.Contains(t, err.Error(), "stray files are not allowed")
	})

	t.Run("walks nested directories", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		sub := filepath.Join(dir, "nested", "deeper")
		require.NoError(t, os.MkdirAll(sub, 0o755))

		writeTestFile(t, dir, "0000002-users.r.up.sql", "select 2;")
		writeTestFile(t, sub, "0000001-init.up.sql", "select 1;")

		files, err := GetFiles(dir, naming.MigrationRegex(), nil)
		require.NoError(t, err)

		require.Len(t, files, 2)
		assert.Equal(t, "0000001-init.up.sql", files[0].Base)
		assert.Equal(t, "0000002-users.r.up.sql", files[1].Base)
	})

	t.Run("fails when tx file contains no-tx pattern", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		writeTestFile(t, dir, "0000001-vacuum.up.sql", "VACUUM FULL users;")

		noTxPatterns := map[string]*regexp.Regexp{
			"vacuum": regexp.MustCompile(`(?i)\bVACUUM\b`),
		}

		files, err := GetFiles(dir, naming.MigrationRegex(), noTxPatterns)
		require.Error(t, err)
		assert.Nil(t, files)
		assert.Contains(t, err.Error(), "check statements in the file")
	})

	t.Run("allows notx file to contain no-tx pattern", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		writeTestFile(t, dir, "0000001-vacuum.notx.up.sql", "VACUUM FULL users;")

		noTxPatterns := map[string]*regexp.Regexp{
			"vacuum": regexp.MustCompile(`(?i)\bVACUUM\b`),
		}

		files, err := GetFiles(dir, naming.MigrationRegex(), noTxPatterns)
		require.NoError(t, err)

		require.Len(t, files, 1)
		assert.Equal(t, "0000001-vacuum.notx.up.sql", files[0].Base)
		assert.EqualValues(t, 1, files[0].Vers)
	})

	t.Run("allows rnotx file to contain no-tx pattern", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		writeTestFile(t, dir, "0000001-refresh.rnotx.up.sql", "VACUUM FULL users;")

		noTxPatterns := map[string]*regexp.Regexp{
			"vacuum": regexp.MustCompile(`(?i)\bVACUUM\b`),
		}

		files, err := GetFiles(dir, naming.MigrationRegex(), noTxPatterns)
		require.NoError(t, err)

		require.Len(t, files, 1)
		assert.Equal(t, "0000001-refresh.rnotx.up.sql", files[0].Base)
	})

	t.Run("allows tx file when no no-tx pattern matches", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		writeTestFile(t, dir, "0000001-init.up.sql", "create table users(id int);")

		noTxPatterns := map[string]*regexp.Regexp{
			"vacuum": regexp.MustCompile(`(?i)\bVACUUM\b`),
		}

		files, err := GetFiles(dir, naming.MigrationRegex(), noTxPatterns)
		require.NoError(t, err)

		require.Len(t, files, 1)
		assert.Equal(t, "0000001-init.up.sql", files[0].Base)
	})

	t.Run("supports empty directory", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		files, err := GetFiles(dir, naming.MigrationRegex(), nil)
		require.NoError(t, err)
		assert.Empty(t, files)
	})
}

func TestGetAllStrayFiles(t *testing.T) {
	t.Parallel()

	t.Run("returns only stray files", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		writeTestFile(t, dir, "0000001-init.up.sql", "select 1;")
		writeTestFile(t, dir, "README.md", "docs")
		writeTestFile(t, dir, "broken.sql", "select 2;")
		writeTestFile(t, dir, "00001-old.do.sql", "select 3;")

		got, err := getAllStrayFiles(dir)
		require.NoError(t, err)

		assert.Len(t, got, 3)
		assert.Contains(t, got, filepath.ToSlash(filepath.Join(dir, "README.md")))
		assert.Contains(t, got, filepath.ToSlash(filepath.Join(dir, "broken.sql")))
		assert.Contains(t, got, filepath.ToSlash(filepath.Join(dir, "00001-old.do.sql")))
	})
}

func TestCheckThatFileIsPossibleShouldNotUseTx(t *testing.T) {
	t.Parallel()

	t.Run("returns warnings for matching patterns", func(t *testing.T) {
		t.Parallel()

		sql := `
VACUUM FULL users;
CREATE INDEX CONCURRENTLY idx_users_name ON users(name);
`
		patterns := map[string]*regexp.Regexp{
			"vacuum":            regexp.MustCompile(`(?i)\bVACUUM\b`),
			"create_concurrent": regexp.MustCompile(`(?i)\bCREATE\s+INDEX\s+CONCURRENTLY\b`),
			"non_match_example": regexp.MustCompile(`(?i)\bALTER\s+SYSTEM\b`),
		}

		got := checkThatFileIsPossibleShouldNotUseTx(sql, patterns)

		assert.Len(t, got, 2)
		assert.Contains(t, got, "Warning: Detected vacuum pattern")
		assert.Contains(t, got, "Warning: Detected create_concurrent pattern")
	})

	t.Run("returns nil when nothing matches", func(t *testing.T) {
		t.Parallel()

		sql := `create table users(id int);`
		patterns := map[string]*regexp.Regexp{
			"vacuum": regexp.MustCompile(`(?i)\bVACUUM\b`),
		}

		got := checkThatFileIsPossibleShouldNotUseTx(sql, patterns)
		assert.Empty(t, got)
	})
}

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	require.NoError(t, err)

	err = os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err)

	return path
}
