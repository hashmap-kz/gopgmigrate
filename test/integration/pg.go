//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// PgDatabase is a fresh Postgres database created for a single test.
type PgDatabase struct {
	ConnStr string
	DB      *sql.DB
}

func NewPgDatabase(t *testing.T) *PgDatabase {
	t.Helper()

	host := "localhost"
	port := "15432"
	user := "postgres"
	pass := "postgres"

	rootDSN := fmt.Sprintf("postgres://%s:%s@%s:%s/postgres?sslmode=disable", user, pass, host, port)

	root, err := sql.Open("pgx", rootDSN)
	if err != nil {
		t.Fatalf("open root connection: %v", err)
	}

	if err := root.Ping(); err != nil {
		root.Close()
		t.Fatalf("ping postgres - is docker-compose up? (%v)", err)
	}

	dbName := fmt.Sprintf("test_%s", strings.ToLower(rand.Text()))

	if _, err := root.Exec("create database " + dbName); err != nil {
		root.Close()
		t.Fatalf("create database %s: %v", dbName, err)
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, dbName)

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		root.Close()
		t.Fatalf("open test database %s: %v", dbName, err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		root.Close()
		t.Fatalf("ping test database %s: %v", dbName, err)
	}

	t.Cleanup(func() {
		db.Close()
		_, _ = root.ExecContext(context.Background(),
			"select pg_terminate_backend(pid) from pg_stat_activity where datname = $1 and pid <> pg_backend_pid()",
			dbName,
		)
		if _, err := root.ExecContext(context.Background(), "drop database if exists "+dbName); err != nil {
			t.Logf("drop database %s: %v", dbName, err)
		}
		root.Close()
	})

	return &PgDatabase{ConnStr: connStr, DB: db}
}
