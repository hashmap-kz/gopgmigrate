package executor

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashmap-kz/gopgmigrate/v2/internal/history"
	"github.com/hashmap-kz/gopgmigrate/v2/internal/manifest"
	"github.com/hashmap-kz/gopgmigrate/v2/internal/progress"
	"github.com/hashmap-kz/gopgmigrate/v2/internal/stmt"
)

// EntryStatus describes the current state of a single file in the manifest.
type EntryStatus struct {
	MigrationID string
	Path        string
	Kind        string
	Applied     bool
	Pending     bool      // true if Run would execute this file
	AppliedAt   time.Time // zero value when not yet applied
	Checksum    string
	Description string
}

// NoTxHistoryError is returned when a no-tx migration was applied successfully
// but writing the history record failed. The migration is in the database but
// unrecorded - re-running without intervention will attempt to apply it again.
//
// Recovery: execute RecoverySQL() manually, then re-run.
type NoTxHistoryError struct {
	MigrationID string
	Path        string
	Table       string
	Checksum    string
	Description string
	Cause       error
}

func (e *NoTxHistoryError) Error() string {
	return fmt.Sprintf(
		"executor: CRITICAL: no-tx migration %q was applied but history record failed to write.\n"+
			"The migration is in the database but will be re-applied on the next run.\n"+
			"To recover, manually execute:\n\n"+
			"  %s\n\n"+
			"Then re-run. Cause: %v",
		e.Path, e.RecoverySQL(), e.Cause,
	)
}

func (e *NoTxHistoryError) Unwrap() error { return e.Cause }

// RecoverySQL returns the exact INSERT needed to mark this migration as applied.
func (e *NoTxHistoryError) RecoverySQL() string {
	desc := "NULL"
	if e.Description != "" {
		desc = fmt.Sprintf("'%s'", e.Description)
	}
	return fmt.Sprintf(
		"INSERT INTO %s (migration_id, path, kind, checksum, description) VALUES ('%s', '%s', 'no-tx', '%s', %s);",
		e.Table, e.MigrationID, e.Path, e.Checksum, desc,
	)
}

type runStats struct {
	applied int
	skipped int
	stmts   int
}

// runner holds all shared state for a single Run invocation.
// Methods on runner replace the standalone applyXxx functions,
// keeping each method signature to (ctx, entry) only.
type runner struct {
	db      *sql.DB
	hist    *history.Exported
	applied map[string]history.Row
	table   string
	dryRun  bool
	tbl     *progress.Table // nil = no progress output
	stats   runStats
}

// Run applies all pending migrations in manifest declaration order.
func Run(ctx context.Context, db *sql.DB, mf *manifest.Manifest, output io.Writer, dryRun bool) error {
	hist := history.NewExported(mf.Table)

	if err := hist.Init(ctx, db); err != nil {
		return err
	}

	ok, err := hist.Lock(ctx, db)
	if err != nil {
		return fmt.Errorf("executor: acquire lock: %w", err)
	}
	if !ok {
		return fmt.Errorf("executor: another migration is running (advisory lock held)")
	}
	defer func() {
		if err := hist.Unlock(ctx, db); err != nil {
			slog.WarnContext(ctx, "executor: release lock", slog.Any("err", err))
		}
	}()

	applied, err := hist.All(ctx, db)
	if err != nil {
		return err
	}

	var tbl *progress.Table
	if output != nil {
		tbl = progress.NewTable(output)
		tbl.Header()
	}

	r := &runner{
		db:      db,
		hist:    hist,
		applied: applied,
		table:   mf.Table,
		dryRun:  dryRun,
		tbl:     tbl,
	}

	start := time.Now()

	// Entries are applied in manifest declaration order.
	// This is the only ordering guarantee - do not sort or parallelize.
	for _, entry := range mf.Entries {
		if err := r.applyEntry(ctx, entry); err != nil {
			return err
		}
	}

	if tbl != nil {
		tbl.Summary(progress.TotalDone{
			Applied:    r.stats.applied,
			Skipped:    r.stats.skipped,
			Statements: r.stats.stmts,
			Took:       time.Since(start),
		})
	}

	slog.InfoContext(ctx, "run complete",
		slog.Int("applied", r.stats.applied),
		slog.Int("skipped", r.stats.skipped),
	)
	return nil
}

