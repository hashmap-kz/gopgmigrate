package migrate

import (
	"context"
	"database/sql"

	"gopgmigrate/internal/history"
	"gopgmigrate/internal/stmts"
)

// RunMigrationsPlainMode applies both versioned and repeatable migrations
func RunMigrationsPlainMode(
	ctx context.Context,
	db *sql.DB,
	mhRepo history.MigrateHistoryRepository,
	pending []MigrationFile,
	directionDo bool,
) error {
	for _, elem := range pending {
		err := migrateOneScript(ctx, db, elem, mhRepo, directionDo)
		if err != nil {
			return err
		}
	}
	return nil
}

// migrateOneScript applies a single script, TX/NO-TX (based on filename pattern)
func migrateOneScript(
	ctx context.Context,
	db *sql.DB,
	file MigrationFile,
	mhRepo history.MigrateHistoryRepository,
	directionDo bool,
) (err error) {
	// TRANSACTION

	if isTx(file) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		script := []string{string(file.data)}
		err = migrateOneScriptFn(ctx, tx, script, file, mhRepo, directionDo, "+")
		if err != nil {
			return err
		}

		err = tx.Commit()
		if err != nil {
			return err
		}
		return nil
	}

	// NO TRANSACTION

	script, _ := stmts.SplitSQLStatements(string(file.data))
	return migrateOneScriptFn(ctx, db, script, file, mhRepo, directionDo, "N")
}
