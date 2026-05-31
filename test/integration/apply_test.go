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

	dir.Add(t, "0000001-create-users.up.sql", "create table users (id int primary key);")
	dir.Add(t, "0000002-add-email.up.sql", "alter table users add column email text;")

	m, err := migrator.NewWithDSN(pg.ConnStr, migrator.Config{Dir: dir.Root, Table: histTable})
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

	dir.Add(t, "0000001-create-users.up.sql", "create table users (id int primary key);")

	cfg := migrator.Config{Dir: dir.Root, Table: histTable}
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

func TestApply_NoTxMode(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "0000001-create-users.up.sql", "create table users (id int primary key);")
	dir.Add(t, "0000002-vacuum.notx.sql", "vacuum users;")

	m, err := migrator.NewWithDSN(pg.ConnStr, migrator.Config{Dir: dir.Root, Table: histTable})
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

func TestApply_RepeatableNoTxMode(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "0000001-create-users.up.sql", "create table users (id int primary key);")
	dir.Add(t, "0000002-view.rnotx.sql",
		"create or replace view v_users as select id from users;")

	cfg := migrator.Config{Dir: dir.Root, Table: histTable}

	run := func() {
		m, err := migrator.NewWithDSN(pg.ConnStr, cfg)
		require.NoError(t, err)
		defer m.Close()
		require.NoError(t, m.Run(context.Background()))
	}

	run()
	hist1 := QueryHistory(t, pg.DB, histTable)
	require.Len(t, hist1, 2)
	checksum1 := hist1[1].Checksum
	assert.Equal(t, "repeatable-notx", hist1[1].Kind)

	dir.Add(t, "0000002-view.rnotx.sql",
		"create or replace view v_users as select id, id*2 as id2 from users;")

	run()
	hist2 := QueryHistory(t, pg.DB, histTable)
	require.Len(t, hist2, 2)
	assert.NotEqual(t, checksum1, hist2[1].Checksum, "checksum must update after re-apply")
}

func TestApply_DryRun_DoesNotApply(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "0000001-create-users.up.sql", "create table users (id int primary key);")

	m, err := migrator.NewWithDSN(pg.ConnStr, migrator.Config{
		Dir:    dir.Root,
		Table:  histTable,
		DryRun: true,
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

	dir.Add(t, "0000001-create-users.up.sql", `
		create table users (
			id    bigint generated always as identity primary key,
			email text not null unique
		);
		create index idx_users_email on users(email);
	`)
	dir.Add(t, "0000002-seed-users.up.sql",
		"insert into users (email) values ('alice@example.com'), ('bob@example.com');")
	dir.Add(t, "0000003-fn-get-users.r.sql",
		"create or replace function get_users() returns setof users language sql as $$ select * from users; $$;")

	before := TakeSnapshot(t, pg.DB)

	m, err := migrator.NewWithDSN(pg.ConnStr, migrator.Config{Dir: dir.Root, Table: histTable})
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

func TestApply_NoTxMode_Idempotent(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "0000001-create-users.up.sql", "create table users (id int primary key);")
	dir.Add(t, "0000002-vacuum.notx.sql", "vacuum users;")

	cfg := migrator.Config{Dir: dir.Root, Table: histTable}
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

	dir.Add(t, "0000001-create-tables.up.sql", `
		create table users (id int primary key);
		create table roles (id int primary key);
	`)
	dir.Add(t, "0000002-vacuum-users.notx.sql", "vacuum users;")
	dir.Add(t, "0000003-vacuum-roles.notx.sql", "vacuum roles;")

	m, err := migrator.NewWithDSN(pg.ConnStr, migrator.Config{Dir: dir.Root, Table: histTable})
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

	dir.Add(t, "0000001-create-users.up.sql", "create table users (id int primary key);")

	const overrideTable = "custom_migrations"

	m, err := migrator.NewWithDSN(pg.ConnStr, migrator.Config{
		Dir:   dir.Root,
		Table: overrideTable,
	})
	require.NoError(t, err)
	defer m.Close()
	require.NoError(t, m.Run(context.Background()))

	assert.True(t, TableExists(t, pg.DB, "public", overrideTable),
		"history table must exist under the override name")
	assert.Len(t, QueryHistory(t, pg.DB, overrideTable), 1)
	assert.False(t, TableExists(t, pg.DB, "public", histTable),
		"default table name must not be used when Config.Table is set")
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
