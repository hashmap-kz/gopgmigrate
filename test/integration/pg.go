//go:build integration

package integration

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// PgDatabase is a fresh Postgres database created for a single test.
// It is dropped automatically when the test ends.
type PgDatabase struct {
	// ConnStr is the connection string scoped to this test's database.
	ConnStr string
	// DB is an open connection to this test's database.
	DB *sql.DB
}

// NewPgDatabase creates a throwaway database inside the shared Postgres
// instance started by docker-compose, connects to it, and registers a
// cleanup that drops the database when the test ends.
//
// Connection parameters are read from environment variables with sensible
// defaults that match docker-compose.yml:
//
//	PGHOST      (default: localhost)
//	PGPORT      (default: 5432)
//	PGUSER      (default: test)
//	PGPASSWORD  (default: test)
func NewPgDatabase(t *testing.T) *PgDatabase {
	t.Helper()

	host := "localhost" // envOr("PGHOST", "localhost")
	port := "15432"     // envOr("PGPORT", "5432")
	user := "test"      // envOr("PGUSER", "test")
	pass := "test"      // envOr("PGPASSWORD", "test")

	// root connection — used only to CREATE / DROP the test database
	rootConnStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/test?sslmode=disable",
		user, pass, host, port,
	)

	root, err := sql.Open("pgx", rootConnStr)
	if err != nil {
		t.Fatalf("open root connection: %v", err)
	}
	defer root.Close()

	if err := root.Ping(); err != nil {
		t.Fatalf("ping postgres — is docker-compose up? (%v)", err)
	}

	// unique name so parallel tests never collide
	dbName := fmt.Sprintf("test_%s", strings.ToLower(rand.Text()))

	if _, err := root.Exec("create database " + dbName); err != nil {
		t.Fatalf("create database %s: %v", dbName, err)
	}

	t.Cleanup(func() {
		//// terminate any lingering connections before dropping
		//_, _ = root.Exec(fmt.Sprintf(`
		//	select pg_terminate_backend(pid)
		//	from pg_stat_activity
		//	where datname = '%s' and pid <> pg_backend_pid()
		//`, dbName))
		//if _, err := root.Exec("drop database if exists " + dbName); err != nil {
		//	t.Logf("drop database %s: %v", dbName, err)
		//}
	})

	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		user, pass, host, port, dbName,
	)

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		t.Fatalf("open test database %s: %v", dbName, err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.Ping(); err != nil {
		t.Fatalf("ping test database %s: %v", dbName, err)
	}

	return &PgDatabase{
		ConnStr: connStr,
		DB:      db,
	}
}
