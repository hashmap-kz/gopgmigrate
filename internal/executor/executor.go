package executor

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashmap-kz/gopgmigrate/v2/internal/history"
	"github.com/hashmap-kz/gopgmigrate/v2/internal/manifest"
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
}

// Run applies all pending migrations in manifest declaration order.
func Run(ctx context.Context, db *sql.DB, mf *manifest.Manifest, dryRun bool) error {
	r := history.NewExported(mf.Table)

	if err := r.Init(ctx, db); err != nil {
		return err
	}

	ok, err := r.Lock(ctx, db)
	if err != nil {
		return fmt.Errorf("executor: acquire lock: %w", err)
	}
	if !ok {
		return fmt.Errorf("executor: another migration is running (advisory lock held)")
	}
	defer func() {
		if err := r.Unlock(ctx, db); err != nil {
			slog.WarnContext(ctx, "executor: release lock", slog.Any("err", err))
		}
	}()

	applied, err := r.All(ctx, db)
	if err != nil {
		return err
	}

	var total runStats

	// Entries are applied in manifest declaration order.
	// This is the only ordering guarantee - do not sort or parallelize.
	for _, entry := range mf.Entries {
		s, err := applyEntry(ctx, db, r, applied, entry, mf.Table, dryRun)
		if err != nil {
			return err
		}
		total.applied += s.applied
		total.skipped += s.skipped
	}

	slog.InfoContext(ctx, "run complete",
		slog.Int("applied", total.applied),
		slog.Int("skipped", total.skipped),
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

// entry dispatch

func applyEntry(
	ctx context.Context,
	db *sql.DB,
	r *history.Exported,
	applied map[string]history.Row,
	entry manifest.Entry,
	table string,
	dryRun bool,
) (runStats, error) {
	switch entry.Mode {
	case manifest.ModeAtomic:
		return applyAtomic(ctx, db, r, applied, entry, dryRun)
	case manifest.ModeNoTx:
		return applyNoTx(ctx, db, r, applied, entry, table, dryRun)
	case manifest.ModeRepeatable:
		return applyRepeatable(ctx, db, r, applied, entry, dryRun)
	default:
		return applyDefault(ctx, db, r, applied, entry, dryRun)
	}
}

// default: one tx per file

func applyDefault(
	ctx context.Context,
	db *sql.DB,
	r *history.Exported,
	applied map[string]history.Row,
	entry manifest.Entry,
	dryRun bool,
) (runStats, error) {
	var stats runStats
	for _, f := range entry.Files {
		row, exists := applied[f.Path]
		if exists {
			if err := checksumGuard(f.AbsPath, row.Checksum); err != nil {
				return stats, err
			}
			slog.InfoContext(ctx, "skip",
				slog.String("path", f.Path),
				slog.String("reason", "already applied"),
			)
			stats.skipped++
			continue
		}
		if dryRun {
			slog.InfoContext(ctx, "dry-run",
				slog.String("path", f.Path),
				slog.String("mode", "default"),
			)
			stats.skipped++
			continue
		}
		migID := buildMigrationID(entry.ID, f.Path)
		if err := execInTx(ctx, db, func(tx *sql.Tx) error {
			content, err := manifest.ReadFile(f.AbsPath)
			if err != nil {
				return err
			}
			if err := execStatements(ctx, tx, f.Path, content); err != nil {
				return err
			}
			checksum, err := manifest.Checksum(f.AbsPath)
			if err != nil {
				return err
			}
			return r.Insert(ctx, tx, &history.Record{
				MigrationID: migID,
				Path:        f.Path,
				Kind:        "once",
				Checksum:    checksum,
				Description: entry.Description,
			})
		}); err != nil {
			return stats, err
		}
		slog.InfoContext(ctx, "applied",
			slog.String("path", f.Path),
			slog.String("mode", "default"),
		)
		stats.applied++
	}
	return stats, nil
}

// atomic: one tx across all files

func applyAtomic(
	ctx context.Context,
	db *sql.DB,
	r *history.Exported,
	applied map[string]history.Row,
	entry manifest.Entry,
	dryRun bool,
) (runStats, error) {
	var stats runStats

	appliedCount := 0
	for _, f := range entry.Files {
		row, exists := applied[f.Path]
		if !exists {
			continue
		}
		if err := checksumGuard(f.AbsPath, row.Checksum); err != nil {
			return stats, err
		}
		appliedCount++
	}

	if appliedCount > 0 && appliedCount < len(entry.Files) {
		return stats, fmt.Errorf(
			"executor: atomic entry partially applied (%d/%d files recorded) - manual intervention required",
			appliedCount, len(entry.Files),
		)
	}
	if appliedCount == len(entry.Files) {
		slog.InfoContext(ctx, "skip atomic",
			slog.String("reason", "already applied"),
			slog.Int("files", len(entry.Files)),
			slog.String("id", entry.ID),
		)
		stats.skipped += len(entry.Files)
		return stats, nil
	}

	if dryRun {
		for _, f := range entry.Files {
			slog.InfoContext(ctx, "dry-run",
				slog.String("path", f.Path),
				slog.String("mode", "atomic"),
			)
		}
		stats.skipped += len(entry.Files)
		return stats, nil
	}

	if err := execInTx(ctx, db, func(tx *sql.Tx) error {
		for _, f := range entry.Files {
			content, err := manifest.ReadFile(f.AbsPath)
			if err != nil {
				return err
			}
			if err := execStatements(ctx, tx, f.Path, content); err != nil {
				return err
			}
			checksum, err := manifest.Checksum(f.AbsPath)
			if err != nil {
				return err
			}
			migID := buildMigrationID(entry.ID, f.Path)
			if err := r.Insert(ctx, tx, &history.Record{
				MigrationID: migID,
				Path:        f.Path,
				Kind:        "once",
				Checksum:    checksum,
				Description: entry.Description,
			}); err != nil {
				return err
			}
			slog.InfoContext(ctx, "atomic file applied", slog.String("path", f.Path))
		}
		return nil
	}); err != nil {
		return stats, err
	}
	stats.applied += len(entry.Files)
	return stats, nil
}

// no-tx: raw execution, no transaction wrapper

func applyNoTx(
	ctx context.Context,
	db *sql.DB,
	r *history.Exported,
	applied map[string]history.Row,
	entry manifest.Entry,
	table string,
	dryRun bool,
) (runStats, error) {
	var stats runStats
	for _, f := range entry.Files {
		row, exists := applied[f.Path]
		if exists {
			if err := checksumGuard(f.AbsPath, row.Checksum); err != nil {
				return stats, err
			}
			slog.InfoContext(ctx, "skip",
				slog.String("path", f.Path),
				slog.String("reason", "already applied"),
			)
			stats.skipped++
			continue
		}
		if dryRun {
			slog.InfoContext(ctx, "dry-run",
				slog.String("path", f.Path),
				slog.String("mode", "no-tx"),
			)
			stats.skipped++
			continue
		}

		content, err := manifest.ReadFile(f.AbsPath)
		if err != nil {
			return stats, err
		}
		if err := execStatements(ctx, db, f.Path, content); err != nil {
			return stats, err
		}

		checksum, err := manifest.Checksum(f.AbsPath)
		if err != nil {
			return stats, err
		}

		migID := buildMigrationID(entry.ID, f.Path)
		// History insert is outside any transaction - gap is inherent to no-tx.
		// On failure, return a NoTxHistoryError with recovery SQL.
		if err := r.Insert(ctx, db, &history.Record{
			MigrationID: migID,
			Path:        f.Path,
			Kind:        "no-tx",
			Checksum:    checksum,
			Description: entry.Description,
		}); err != nil {
			return stats, &NoTxHistoryError{
				MigrationID: migID,
				Path:        f.Path,
				Table:       table,
				Checksum:    checksum,
				Description: entry.Description,
				Cause:       err,
			}
		}
		slog.InfoContext(ctx, "applied",
			slog.String("path", f.Path),
			slog.String("mode", "no-tx"),
		)
		stats.applied++
	}
	return stats, nil
}

// repeatable: reruns when checksum changes, one tx per file

func applyRepeatable(
	ctx context.Context,
	db *sql.DB,
	r *history.Exported,
	applied map[string]history.Row,
	entry manifest.Entry,
	dryRun bool,
) (runStats, error) {
	var stats runStats
	for _, f := range entry.Files {
		checksum, err := manifest.Checksum(f.AbsPath)
		if err != nil {
			return stats, err
		}

		row, exists := applied[f.Path]
		if exists && row.Checksum == checksum {
			slog.InfoContext(ctx, "skip",
				slog.String("path", f.Path),
				slog.String("reason", "unchanged"),
			)
			stats.skipped++
			continue
		}

		if dryRun {
			slog.InfoContext(ctx, "dry-run",
				slog.String("path", f.Path),
				slog.String("mode", "repeatable"),
			)
			stats.skipped++
			continue
		}

		migID := buildMigrationID(entry.ID, f.Path)
		if err := execInTx(ctx, db, func(tx *sql.Tx) error {
			content, err := manifest.ReadFile(f.AbsPath)
			if err != nil {
				return err
			}
			if err := execStatements(ctx, tx, f.Path, content); err != nil {
				return err
			}
			return r.Upsert(ctx, tx, &history.Record{
				MigrationID: migID,
				Path:        f.Path,
				Kind:        "repeatable",
				Checksum:    checksum,
				Description: entry.Description,
			})
		}); err != nil {
			return stats, err
		}
		slog.InfoContext(ctx, "applied",
			slog.String("path", f.Path),
			slog.String("mode", "repeatable"),
		)
		stats.applied++
	}
	return stats, nil
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

func execStatements(ctx context.Context, db execer, path, content string) error {
	stmts, err := stmt.SplitSQLStatements(content)
	if err != nil {
		return fmt.Errorf("executor: parse %q: %w", path, err)
	}
	for _, s := range stmts {
		if strings.TrimSpace(s) == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("executor: exec in %q: %w\nstatement: %s", path, err, s)
		}
	}
	return nil
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
