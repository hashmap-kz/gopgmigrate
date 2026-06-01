package differ

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashmap-kz/gopgmigrate/v2/internal/conn"
	"github.com/hashmap-kz/gopgmigrate/v2/internal/manifest"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
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
	defer func() { _ = c.Stop() }()
	fmt.Fprintf(os.Stderr, "container ready: %s\n\n", c.ID[:12])

	const dbName = "diffdb"
	if err := containerExecSQL(ctx, c, "postgres", "CREATE DATABASE "+dbName+";"); err != nil {
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
	if err := applyEntries(ctx, c, dbName, applied); err != nil {
		return err
	}
	if err := schemaDump(ctx, c, dbName, curSQL); err != nil {
		return err
	}
	if err := globalsDump(ctx, c, curGlob); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "applying %d pending migration(s) to container...\n", len(pending))
	if err := applyEntries(ctx, c, dbName, pending); err != nil {
		return err
	}
	if err := schemaDump(ctx, c, dbName, planSQL); err != nil {
		return err
	}
	if err := globalsDump(ctx, c, planGlob); err != nil {
		return err
	}

	schemaDiff, err := fileDiff(curSQL, planSQL)
	if err != nil {
		return err
	}
	globalsDiff, err := fileDiff(curGlob, planGlob)
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

func applyEntries(ctx context.Context, c *Container, dbName string, entries []manifest.Entry) error {
	for _, e := range entries {
		for _, f := range e.Files {
			if err := applyFile(ctx, c, dbName, f); err != nil {
				return fmt.Errorf("apply %s: %w", f.Path, err)
			}
		}
	}
	return nil
}

// applyFile copies the SQL file into the container and runs it with psql -f.
func applyFile(ctx context.Context, c *Container, dbName string, f manifest.File) error {
	const dst = "/tmp/_gopgmigrate_mig.sql"
	if err := c.copyFile(ctx, f.AbsPath, dst); err != nil {
		return fmt.Errorf("copy to container: %w", err)
	}
	_, err := c.execOutput(ctx,
		"psql", "-U", "postgres", "-d", dbName, "-v", "ON_ERROR_STOP=1", "-f", dst,
	)
	return err
}

func containerExecSQL(ctx context.Context, c *Container, dbName, query string) error {
	_, err := c.execOutput(ctx,
		"psql", "-U", "postgres", "-d", dbName, "-v", "ON_ERROR_STOP=1", "-c", query,
	)
	return err
}

func schemaDump(ctx context.Context, c *Container, dbName, outPath string) error {
	out, err := c.execOutput(ctx,
		"pg_dump", "-U", "postgres", "-s", "--no-comments", "--restrict-key", "0", "-d", dbName,
	)
	if err != nil {
		return fmt.Errorf("pg_dump: %w", err)
	}
	return os.WriteFile(outPath, out, 0o644)
}

func globalsDump(ctx context.Context, c *Container, outPath string) error {
	out, err := c.execOutput(ctx,
		"pg_dumpall", "-U", "postgres", "--globals-only", "--restrict-key", "0",
	)
	if err != nil {
		return fmt.Errorf("pg_dumpall: %w", err)
	}
	return os.WriteFile(outPath, out, 0o644)
}

// fileDiff computes a unified diff between two files using the Myers algorithm.
// No external tools required.
func fileDiff(pathA, pathB string) (string, error) {
	a, err := os.ReadFile(pathA)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(pathB)
	if err != nil {
		return "", err
	}
	edits := myers.ComputeEdits(span.URIFromPath(pathA), string(a), string(b))
	return fmt.Sprint(gotextdiff.ToUnified(
		filepath.Base(pathA), filepath.Base(pathB),
		string(a), edits,
	)), nil
}
