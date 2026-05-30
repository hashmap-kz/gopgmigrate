package executor

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"github.com/hashmap-kz/gopgmigrate/v2/internal/history"
	"github.com/hashmap-kz/gopgmigrate/v2/internal/manifest"
	"github.com/hashmap-kz/gopgmigrate/v2/internal/stmt"
)

// EntryStatus describes the current state of a single file in the manifest.
type EntryStatus struct {
	Path        string
	Kind        string
	Applied     bool
	Checksum    string
	Description string
}

// NoTxHistoryError is returned when a no-tx migration was applied successfully
// but writing the history record failed. The migration is in the database but
// unrecorded — re-running without intervention will attempt to apply it again.
//
// Recovery: execute RecoverySQL() manually, then re-run.
type NoTxHistoryError struct {
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
		"INSERT INTO %s (path, kind, checksum, description) VALUES ('%s', 'no-tx', '%s', %s);",
		e.Table, e.Path, e.Checksum, desc,
	)
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
			slog.Warn("executor: release lock", "err", err)
		}
	}()

	applied, err := r.All(ctx, db)
	if err != nil {
		return err
	}

	// Entries are applied in manifest declaration order.
	// This is the only ordering guarantee — do not sort or parallelize.
	for _, entry := range mf.Entries {
		if err := applyEntry(ctx, db, r, applied, entry, mf.Table, dryRun); err != nil {
			return err
		}
	}
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
		for _, path := range entry.Files {
			checksum, err := manifest.Checksum(path)
			if err != nil {
				return nil, err
			}
			row, exists := applied[path]
			kind := kindLabel(entry)
			if exists && entry.Mode != manifest.ModeRepeatable && row.Checksum != checksum {
				kind += " [CHECKSUM MISMATCH]"
			}
			out = append(out, EntryStatus{
				Path:        path,
				Kind:        kind,
				Checksum:    checksum,
				Description: entry.Description,
				Applied:     exists,
			})
		}
	}
	return out, nil
}

// Validate checks that all files referenced in the manifest exist and are readable.
// Does not require a DB connection.
func Validate(mf *manifest.Manifest) error {
	for _, entry := range mf.Entries {
		for _, path := range entry.Files {
			if _, err := manifest.Checksum(path); err != nil {
				return fmt.Errorf("validate: %w", err)
			}
		}
	}
	return nil
}

// --- entry dispatch ---

func applyEntry(
	ctx context.Context,
	db *sql.DB,
	r *history.Exported,
	applied map[string]history.Row,
	entry manifest.Entry,
	table string,
	dryRun bool,
) error {
	switch entry.Mode {
	case manifest.ModeAtomic:
		return applyAtomic(ctx, db, r, applied, entry, dryRun)
	case manifest.ModeNoTx:
		return applyNoTx(ctx, db, r, applied, entry, table, dryRun)
	case manifest.ModeRepeatable:
		// repeatable enforces single file at manifest load time
		return applyRepeatable(ctx, db, r, applied, entry, dryRun)
	default:
		return applyDefault(ctx, db, r, applied, entry, dryRun)
	}
}

// --- default: one tx per file ---

func applyDefault(
	ctx context.Context,
	db *sql.DB,
	r *history.Exported,
	applied map[string]history.Row,
	entry manifest.Entry,
	dryRun bool,
) error {
	for _, path := range entry.Files {
		row, exists := applied[path]
		if exists {
			if err := checksumGuard(path, row.Checksum); err != nil {
				return err
			}
			slog.Info("skip", "path", path, "reason", "already applied")
			continue
		}
		if dryRun {
			slog.Info("dry-run", "path", path, "mode", "default")
			continue
		}
		if err := execInTx(ctx, db, func(tx *sql.Tx) error {
			content, err := manifest.ReadFile(path)
			if err != nil {
				return err
			}
			if err := execStatements(ctx, tx, path, content); err != nil {
				return err
			}
			checksum, err := manifest.Checksum(path)
			if err != nil {
				return err
			}
			return r.Insert(ctx, tx, path, "once", checksum, entry.Description)
		}); err != nil {
			return err
		}
		slog.Info("applied", "path", path, "mode", "default")
	}
	return nil
}

// --- atomic: one tx across all files ---

