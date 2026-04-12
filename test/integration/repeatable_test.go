package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopgmigrate/internal/migration"
)

// integration/repeatable_test.go
func TestRepeatable_ReappliedOnChange(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "0000001-fn-get-users.r.up.sql",
		"create or replace function get_users() returns void language sql as $$ select 1; $$;")

	opts := &migration.ApplyOpts{
		MigrationDir:     dir.Root,
		ConnStr:          pg.ConnStr,
		HistoryTableName: "public.migrate_history",
	}

	require.NoError(t, migration.RunMigrationsUp(context.Background(), opts))

	hist1 := QueryHistory(t, pg.DB, "public.migrate_history")
	require.Len(t, hist1, 1)
	hash1 := hist1[0].Hash

	// change the file content
	dir.Add(t, "0000001-fn-get-users.r.up.sql",
		"create or replace function get_users() returns void language sql as $$ select 2; $$;")

	require.NoError(t, migration.RunMigrationsUp(context.Background(), opts))

	hist2 := QueryHistory(t, pg.DB, "public.migrate_history")
	require.Len(t, hist2, 1)
	assert.NotEqual(t, hash1, hist2[0].Hash) // hash updated
}

func TestRepeatable_SkippedWhenUnchanged(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "0000001-fn-get-users.r.up.sql",
		"create or replace function get_users() returns void language sql as $$ select 1; $$;")

	opts := &migration.ApplyOpts{
		MigrationDir:     dir.Root,
		ConnStr:          pg.ConnStr,
		HistoryTableName: "public.migrate_history",
	}

	require.NoError(t, migration.RunMigrationsUp(context.Background(), opts))
	hist1 := QueryHistory(t, pg.DB, "public.migrate_history")

	require.NoError(t, migration.RunMigrationsUp(context.Background(), opts))
	hist2 := QueryHistory(t, pg.DB, "public.migrate_history")

	assert.Equal(t, hist1[0].Hash, hist2[0].Hash)
}
