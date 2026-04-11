package filters

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
	"gopgmigrate/internal/naming"
)

func TestFilterMigrationFiles(t *testing.T) {
	t.Parallel()

	files := []naming.MigrationFile{
		{Vers: 1, Base: "0000001-init.up.sql", Path: "/migrations/0000001-init.up.sql"},
		{Vers: 2, Base: "0000002-users.r.up.sql", Path: "/migrations/0000002-users.r.up.sql"},
		{Vers: 3, Base: "0000003-vacuum.notx.up.sql", Path: "/migrations/0000003-vacuum.notx.up.sql"},
		{Vers: 4, Base: "0000004-users.down.sql", Path: "/migrations/0000004-users.down.sql"},
	}

	t.Run("keeps only matching files and preserves order", func(t *testing.T) {
		t.Parallel()

		got := filterMigrationFiles(files, func(f naming.MigrationFile) bool {
			return naming.IsVersioned(f.Base)
		})

		expected := []naming.MigrationFile{
			files[0],
			files[1],
			files[2],
		}

		assert.Equal(t, expected, got)
	})

	t.Run("returns empty slice when nothing matches", func(t *testing.T) {
		t.Parallel()

		got := filterMigrationFiles(files, func(f naming.MigrationFile) bool {
			return false
		})

		assert.Empty(t, got)
		assert.NotNil(t, got)
	})

	t.Run("returns all files when everything matches", func(t *testing.T) {
		t.Parallel()

		got := filterMigrationFiles(files, func(f naming.MigrationFile) bool {
			return true
		})

		assert.Equal(t, files, got)
	})

	t.Run("works with nil input", func(t *testing.T) {
		t.Parallel()

		var filesNil []naming.MigrationFile

		got := filterMigrationFiles(filesNil, func(f naming.MigrationFile) bool {
			return true
		})

		assert.Empty(t, got)
		assert.NotNil(t, got)
	})

	t.Run("works with empty input", func(t *testing.T) {
		t.Parallel()

		got := filterMigrationFiles([]naming.MigrationFile{}, func(f naming.MigrationFile) bool {
			return true
		})

		assert.Empty(t, got)
		assert.NotNil(t, got)
	})

	t.Run("does not mutate input slice", func(t *testing.T) {
		t.Parallel()

		original := append([]naming.MigrationFile(nil), files...)

		_ = filterMigrationFiles(files, func(f naming.MigrationFile) bool {
			return naming.IsDown(f.Base)
		})

		assert.Equal(t, original, files)
	})

	t.Run("can filter only rollback files", func(t *testing.T) {
		t.Parallel()

		got := filterMigrationFiles(files, func(f naming.MigrationFile) bool {
			return naming.IsDown(f.Base)
		})

		expected := []naming.MigrationFile{
			files[3],
		}

		assert.Equal(t, expected, got)
	})

	t.Run("can filter only non transactional files", func(t *testing.T) {
		t.Parallel()

		got := filterMigrationFiles(files, func(f naming.MigrationFile) bool {
			return naming.IsNonTransaction(f.Base)
		})

		expected := []naming.MigrationFile{
			files[2],
		}

		assert.Equal(t, expected, got)
	})
}

func TestCheckFilesAreUniqueByVersion(t *testing.T) {
	t.Parallel()

	t.Run("passes when versions are unique", func(t *testing.T) {
		t.Parallel()

		files := []naming.MigrationFile{
			{Vers: 1, Path: "/m/0000001-init.up.sql", Base: "0000001-init.up.sql"},
			{Vers: 2, Path: "/m/0000002-users.up.sql", Base: "0000002-users.up.sql"},
		}

		err := checkFilesAreUniqueByVersion(files)
		assert.NoError(t, err)
	})

	t.Run("fails when versions are duplicated", func(t *testing.T) {
		t.Parallel()

		files := []naming.MigrationFile{
			{Vers: 1, Path: "/m/0000001-init.up.sql", Base: "0000001-init.up.sql"},
			{Vers: 1, Path: "/m/0000001-users.down.sql", Base: "0000001-users.down.sql"},
		}

		err := checkFilesAreUniqueByVersion(files)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already in use")
	})
}