func applyAtomic(
	ctx context.Context,
	db *sql.DB,
	r *history.Exported,
	applied map[string]history.Row,
	entry manifest.Entry,
	dryRun bool,
) error {
	appliedCount := 0
	for _, path := range entry.Files {
		row, exists := applied[path]
		if !exists {
			continue
		}
		if err := checksumGuard(path, row.Checksum); err != nil {
			return err
		}
		appliedCount++
	}

	if appliedCount > 0 && appliedCount < len(entry.Files) {
		return fmt.Errorf(
			"executor: atomic entry partially applied (%d/%d files recorded) — manual intervention required",
			appliedCount, len(entry.Files),
		)
	}
	if appliedCount == len(entry.Files) {
		slog.Info("skip atomic", "reason", "already applied", "files", len(entry.Files))
		return nil
	}

	if dryRun {
		for _, path := range entry.Files {
			slog.Info("dry-run", "path", path, "mode", "atomic")
		}
		return nil
	}

	return execInTx(ctx, db, func(tx *sql.Tx) error {
		for _, path := range entry.Files {
			content, err := manifest.ReadFile(path)
			if err != nil {
				return err
			}
			if err := execStatements(ctx, tx, path, content); err != nil {
				return err
			}
			checksum, err := manifest.Checksum(path)
			if err != nil {
				return err
			}
			if err := r.Insert(ctx, tx, path, "once", checksum, entry.Description); err != nil {
				return err
			}
			slog.Info("atomic file applied", "path", path)
		}
		return nil
	})
}

// --- no-tx: raw execution, no transaction wrapper ---

func applyNoTx(
	ctx context.Context,
	db *sql.DB,
	r *history.Exported,
	applied map[string]history.Row,
	entry manifest.Entry,
	table string,
	dryRun bool,
) error {
	for _, path := range entry.Files {
		row, exists := applied[path]
		if exists {
			if err := checksumGuard(path, row.Checksum); err != nil {
				return err
			}
			slog.Info("skip", "path", path, "reason", "already applied")
			continue
		}
		if dryRun {
			slog.Info("dry-run", "path", path, "mode", "no-tx")
			continue
		}

		content, err := manifest.ReadFile(path)
		if err != nil {
			return err
		}
		if err := execStatements(ctx, db, path, content); err != nil {
			return err
		}

		checksum, err := manifest.Checksum(path)
		if err != nil {
			return err
		}

		// History insert is outside any transaction — gap is inherent to no-tx.
		// On failure, return a NoTxHistoryError with recovery SQL.
		if err := r.Insert(ctx, db, path, "no-tx", checksum, entry.Description); err != nil {
			return &NoTxHistoryError{
				Path:        path,
				Table:       table,
				Checksum:    checksum,
				Description: entry.Description,
				Cause:       err,
			}
		}
		slog.Info("applied", "path", path, "mode", "no-tx")
	}
	return nil
}

// --- repeatable: reruns when checksum changes, one tx per file ---

func applyRepeatable(
	ctx context.Context,
	db *sql.DB,
	r *history.Exported,
	applied map[string]history.Row,
	entry manifest.Entry,
	dryRun bool,
) error {
	// manifest load guarantees exactly one file for repeatable
	path := entry.Files[0]

	checksum, err := manifest.Checksum(path)
	if err != nil {
		return err
	}

	row, exists := applied[path]
	if exists && row.Checksum == checksum {
		slog.Info("skip", "path", path, "reason", "unchanged")
		return nil
	}

	if dryRun {
		slog.Info("dry-run", "path", path, "mode", "repeatable")
		return nil
	}

	return execInTx(ctx, db, func(tx *sql.Tx) error {
		content, err := manifest.ReadFile(path)
		if err != nil {
			return err
		}
		if err := execStatements(ctx, tx, path, content); err != nil {
			return err
		}
		if err := r.Upsert(ctx, tx, path, "repeatable", checksum, entry.Description); err != nil {
			return err
		}
		slog.Info("applied", "path", path, "mode", "repeatable")
		return nil
	})
}

// --- helpers ---

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

// checksumGuard returns an error if the on-disk file differs from the recorded checksum.
func checksumGuard(path, recorded string) error {
	current, err := manifest.Checksum(path)
	if err != nil {
		return err
	}
	if current != recorded {
		return fmt.Errorf(
			"executor: checksum mismatch for applied migration %q — file was modified after apply",
			path,
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
