//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/hashmap-kz/gopgmigrate/v2/pkg/migrator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepeatable_ReappliedOnChange(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_fn_get_users.sql",
		"create or replace function get_users() returns void language sql as $$ select 1; $$;")

	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_fn_get_users.sql"}, Mode: "repeatable"},
	})
	cfg := migrator.Config{ManifestPath: manifest, Table: histTable}

	run := func() {
		m, err := migrator.NewWithDSN(pg.ConnStr, cfg)
		require.NoError(t, err)
		defer m.Close()
		require.NoError(t, m.Run(context.Background()))
	}

	run()
	hist1 := QueryHistory(t, pg.DB, histTable)
	require.Len(t, hist1, 1)
	checksum1 := hist1[0].Checksum

	// change file content — must trigger a re-apply
	dir.Add(t, "001_fn_get_users.sql",
		"create or replace function get_users() returns void language sql as $$ select 2; $$;")

	run()
	hist2 := QueryHistory(t, pg.DB, histTable)
	require.Len(t, hist2, 1)
	assert.NotEqual(t, checksum1, hist2[0].Checksum, "checksum must be updated after re-apply")
	assert.Equal(t, "repeatable", hist2[0].Kind)
}

func TestRepeatable_SkippedWhenUnchanged(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_fn_get_users.sql",
		"create or replace function get_users() returns void language sql as $$ select 1; $$;")

	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_fn_get_users.sql"}, Mode: "repeatable"},
	})
	cfg := migrator.Config{ManifestPath: manifest, Table: histTable}

	run := func() {
		m, err := migrator.NewWithDSN(pg.ConnStr, cfg)
		require.NoError(t, err)
		defer m.Close()
		require.NoError(t, m.Run(context.Background()))
	}

	run()
	hist1 := QueryHistory(t, pg.DB, histTable)
	require.Len(t, hist1, 1)

	run()
	hist2 := QueryHistory(t, pg.DB, histTable)
	require.Len(t, hist2, 1)
	assert.Equal(t, hist1[0].Checksum, hist2[0].Checksum, "checksum must not change when file is unchanged")
}