// Status returns the current state of every file in the manifest.
func Status(ctx context.Context, db *sql.DB, mf *manifest.Manifest) ([]EntryStatus, error) {
	r := history.NewExported(mf.Table)
	if err := r.Init(ctx, db); err != nil {
		return nil, err
	}
	applied, err := r.All(ctx, db)
	if err != nil {
		return nil, err
	}

	var out []EntryStatus
	for _, entry := range mf.Entries {
		for _, f := range entry.Files {
			checksum, err := manifest.Checksum(f.AbsPath)
			if err != nil {
				return nil, err
			}
			row, exists := applied[f.Path]
			kind := kindLabel(entry)
			pending := !exists
			if exists && entry.Mode != manifest.ModeRepeatable && row.Checksum != checksum {
				kind += " [CHECKSUM MISMATCH]"
			}
			if exists && entry.Mode == manifest.ModeRepeatable && row.Checksum != checksum {
				pending = true
			}
			out = append(out, EntryStatus{
				MigrationID: buildMigrationID(entry.ID, f.Path),
				Path:        f.Path,
				Kind:        kind,
				Applied:     exists,
				Pending:     pending,
				AppliedAt:   row.AppliedAt,
				Checksum:    checksum,
				Description: entry.Description,
			})
		}
	}
	return out, nil
}

// Validate checks that all files referenced in the manifest exist and are readable.
// Does not require a DB connection.
func Validate(mf *manifest.Manifest) error {
	for _, entry := range mf.Entries {
		for _, f := range entry.Files {
			if _, err := manifest.Checksum(f.AbsPath); err != nil {
				return fmt.Errorf("validate: %w", err)
			}
		}
	}
	return nil
}

func (r *runner) applyEntry(ctx context.Context, entry manifest.Entry) error {
	switch entry.Mode {
	case manifest.ModeAtomic:
		return r.applyAtomic(ctx, entry)
	case manifest.ModeNoTx:
		return r.applyNoTx(ctx, entry)
	case manifest.ModeRepeatable:
		return r.applyRepeatable(ctx, entry)
	default:
		return r.applyDefault(ctx, entry)
	}
}

// default: one tx per file

func (r *runner) applyDefault(ctx context.Context, entry manifest.Entry) error {
	for _, f := range entry.Files {
		row, exists := r.applied[f.Path]
		if exists {
			if err := checksumGuard(f.AbsPath, row.Checksum); err != nil {
				return err
			}
			slog.DebugContext(ctx, "skip",
				slog.String("path", f.Path),
				slog.String("reason", "already applied"),
			)
			if r.tbl != nil {
				r.tbl.Skip(f.Path, progress.ModeTx, "already applied")
			}
			r.stats.skipped++
			continue
		}
		if r.dryRun {
			slog.DebugContext(ctx, "dry-run",
				slog.String("path", f.Path),
				slog.String("mode", "default"),
			)
			if r.tbl != nil {
				r.tbl.Skip(f.Path, progress.ModeTx, "dry-run")
			}
			r.stats.skipped++
			continue
		}

		if r.tbl != nil {
			r.tbl.Run(f.Path, progress.ModeTx)
		}
		start := time.Now()

		migID := buildMigrationID(entry.ID, f.Path)
		var n int
		if err := execInTx(ctx, r.db, func(tx *sql.Tx) error {
			content, err := manifest.ReadFile(f.AbsPath)
			if err != nil {
				return err
			}
			n, err = execStatements(ctx, tx, f.Path, content)
			if err != nil {
				return err
			}
			checksum, err := manifest.Checksum(f.AbsPath)
			if err != nil {
				return err
			}
			return r.hist.Insert(ctx, tx, &history.Record{
				MigrationID: migID,
				Path:        f.Path,
				Kind:        "once",
				Checksum:    checksum,
				Description: entry.Description,
			})
		}); err != nil {
			if r.tbl != nil {
				r.tbl.Fail(f.Path, progress.ModeTx, time.Since(start), err.Error())
			}
			return err
		}

		took := time.Since(start)
		slog.DebugContext(ctx, "applied",
			slog.String("path", f.Path),
			slog.String("mode", "default"),
		)
		if r.tbl != nil {
			r.tbl.OK(f.Path, progress.ModeTx, progress.Done{Statements: n, Took: took})
		}
		r.stats.applied++
		r.stats.stmts += n
	}
	return nil
}

// atomic: one tx across all files

