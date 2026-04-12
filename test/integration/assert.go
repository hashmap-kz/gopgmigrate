package integration

import (
	"database/sql"
	"testing"
)

type HistoryRow struct {
	Version int64
	Name    string
	Hash    string
}

func QueryHistory(t *testing.T, db *sql.DB, table string) []HistoryRow {
	t.Helper()
	rows, err := db.QueryContext(
		t.Context(),
		"select mh_version, mh_name, mh_hash from "+table+" order by mh_version",
	)
	if err != nil {
		t.Fatalf("query history: %v", err)
	}
	defer rows.Close()

	var result []HistoryRow
	for rows.Next() {
		var r HistoryRow
		if err := rows.Scan(&r.Version, &r.Name, &r.Hash); err != nil {
			t.Fatalf("scan history row: %v", err)
		}
		result = append(result, r)
	}
	return result
}

func TableExists(t *testing.T, db *sql.DB, schema, table string) bool {
	t.Helper()
	var exists bool
	err := db.QueryRowContext(t.Context(),
		"select exists (select 1 from information_schema.tables where table_schema=$1 and table_name=$2)",
		schema, table,
	).Scan(&exists)
	if err != nil {
		t.Fatalf("check table exists: %v", err)
	}
	return exists
}

func RowCount(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var count int
	if err := db.QueryRowContext(t.Context(), "select count(*) from "+table).Scan(&count); err != nil {
		t.Fatalf("row count: %v", err)
	}
	return count
}
