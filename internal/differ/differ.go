package differ

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hashmap-kz/gopgmigrate/v2/internal/conn"
	"github.com/hashmap-kz/gopgmigrate/v2/internal/manifest"
	"github.com/jackc/pgx/v5/pgconn"
)

// Options configures a diff run.
type Options struct {
	DSN     string
	Dir     string
	Table   string
	NoColor bool
	OutDir  string // leave empty to use a temp dir
}

// Run connects to the real DB to learn which migrations are applied,
// spins up a Docker PostgreSQL container, replays applied and pending
// migrations, dumps schema state at each checkpoint, and prints a
// colored diff of the changes the pending migrations would introduce.
func Run(ctx context.Context, opts Options) error {
	if opts.DSN == "" && !hasPGEnv() {
		return fmt.Errorf("diff: no connection configured: provide --dsn or set PGHOST / PGPORT / PGDATABASE / PGUSER")
	}

	db, err := conn.Open(opts.DSN)
	if err != nil {
		return fmt.Errorf("diff: connect: %w", err)
	}
	appliedIDs, pgMajor, err := getDBInfo(ctx, db, opts.Table)
	db.Close()
	if err != nil {
		return fmt.Errorf("diff: query DB: %w", err)
	}
	// pgMajor is reserved for future version-aware image selection.
	_ = pgMajor

	mf, err := manifest.Scan(opts.Dir)
	if err != nil {
		return err
	}

	var applied, pending []manifest.Entry
	for _, e := range mf.Entries {
		if appliedIDs[e.Revision] {
			applied = append(applied, e)
		} else {
			pending = append(pending, e)
		}
	}

	if len(pending) == 0 {
		fmt.Fprintln(os.Stdout, "no pending migrations - schema is up to date")
		return nil
	}

	fmt.Fprintf(os.Stderr, "applied: %d  pending: %d\n", len(applied), len(pending))
	for _, e := range pending {
		fmt.Fprintf(os.Stderr, "  + %s\n", e.Files[0].Path)
	}

	fmt.Fprintf(os.Stderr, "\nstarting container (%s)...\n", defaultImage)
	c, err := StartContainer(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = StopContainer(c.ID) }()
	fmt.Fprintf(os.Stderr, "container ready: %s\n\n", c.ID[:12])

	const dbName = "diffdb"
	if err := containerExecSQL(ctx, c.ID, "postgres", "CREATE DATABASE "+dbName+";"); err != nil {
		return fmt.Errorf("diff: create database: %w", err)
	}

	outDir := opts.OutDir
	if outDir == "" {
		outDir, err = os.MkdirTemp("", "gopgmigrate-diff-*")
		if err != nil {
			return fmt.Errorf("diff: create temp dir: %w", err)
		}
		fmt.Fprintf(os.Stderr, "dump files: %s\n\n", outDir)
	} else {
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return fmt.Errorf("diff: out-dir: %w", err)
		}
	}

	curSQL := filepath.Join(outDir, "dbstate-current.sql")
	curGlob := filepath.Join(outDir, "dbstate-current-globals.sql")
	planSQL := filepath.Join(outDir, "dbstate-planned.sql")
	planGlob := filepath.Join(outDir, "dbstate-planned-globals.sql")

	fmt.Fprintf(os.Stderr, "applying %d applied migration(s) to container...\n", len(applied))
	if err := applyEntries(ctx, c.ID, dbName, applied); err != nil {
		return err
	}
	if err := schemaDump(ctx, c.ID, dbName, curSQL); err != nil {
		return err
	}
	if err := globalsDump(ctx, c.ID, curGlob); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "applying %d pending migration(s) to container...\n", len(pending))
	if err := applyEntries(ctx, c.ID, dbName, pending); err != nil {
		return err
	}
	if err := schemaDump(ctx, c.ID, dbName, planSQL); err != nil {
		return err
	}
	if err := globalsDump(ctx, c.ID, planGlob); err != nil {
		return err
	}

	schemaDiff, err := gitDiff(curSQL, planSQL)
	if err != nil {
		return err
	}
	globalsDiff, err := gitDiff(curGlob, planGlob)
	if err != nil {
		return err
	}

	color := !opts.NoColor
	printSection("Schema diff  (dbstate-current.sql -> dbstate-planned.sql)", schemaDiff, color)
	printSection("Globals diff (dbstate-current-globals.sql -> dbstate-planned-globals.sql)", globalsDiff, color)
	return nil
}