func (r *runner) applyAtomic(ctx context.Context, entry manifest.Entry) error {
	appliedCount := 0
	for _, f := range entry.Files {
		row, exists := r.applied[f.Path]
		if !exists {
			continue
		}
		if err := checksumGuard(f.AbsPath, row.Checksum); err != nil {
			return err
		}
		appliedCount++
	}

	if appliedCount > 0 && appliedCount < len(entry.Files) {
		return fmt.Errorf(
			"executor: atomic entry partially applied (%d/%d files recorded) - manual intervention required",
			appliedCount, len(entry.Files),
		)
	}
	if appliedCount == len(entry.Files) {
		slog.DebugContext(ctx, "skip atomic",
			slog.String("reason", "already applied"),
			slog.Int("files", len(entry.Files)),
		)
		if r.tbl != nil {
			r.tbl.Skip("atomic/"+entry.ID, progress.ModeAtomic, "already applied")
		}
		r.stats.skipped += len(entry.Files)
		return nil
	}

	if r.dryRun {
		for _, f := range entry.Files {
			slog.DebugContext(ctx, "dry-run",
				slog.String("path", f.Path),
				slog.String("mode", "atomic"),
			)
			if r.tbl != nil {
				r.tbl.Skip(f.Path, progress.ModeAtomic, "dry-run")
			}
		}
		r.stats.skipped += len(entry.Files)
		return nil
	}

	if r.tbl != nil {
		r.tbl.BeginAtomic(entry.ID)
	}
	start := time.Now()
	var totalStmts int

	if err := execInTx(ctx, r.db, func(tx *sql.Tx) error {
		for _, f := range entry.Files {
			if r.tbl != nil {
				r.tbl.Run(f.Path, progress.ModeAtomic)
			}
			fileStart := time.Now()

			content, err := manifest.ReadFile(f.AbsPath)
			if err != nil {
				return err
			}
			n, err := execStatements(ctx, tx, f.Path, content)
			if err != nil {
				return err
			}
			checksum, err := manifest.Checksum(f.AbsPath)
			if err != nil {
				return err
			}
			migID := buildMigrationID(entry.ID, f.Path)
			if err := r.hist.Insert(ctx, tx, &history.Record{
				MigrationID: migID,
				Path:        f.Path,
				Kind:        "once",
				Checksum:    checksum,
				Description: entry.Description,
			}); err != nil {
				return err
			}
			slog.DebugContext(ctx, "atomic file applied", slog.String("path", f.Path))
			if r.tbl != nil {
				r.tbl.OK(f.Path, progress.ModeAtomic, progress.Done{Statements: n, Took: time.Since(fileStart)})
			}
			totalStmts += n
		}
		return nil
	}); err != nil {
		took := time.Since(start)
		if r.tbl != nil {
			r.tbl.AbortAtomic(entry.ID, took, err.Error())
		}
		return err
	}

	took := time.Since(start)
	if r.tbl != nil {
		r.tbl.CommitAtomic(entry.ID, progress.Done{
			Files:      len(entry.Files),
			Statements: totalStmts,
			Took:       took,
		})
	}
	r.stats.applied += len(entry.Files)
	r.stats.stmts += totalStmts
	return nil
}

// no-tx: raw execution, no transaction wrapper

func (r *runner) applyNoTx(ctx context.Context, entry manifest.Entry) error {
	for _, f := range entry.Files {
		row, exists := r.applied[f.Path]
		if exists {
			if err := checksumGuard(f.AbsPath, row.Checksum); err != nil {
				return err
			}
			slog.DebugContext(ctx, "skip",
				slog.String("path", f.Path),
				slog.String("reason", "already applied"),
			)
			if r.tbl != nil {
				r.tbl.Skip(f.Path, progress.ModeNoTx, "already applied")
			}
			r.stats.skipped++
			continue
		}
		if r.dryRun {
			slog.DebugContext(ctx, "dry-run",
				slog.String("path", f.Path),
				slog.String("mode", "no-tx"),
			)
			if r.tbl != nil {
				r.tbl.Skip(f.Path, progress.ModeNoTx, "dry-run")
			}
			r.stats.skipped++
			continue
		}

		if r.tbl != nil {
			r.tbl.Run(f.Path, progress.ModeNoTx)
		}
		start := time.Now()

		content, err := manifest.ReadFile(f.AbsPath)
		if err != nil {
			return err
		}
		n, err := execStatements(ctx, r.db, f.Path, content)
		if err != nil {
			if r.tbl != nil {
				r.tbl.Fail(f.Path, progress.ModeNoTx, time.Since(start), err.Error())
			}
			return err
		}

		checksum, err := manifest.Checksum(f.AbsPath)
		if err != nil {
			return err
		}

		migID := buildMigrationID(entry.ID, f.Path)
		// History insert is outside any transaction - gap is inherent to no-tx.
		// On failure, return a NoTxHistoryError with recovery SQL.
		if err := r.hist.Insert(ctx, r.db, &history.Record{
			MigrationID: migID,
			Path:        f.Path,
			Kind:        "no-tx",
			Checksum:    checksum,
			Description: entry.Description,
		}); err != nil {
			noTxErr := &NoTxHistoryError{
				MigrationID: migID,
				Path:        f.Path,
				Table:       r.table,
				Checksum:    checksum,
				Description: entry.Description,
				Cause:       err,
			}
			if r.tbl != nil {
				r.tbl.Fail(f.Path, progress.ModeNoTx, time.Since(start), "history write failed")
			}
			return noTxErr
		}

		took := time.Since(start)
		slog.DebugContext(ctx, "applied",
			slog.String("path", f.Path),
			slog.String("mode", "no-tx"),
		)
		if r.tbl != nil {
			r.tbl.OK(f.Path, progress.ModeNoTx, progress.Done{Statements: n, Took: took})
		}
		r.stats.applied++
		r.stats.stmts += n
	}
	return nil
}

