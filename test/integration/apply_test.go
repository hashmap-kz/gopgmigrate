//go:build integration

package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gopgmigrate/internal/migration"
)

func TestApply_AppliesPendingInOrder(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "0000001-create-users.up.sql",
		"create table users (id int primary key);")
	dir.Add(t, "0000002-add-email.up.sql",
		"alter table users add column email text;")

	err := migration.RunMigrationsUp(context.Background(), &migration.ApplyOpts{
		MigrationDir:     dir.Root,
		ConnStr:          pg.ConnStr,
		HistoryTableName: "public.migrate_history",
	})
	require.NoError(t, err)

	hist := QueryHistory(t, pg.DB, "public.migrate_history")
	require.Len(t, hist, 2)
	assert.Equal(t, int64(1), hist[0].Version)
	assert.Equal(t, int64(2), hist[1].Version)
	assert.True(t, TableExists(t, pg.DB, "public", "users"))
}

func TestApply_Idempotent(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)
	dir.Add(t, "0000001-create-users.up.sql",
		"create table users (id int primary key);")

	opts := &migration.ApplyOpts{
		MigrationDir:     dir.Root,
		ConnStr:          pg.ConnStr,
		HistoryTableName: "public.migrate_history",
	}

	require.NoError(t, migration.RunMigrationsUp(context.Background(), opts))
	require.NoError(t, migration.RunMigrationsUp(context.Background(), opts))

	// second run must not duplicate history
	hist := QueryHistory(t, pg.DB, "public.migrate_history")
	assert.Len(t, hist, 1)
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
	dir.Add(t, "0000002-seed-users.up.sql", `
        insert into users (email) values ('alice@example.com'), ('bob@example.com');
    `)
	dir.Add(t, "0000003-fn-get-users.r.up.sql", `
        create or replace function get_users()
        returns setof users language sql as $$ select * from users; $$;
    `)

	before := TakeSnapshot(t, pg.DB)

	err := migration.RunMigrationsUp(context.Background(), &migration.ApplyOpts{
		MigrationDir:     dir.Root,
		ConnStr:          pg.ConnStr,
		HistoryTableName: "public.migrate_history",
	})
	require.NoError(t, err)

	after := TakeSnapshot(t, pg.DB)
	diff := Diff(before, after)

	// schema changes
	assert.Contains(t, diff.TablesAdded, "public.users",
		"users table should have been created\n%s", diff)
	assert.Contains(t, diff.FunctionsAdded, "public.get_users()",
		"get_users function should have been created\n%s", diff)

	//// data changes
	//td := diff.TableChanges["public.users"]
	//assert.Equal(t, 2, td.RowsAdded(),
	//	"two users should have been seeded\n%s", diff)

	// column structure
	usersTable := after.Tables["public.users"]
	cols := columnNames(usersTable.Columns)
	assert.Equal(t, []string{"id", "email"}, cols)

	// index created
	idxNames := indexNames(usersTable.Indexes)
	assert.Contains(t, idxNames, "idx_users_email")
}

func TestRollback_ReversesSchemaExactly(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "0000001-create-users.up.sql", `
        create table users (id bigint generated always as identity primary key, email text not null);
    `)
	dir.Add(t, "0000001-create-users.down.sql", `
        drop table users;
    `)
	dir.Add(t, "0000002-add-column.up.sql", `
        alter table users add column name text;
    `)
	dir.Add(t, "0000002-add-column.down.sql", `
        alter table users drop column name;
    `)

	opts := &migration.ApplyOpts{
		MigrationDir:     dir.Root,
		ConnStr:          pg.ConnStr,
		HistoryTableName: "public.migrate_history",
	}

	// apply both
	require.NoError(t, migration.RunMigrationsUp(context.Background(), opts))
	afterApply := TakeSnapshot(t, pg.DB)

	// columns should include name
	cols := columnNames(afterApply.Tables["public.users"].Columns)
	assert.Equal(t, []string{"id", "email", "name"}, cols)

	// rollback revision 2 only
	require.NoError(t, migration.RunMigrationsDown(context.Background(), &migration.RollbackOpts{
		MigrationDir:     dir.Root,
		ConnStr:          pg.ConnStr,
		HistoryTableName: "public.migrate_history",
		UndoCount:        1,
	}))

	afterRollback := TakeSnapshot(t, pg.DB)
	diff := Diff(afterApply, afterRollback)

	// name column dropped
	td := diff.TableChanges["public.users"]
	assert.Contains(t, td.ColumnsRemoved, "name",
		"name column should have been dropped\n%s", diff)
	assert.Empty(t, td.ColumnsAdded)

	// table still exists
	assert.NotContains(t, diff.TablesRemoved, "public.users")

	// history has one entry now
	hist := QueryHistory(t, pg.DB, "public.migrate_history")
	assert.Len(t, hist, 1)
	assert.Equal(t, int64(1), hist[0].Version)
}

