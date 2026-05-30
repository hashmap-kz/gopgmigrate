//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/hashmap-kz/gopgmigrate/v2/pkg/migrator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const histTable = "schema_migrations"

func TestApply_DefaultMode_AppliesInOrder(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_create_users.sql", "create table users (id int primary key);")
	dir.Add(t, "002_add_email.sql", "alter table users add column email text;")

	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql"}},
		{Files: []string{"002_add_email.sql"}},
	})

	m, err := migrator.NewWithDSN(pg.ConnStr, migrator.Config{ManifestPath: manifest, Table: histTable})
	require.NoError(t, err)
	defer m.Close()
	require.NoError(t, m.Run(context.Background()))

	hist := QueryHistory(t, pg.DB, histTable)
	require.Len(t, hist, 2)
	assert.Equal(t, "once", hist[0].Kind)
	assert.Equal(t, "once", hist[1].Kind)
	assert.True(t, TableExists(t, pg.DB, "public", "users"))
}

func TestApply_Idempotent(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_create_users.sql", "create table users (id int primary key);")

	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql"}},
	})
	cfg := migrator.Config{ManifestPath: manifest, Table: histTable}

	run := func() {
		m, err := migrator.NewWithDSN(pg.ConnStr, cfg)
		require.NoError(t, err)
		defer m.Close()
		require.NoError(t, m.Run(context.Background()))
	}
	run()
	run()

	assert.Len(t, QueryHistory(t, pg.DB, histTable), 1)
}

func TestApply_AtomicMode_AllFilesInOneTx(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_create_users.sql", "create table users (id int primary key);")
	dir.Add(t, "002_create_roles.sql", "create table roles (id int primary key, name text);")

	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql", "002_create_roles.sql"}, Mode: "atomic"},
	})

	m, err := migrator.NewWithDSN(pg.ConnStr, migrator.Config{ManifestPath: manifest, Table: histTable})
	require.NoError(t, err)
	defer m.Close()
	require.NoError(t, m.Run(context.Background()))

	hist := QueryHistory(t, pg.DB, histTable)
	require.Len(t, hist, 2)
	assert.Equal(t, "once", hist[0].Kind)
	assert.Equal(t, "once", hist[1].Kind)
	assert.True(t, TableExists(t, pg.DB, "public", "users"))
	assert.True(t, TableExists(t, pg.DB, "public", "roles"))
}

func TestApply_AtomicMode_RollsBackOnFailure(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_create_users.sql", "create table users (id int primary key);")
	dir.Add(t, "002_bad.sql", "this is not valid sql;")

	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql", "002_bad.sql"}, Mode: "atomic"},
	})

	m, err := migrator.NewWithDSN(pg.ConnStr, migrator.Config{ManifestPath: manifest, Table: histTable})
	require.NoError(t, err)
	defer m.Close()

	err = m.Run(context.Background())
	require.Error(t, err)

	// both files were in one transaction - the whole batch must have rolled back
	assert.False(t, TableExists(t, pg.DB, "public", "users"))
	assert.Empty(t, QueryHistory(t, pg.DB, histTable))
}

func TestApply_NoTxMode(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	// VACUUM cannot run inside a transaction, so it requires no-tx mode
	dir.Add(t, "001_create_users.sql", "create table users (id int primary key);")
	dir.Add(t, "002_vacuum.sql", "vacuum users;")

	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql"}},
		{Files: []string{"002_vacuum.sql"}, Mode: "no-tx"},
	})

	m, err := migrator.NewWithDSN(pg.ConnStr, migrator.Config{ManifestPath: manifest, Table: histTable})
	require.NoError(t, err)
	defer m.Close()
	require.NoError(t, m.Run(context.Background()))

	hist := QueryHistory(t, pg.DB, histTable)
	require.Len(t, hist, 2)

	var noTxFound bool
	for _, r := range hist {
		if r.Kind == "no-tx" {
			noTxFound = true
		}
	}
	assert.True(t, noTxFound, "expected a no-tx history row")
}

func TestApply_DryRun_DoesNotApply(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_create_users.sql", "create table users (id int primary key);")

	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql"}},
	})

	m, err := migrator.NewWithDSN(pg.ConnStr, migrator.Config{
		ManifestPath: manifest,
		Table:        histTable,
		DryRun:       true,
	})
	require.NoError(t, err)
	defer m.Close()
	require.NoError(t, m.Run(context.Background()))

	assert.False(t, TableExists(t, pg.DB, "public", "users"))
	assert.Empty(t, QueryHistory(t, pg.DB, histTable))
}

