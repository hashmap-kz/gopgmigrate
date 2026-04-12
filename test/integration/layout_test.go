package integration

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopgmigrate/internal/migration"
	"testing"
)

// integration/layout_test.go
func TestLayout_RecursiveSubdirectories(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	// files scattered across subdirs — version order must still be correct
	dir.Add(t, "schema/v1.0.0/0000001-create-users.up.sql",
		"create table users (id int primary key);")
	dir.Add(t, "data/v1.0.0/0000002-seed-users.up.sql",
		"insert into users values (1);")
	dir.Add(t, "functions/0000003-fn-get-users.r.up.sql",
		"create or replace function get_users() returns void language sql as $$ select 1; $$;")

	err := migration.RunMigrationsUp(context.Background(), &migration.ApplyOpts{
		MigrationDir:     dir.Root,
		ConnStr:          pg.ConnStr,
		HistoryTableName: "public.migrate_history",
	})
	require.NoError(t, err)

	hist := QueryHistory(t, pg.DB, "public.migrate_history")
	require.Len(t, hist, 3)
	assert.Equal(t, int64(1), hist[0].Version)
	assert.Equal(t, int64(2), hist[1].Version)
	assert.Equal(t, int64(3), hist[2].Version)
}

func TestLayout_DuplicateVersionRejected(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "schema/0000001-create-users.up.sql",
		"create table users (id int primary key);")
	dir.Add(t, "data/0000001-seed-users.up.sql", // duplicate revision
		"select 1;")

	err := migration.RunMigrationsUp(context.Background(), &migration.ApplyOpts{
		MigrationDir:     dir.Root,
		ConnStr:          pg.ConnStr,
		HistoryTableName: "public.migrate_history",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "0000001")
}