// getDBInfo returns the set of applied migration IDs and the PostgreSQL major
// version from the real database. If the history table does not yet exist,
// an empty map is returned (all migrations are pending).
func getDBInfo(ctx context.Context, db *sql.DB, table string) (map[int64]bool, int, error) {
	var vnum int
	if err := db.QueryRowContext(ctx,
		"SELECT current_setting('server_version_num')::integer",
	).Scan(&vnum); err != nil {
		return nil, 0, err
	}

	if table == "" {
		table = "schema_migrations"
	}

	rows, err := db.QueryContext(ctx, fmt.Sprintf("SELECT migration_id FROM %s", table))
	if err != nil {
		if isUndefinedTable(err) {
			return make(map[int64]bool), vnum / 10000, nil
		}
		return nil, vnum / 10000, err
	}
	defer rows.Close()

	ids := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, vnum / 10000, err
		}
		ids[id] = true
	}
	return ids, vnum / 10000, rows.Err()
}

func isUndefinedTable(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42P01"
}

func hasPGEnv() bool {
	for _, k := range []string{"PGHOST", "PGPORT", "PGDATABASE", "PGUSER", "PGPASSWORD"} {
		if os.Getenv(k) != "" {
			return true
		}
	}
	return false
}

func applyEntries(ctx context.Context, containerID, dbName string, entries []manifest.Entry) error {
	for _, e := range entries {
		for _, f := range e.Files {
			if err := applyFile(ctx, containerID, dbName, f); err != nil {
				return fmt.Errorf("apply %s: %w", f.Path, err)
			}
		}
	}
	return nil
}

// applyFile copies the SQL file into the container and runs it with psql -f.
// Using docker cp + psql -f avoids stdin-pipe reliability issues on Windows.
func applyFile(ctx context.Context, containerID, dbName string, f manifest.File) error {
	const dst = "/tmp/_gopgmigrate_mig.sql"
	if err := CopyToContainer(ctx, containerID, f.AbsPath, dst); err != nil {
		return fmt.Errorf("copy to container: %w", err)
	}
	_, err := ExecOutput(ctx, containerID,
		"psql", "-U", "postgres", "-d", dbName, "-v", "ON_ERROR_STOP=1", "-f", dst,
	)
	return err
}

func containerExecSQL(ctx context.Context, containerID, dbName, query string) error {
	_, err := ExecOutput(ctx, containerID,
		"psql", "-U", "postgres", "-d", dbName, "-v", "ON_ERROR_STOP=1", "-c", query,
	)
	return err
}

func schemaDump(ctx context.Context, containerID, dbName, outPath string) error {
	out, err := ExecOutput(ctx, containerID,
		"pg_dump", "-U", "postgres", "-s", "--no-comments", "--restrict-key", "0", "-d", dbName,
	)
	if err != nil {
		return fmt.Errorf("pg_dump: %w", err)
	}
	return os.WriteFile(outPath, out, 0o644)
}

func globalsDump(ctx context.Context, containerID, outPath string) error {
	out, err := ExecOutput(ctx, containerID,
		"pg_dumpall", "-U", "postgres", "--globals-only", "--restrict-key", "0",
	)
	if err != nil {
		return fmt.Errorf("pg_dumpall: %w", err)
	}
	return os.WriteFile(outPath, out, 0o644)
}

func gitDiff(fileA, fileB string) (string, error) {
	cmd := exec.Command("git", "diff", "--no-index", fileA, fileB)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// exit 1 means files differ - expected, not an error
			return string(out), nil
		}
		return "", fmt.Errorf("git diff: %w\n%s", err, out)
	}
	return string(out), nil
}