func TestApply_DDLChangesReflectedInSchema(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_create_users.sql", `
		create table users (
			id    bigint generated always as identity primary key,
			email text not null unique
		);
		create index idx_users_email on users(email);
	`)
	dir.Add(t, "002_seed_users.sql",
		"insert into users (email) values ('alice@example.com'), ('bob@example.com');")
	dir.Add(t, "003_fn_get_users.sql",
		"create or replace function get_users() returns setof users language sql as $$ select * from users; $$;")

	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql"}},
		{Files: []string{"002_seed_users.sql"}},
		{Files: []string{"003_fn_get_users.sql"}},
	})

	before := TakeSnapshot(t, pg.DB)

	m, err := migrator.NewWithDSN(pg.ConnStr, migrator.Config{ManifestPath: manifest, Table: histTable})
	require.NoError(t, err)
	defer m.Close()
	require.NoError(t, m.Run(context.Background()))

	after := TakeSnapshot(t, pg.DB)
	diff := Diff(before, after)

	assert.Contains(t, diff.TablesAdded, "public.users",
		"users table should have been created\n%s", diff)
	assert.Contains(t, diff.FunctionsAdded, "public.get_users()",
		"get_users function should have been created\n%s", diff)

	usersTable := after.Tables["public.users"]
	assert.Equal(t, []string{"id", "email"}, columnNames(usersTable.Columns))
	assert.Contains(t, indexNames(usersTable.Indexes), "idx_users_email")
}

func TestApply_AtomicMode_Idempotent(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_create_users.sql", "create table users (id int primary key);")
	dir.Add(t, "002_create_roles.sql", "create table roles (id int primary key);")

	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql", "002_create_roles.sql"}, Mode: "atomic"},
	})
	cfg := migrator.Config{ManifestPath: manifest, Table: histTable}

	run := func() {
		m, err := migrator.NewWithDSN(pg.ConnStr, cfg)
		require.NoError(t, err)
		defer m.Close()
		require.NoError(t, m.Run(context.Background()))
	}
	run()
	run()

	assert.Len(t, QueryHistory(t, pg.DB, histTable), 2)
}

func TestApply_AtomicMode_PartialApplyError(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_create_users.sql", "create table users (id int primary key);")
	dir.Add(t, "002_create_roles.sql", "create table roles (id int primary key);")

	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql", "002_create_roles.sql"}, Mode: "atomic"},
	})
	cfg := migrator.Config{ManifestPath: manifest, Table: histTable}

	// apply successfully so the history table exists with both rows
	m1, err := migrator.NewWithDSN(pg.ConnStr, cfg)
	require.NoError(t, err)
	defer m1.Close()
	require.NoError(t, m1.Run(context.Background()))

	hist := QueryHistory(t, pg.DB, histTable)
	require.Len(t, hist, 2)

	// remove one row to simulate a partially-applied atomic batch
	_, err = pg.DB.ExecContext(context.Background(),
		"delete from "+histTable+" where path = $1", hist[1].Path)
	require.NoError(t, err)

	// next run must fail: 1 of 2 files recorded is a partial-apply state
	m2, err := migrator.NewWithDSN(pg.ConnStr, cfg)
	require.NoError(t, err)
	defer m2.Close()
	err = m2.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "partially applied")
}

func TestApply_NoTxMode_Idempotent(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_create_users.sql", "create table users (id int primary key);")
	dir.Add(t, "002_vacuum.sql", "vacuum users;")

	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql"}},
		{Files: []string{"002_vacuum.sql"}, Mode: "no-tx"},
	})
	cfg := migrator.Config{ManifestPath: manifest, Table: histTable}

	run := func() {
		m, err := migrator.NewWithDSN(pg.ConnStr, cfg)
		require.NoError(t, err)
		defer m.Close()
		require.NoError(t, m.Run(context.Background()))
	}
	run()
	run()

	assert.Len(t, QueryHistory(t, pg.DB, histTable), 2)
}

func TestApply_NoTxMode_MultipleFiles(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_create_tables.sql", `
		create table users (id int primary key);
		create table roles (id int primary key);
	`)
	dir.Add(t, "002_vacuum_users.sql", "vacuum users;")
	dir.Add(t, "003_vacuum_roles.sql", "vacuum roles;")

	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_tables.sql"}},
		{Files: []string{"002_vacuum_users.sql", "003_vacuum_roles.sql"}, Mode: "no-tx"},
	})

	m, err := migrator.NewWithDSN(pg.ConnStr, migrator.Config{ManifestPath: manifest, Table: histTable})
	require.NoError(t, err)
	defer m.Close()
	require.NoError(t, m.Run(context.Background()))

	hist := QueryHistory(t, pg.DB, histTable)
	require.Len(t, hist, 3)

	var noTxCount int
	for _, r := range hist {
		if r.Kind == "no-tx" {
			noTxCount++
		}
	}
	assert.Equal(t, 2, noTxCount, "both no-tx files must be recorded with kind=no-tx")
}

func TestApply_ConfigTableOverride(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "001_create_users.sql", "create table users (id int primary key);")

	manifest := dir.WriteManifest(t, histTable, []ManifestEntry{
		{Files: []string{"001_create_users.sql"}},
	})

	const overrideTable = "custom_migrations"

	m, err := migrator.NewWithDSN(pg.ConnStr, migrator.Config{
		ManifestPath: manifest,
		Table:        overrideTable,
	})
	require.NoError(t, err)
	defer m.Close()
	require.NoError(t, m.Run(context.Background()))

	assert.True(t, TableExists(t, pg.DB, "public", overrideTable),
		"history table must exist under the override name")
	assert.Len(t, QueryHistory(t, pg.DB, overrideTable), 1)
	assert.False(t, TableExists(t, pg.DB, "public", histTable),
		"manifest table name must not be used when Config.Table is set")
}

func columnNames(cols []ColumnSnapshot) []string {
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.Name
	}
	return names
}

func indexNames(idxs []IndexSnapshot) []string {
	names := make([]string, len(idxs))
	for i, idx := range idxs {
		names[i] = idx.Name
	}
	return names
}
