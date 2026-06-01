//go:build integration

package integration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

// DBSnapshot captures the structural and data state of a Postgres database.
type DBSnapshot struct {
	Tables    map[string]TableSnapshot
	Functions map[string]FunctionSnapshot
	Views     map[string]string // name -> definition
}

type TableSnapshot struct {
	Columns     []ColumnSnapshot
	Constraints []ConstraintSnapshot
	Indexes     []IndexSnapshot
	RowCount    int
}

type ColumnSnapshot struct {
	Name     string
	Type     string
	Nullable bool
	Default  sql.NullString
	Position int
}

type ConstraintSnapshot struct {
	Name       string
	Type       string // p=primary, u=unique, f=foreign, c=check
	Definition string
}

type IndexSnapshot struct {
	Name       string
	Unique     bool
	Definition string
}

type FunctionSnapshot struct {
	Schema     string
	Name       string
	ArgTypes   string
	ReturnType string
	BodyHash   string // sha256 of the function body
}

// TakeSnapshot captures the full schema state visible to the current user.
func TakeSnapshot(t *testing.T, db *sql.DB) *DBSnapshot {
	t.Helper()
	ctx := context.Background()

	snap := &DBSnapshot{
		Tables:    make(map[string]TableSnapshot),
		Functions: make(map[string]FunctionSnapshot),
		Views:     make(map[string]string),
	}

	snapshotTables(t, ctx, db, snap)
	snapshotFunctions(t, ctx, db, snap)
	snapshotViews(t, ctx, db, snap)

	return snap
}

func snapshotTables(t *testing.T, ctx context.Context, db *sql.DB, snap *DBSnapshot) {
	t.Helper()

	// list all user tables
	rows, err := db.QueryContext(ctx, `
        select schemaname || '.' || tablename
        from pg_tables
        where schemaname not in ('pg_catalog', 'information_schema')
        order by 1
    `)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table name: %v", err)
		}
		tables = append(tables, name)
	}

	for _, table := range tables {
		snap.Tables[table] = TableSnapshot{
			Columns:     snapshotColumns(t, ctx, db, table),
			Constraints: snapshotConstraints(t, ctx, db, table),
			Indexes:     snapshotIndexes(t, ctx, db, table),
			RowCount:    rowCount(t, ctx, db, table),
		}
	}
}

func snapshotColumns(t *testing.T, ctx context.Context, db *sql.DB, table string) []ColumnSnapshot {
	t.Helper()
	parts := strings.SplitN(table, ".", 2)

	rows, err := db.QueryContext(ctx, `
        select
            column_name,
            data_type,
            is_nullable = 'YES',
            column_default,
            ordinal_position
        from information_schema.columns
        where table_schema = $1 and table_name = $2
        order by ordinal_position
    `, parts[0], parts[1])
	if err != nil {
		t.Fatalf("snapshot columns for %s: %v", table, err)
	}
	defer rows.Close()

	var cols []ColumnSnapshot
	for rows.Next() {
		var c ColumnSnapshot
		if err := rows.Scan(&c.Name, &c.Type, &c.Nullable, &c.Default, &c.Position); err != nil {
			t.Fatalf("scan column: %v", err)
		}
		cols = append(cols, c)
	}
	return cols
}

func snapshotConstraints(t *testing.T, ctx context.Context, db *sql.DB, table string) []ConstraintSnapshot {
	t.Helper()
	parts := strings.SplitN(table, ".", 2)

	rows, err := db.QueryContext(ctx, `
        select
            c.conname,
            c.contype::text,
            pg_get_constraintdef(c.oid)
        from pg_constraint c
        join pg_class cl on cl.oid = c.conrelid
        join pg_namespace n on n.oid = cl.relnamespace
        where n.nspname = $1 and cl.relname = $2
        order by c.conname
    `, parts[0], parts[1])
	if err != nil {
		t.Fatalf("snapshot constraints for %s: %v", table, err)
	}
	defer rows.Close()

	var cs []ConstraintSnapshot
	for rows.Next() {
		var c ConstraintSnapshot
		if err := rows.Scan(&c.Name, &c.Type, &c.Definition); err != nil {
			t.Fatalf("scan constraint: %v", err)
		}
		cs = append(cs, c)
	}
	return cs
}

func snapshotIndexes(t *testing.T, ctx context.Context, db *sql.DB, table string) []IndexSnapshot {
	t.Helper()
	parts := strings.SplitN(table, ".", 2)

	rows, err := db.QueryContext(ctx, `
        select
            i.relname,
            ix.indisunique,
            pg_get_indexdef(ix.indexrelid)
        from pg_index ix
        join pg_class t on t.oid = ix.indrelid
        join pg_class i on i.oid = ix.indexrelid
        join pg_namespace n on n.oid = t.relnamespace
        where n.nspname = $1 and t.relname = $2
        order by i.relname
    `, parts[0], parts[1])
	if err != nil {
		t.Fatalf("snapshot indexes for %s: %v", table, err)
	}
	defer rows.Close()

	var idxs []IndexSnapshot
	for rows.Next() {
		var idx IndexSnapshot
		if err := rows.Scan(&idx.Name, &idx.Unique, &idx.Definition); err != nil {
			t.Fatalf("scan index: %v", err)
		}
		idxs = append(idxs, idx)
	}
	return idxs
}

func snapshotFunctions(t *testing.T, ctx context.Context, db *sql.DB, snap *DBSnapshot) {
	t.Helper()

	rows, err := db.QueryContext(ctx, `
        select
            n.nspname,
            p.proname,
            pg_get_function_identity_arguments(p.oid),
            pg_get_function_result(p.oid),
            p.prosrc
        from pg_proc p
        join pg_namespace n on n.oid = p.pronamespace
        where n.nspname not in ('pg_catalog', 'information_schema')
        order by n.nspname, p.proname
    `)
	if err != nil {
		t.Fatalf("snapshot functions: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var f FunctionSnapshot
		var body string
		if err := rows.Scan(&f.Schema, &f.Name, &f.ArgTypes, &f.ReturnType, &body); err != nil {
			t.Fatalf("scan function: %v", err)
		}
		h := sha256.Sum256([]byte(body))
		f.BodyHash = hex.EncodeToString(h[:])
		key := fmt.Sprintf("%s.%s(%s)", f.Schema, f.Name, f.ArgTypes)
		snap.Functions[key] = f
	}
}

func snapshotViews(t *testing.T, ctx context.Context, db *sql.DB, snap *DBSnapshot) {
	t.Helper()

	rows, err := db.QueryContext(ctx, `
        select
            schemaname || '.' || viewname,
            definition
        from pg_views
        where schemaname not in ('pg_catalog', 'information_schema')
        order by 1
    `)
	if err != nil {
		t.Fatalf("snapshot views: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name, def string
		if err := rows.Scan(&name, &def); err != nil {
			t.Fatalf("scan view: %v", err)
		}
		snap.Views[name] = def
	}
}

func rowCount(t *testing.T, ctx context.Context, db *sql.DB, table string) int {
	t.Helper()
	var count int
	if err := db.QueryRowContext(ctx, "select count(*) from "+table).Scan(&count); err != nil {
		t.Fatalf("row count for %s: %v", table, err)
	}
	return count
}