func TestRollback_FullRollbackRestoresEmptySchema(t *testing.T) {
	t.Parallel()
	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	dir.Add(t, "0000001-create-users.up.sql",
		"create table users (id int primary key);")
	dir.Add(t, "0000001-create-users.down.sql",
		"drop table users;")
	dir.Add(t, "0000002-create-roles.up.sql",
		"create table roles (id int primary key);")
	dir.Add(t, "0000002-create-roles.down.sql",
		"drop table roles;")

	applyOpts := &migration.ApplyOpts{
		MigrationDir:     dir.Root,
		ConnStr:          pg.ConnStr,
		HistoryTableName: "public.migrate_history",
	}
	rollbackOpts := &migration.RollbackOpts{
		MigrationDir:     dir.Root,
		ConnStr:          pg.ConnStr,
		HistoryTableName: "public.migrate_history",
		UndoCount:        2,
	}

	baseline := TakeSnapshot(t, pg.DB)

	require.NoError(t, migration.RunMigrationsUp(context.Background(), applyOpts))
	require.NoError(t, migration.RunMigrationsDown(context.Background(), rollbackOpts))

	restored := TakeSnapshot(t, pg.DB)
	diff := Diff(baseline, restored)

	// history table exists (it is not rolled back - it is infrastructure)
	// but no user tables should remain
	userTables := userTablesOnly(restored.Tables)
	assert.Empty(t, userTables,
		"all user tables should be gone after full rollback\n%s", diff)

	assert.Empty(t, QueryHistory(t, pg.DB, "public.migrate_history"),
		"history should be empty after full rollback")
}

// perf

// TestApply_Performance_ManyFiles generates a large number of migration files
// and measures how long the full apply cycle takes. It is not a benchmark
// (no b.N loop) because real performance here is dominated by Postgres
// round-trips, not CPU - a benchmark would just repeat the same DB work N
// times without adding signal.
//
// What it measures:
//   - file discovery and sorting across many files
//   - history table reads and writes at scale
//   - overall apply latency for a realistic large project
//
// Run it explicitly when you want a perf signal:
//
//	go test -tags integration -run TestApply_Performance_ManyFiles -v ./integration/...
func TestApply_Performance_ManyFiles(t *testing.T) {
	t.Parallel()

	const (
		nVersioned  = 200 // plain DDL migrations
		nRepeatable = 50  // functions / views re-applied on change
		nNotx       = 10  // non-transactional (VACUUM-style)
	)
	total := nVersioned + nRepeatable + nNotx

	pg := NewPgDatabase(t)
	dir := NewMigrationDir(t)

	// --- generate versioned migrations ---
	// Each creates a small table so the SQL is not trivially empty.
	// Tables are named tN to avoid identifier length limits.
	for i := 1; i <= nVersioned; i++ {
		dir.Add(t,
			fmt.Sprintf("%07d-create-table-%04d.up.sql", i, i),
			fmt.Sprintf(
				"create table t%04d (id bigint generated always as identity primary key, v text);",
				i,
			),
		)
	}

	// --- generate repeatable migrations ---
	// Functions that reference the tables created above.
	base := nVersioned
	for i := 1; i <= nRepeatable; i++ {
		rev := base + i
		dir.Add(t,
			fmt.Sprintf("%07d-fn-get-%04d.r.up.sql", rev, i),
			fmt.Sprintf(
				"create or replace function fn_get_%04d() returns bigint language sql as $$ select count(*) from t%04d; $$;",
				i, i,
			),
		)
	}

	// --- generate non-transactional migrations ---
	// ANALYZE statements on the tables - safe no-ops in a test, but exercises
	// the notx execution path (statement-by-statement, outside BEGIN/COMMIT).
	base = nVersioned + nRepeatable
	for i := 1; i <= nNotx; i++ {
		rev := base + i
		dir.Add(t,
			fmt.Sprintf("%07d-analyze-%04d.notx.up.sql", rev, i),
			fmt.Sprintf("analyze t%04d;", i),
		)
	}

	opts := &migration.ApplyOpts{
		MigrationDir:     dir.Root,
		ConnStr:          pg.ConnStr,
		HistoryTableName: "public.migrate_history",
	}

	// --- first apply: everything is pending ---
	start := time.Now()
	require.NoError(t, migration.RunMigrationsUp(context.Background(), opts))
	firstApply := time.Since(start)

	hist := QueryHistory(t, pg.DB, "public.migrate_history")
	require.Len(t, hist, total,
		"expected %d history rows after first apply", total)

	// --- second apply: nothing is pending ---
	// This measures the cost of scanning a large history table and
	// resolving that all files are already applied. Should be fast.
	start = time.Now()
	require.NoError(t, migration.RunMigrationsUp(context.Background(), opts))
	secondApply := time.Since(start)

	hist = QueryHistory(t, pg.DB, "public.migrate_history")
	require.Len(t, hist, total,
		"history must not grow on a no-op second apply")

	t.Logf("files:        %d (%d versioned, %d repeatable, %d notx)",
		total, nVersioned, nRepeatable, nNotx)
	t.Logf("first apply:  %v  (%.1f ms/migration)",
		firstApply, float64(firstApply.Milliseconds())/float64(total))
	t.Logf("second apply: %v  (no-op, overhead only)",
		secondApply)

	// Soft thresholds - not hard limits, just a signal that something
	// regressed badly. Adjust to match your hardware and Postgres config.
	assert.Less(t, firstApply, 60*time.Second,
		"first apply of %d migrations took too long", total)
	assert.Less(t, secondApply, 5*time.Second,
		"no-op second apply of %d migrations took too long", total)
}

// helpers

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

func userTablesOnly(tables map[string]TableSnapshot) []string {
	var result []string
	for name := range tables {
		if !strings.HasPrefix(name, "public.migrate_") {
			result = append(result, name)
		}
	}
	return result
}