// repeatable: reruns when checksum changes, one tx per file

func (r *runner) applyRepeatable(ctx context.Context, entry manifest.Entry) error {
	for _, f := range entry.Files {
		checksum, err := manifest.Checksum(f.AbsPath)
		if err != nil {
			return err
		}

		row, exists := r.applied[f.Path]
		if exists && row.Checksum == checksum {
			slog.DebugContext(ctx, "skip",
				slog.String("path", f.Path),
				slog.String("reason", "unchanged"),
			)
			if r.tbl != nil {
				r.tbl.Skip(f.Path, progress.ModeRepeat, "unchanged")
			}
			r.stats.skipped++
			continue
		}

		if r.dryRun {
			slog.DebugContext(ctx, "dry-run",
				slog.String("path", f.Path),
				slog.String("mode", "repeatable"),
			)
			if r.tbl != nil {
				r.tbl.Skip(f.Path, progress.ModeRepeat, "dry-run")
			}
			r.stats.skipped++
			continue
		}

		if r.tbl != nil {
			r.tbl.Run(f.Path, progress.ModeRepeat)
		}
		start := time.Now()

		migID := buildMigrationID(entry.ID, f.Path)
		var n int
		if err := execInTx(ctx, r.db, func(tx *sql.Tx) error {
			content, err := manifest.ReadFile(f.AbsPath)
			if err != nil {
				return err
			}
			n, err = execStatements(ctx, tx, f.Path, content)
			if err != nil {
				return err
			}
			return r.hist.Upsert(ctx, tx, &history.Record{
				MigrationID: migID,
				Path:        f.Path,
				Kind:        "repeatable",
				Checksum:    checksum,
				Description: entry.Description,
			})
		}); err != nil {
			if r.tbl != nil {
				r.tbl.Fail(f.Path, progress.ModeRepeat, time.Since(start), err.Error())
			}
			return err
		}

		took := time.Since(start)
		slog.DebugContext(ctx, "applied",
			slog.String("path", f.Path),
			slog.String("mode", "repeatable"),
		)
		if r.tbl != nil {
			r.tbl.OK(f.Path, progress.ModeRepeat, progress.Done{Statements: n, Took: took})
		}
		r.stats.applied++
		r.stats.stmts += n
	}
	return nil
}

// helpers

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func execInTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("executor: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("executor: commit: %w", err)
	}
	return nil
}

func execStatements(ctx context.Context, db execer, path, content string) (int, error) {
	stmts, err := stmt.SplitSQLStatements(content)
	if err != nil {
		return 0, fmt.Errorf("executor: parse %q: %w", path, err)
	}
	count := 0
	for _, s := range stmts {
		if strings.TrimSpace(s) == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, s); err != nil {
			return count, fmt.Errorf("executor: exec in %q: %w\nstatement: %s", path, err, s)
		}
		count++
	}
	return count, nil
}

func buildMigrationID(entryID, path string) string {
	return entryID + "/" + filepath.Base(path)
}

// checksumGuard returns an error if the on-disk file differs from the recorded checksum.
func checksumGuard(absPath, recorded string) error {
	current, err := manifest.Checksum(absPath)
	if err != nil {
		return err
	}
	if current != recorded {
		return fmt.Errorf(
			"executor: checksum mismatch for applied migration %q - file was modified after apply",
			absPath,
		)
	}
	return nil
}

func kindLabel(e manifest.Entry) string {
	switch e.Mode {
	case manifest.ModeAtomic:
		return "atomic"
	case manifest.ModeNoTx:
		return "no-tx"
	case manifest.ModeRepeatable:
		return "repeatable"
	default:
		return "once"
	}
}
