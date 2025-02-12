package migrate

import (
	"context"
	"database/sql"

	"gopgmigrate/internal/history"
	"gopgmigrate/internal/stmts"
)

// RunMigrationsGroupMode applies all pending migrations, wrapping in TX (for tx-based), and no-tx (for *.ntx.)
func RunMigrationsGroupMode(
	ctx context.Context,
	db *sql.DB,
	mhRepo history.MigrateHistoryRepository,
	batches []*GroupEntry,
	directionDo bool,
) error {
	for _, elem := range batches {
		err := migrateOneGroup(ctx, db, elem, mhRepo, directionDo)
		if err != nil {
			return err
		}
	}
	return nil
}

func migrateOneGroup(
	ctx context.Context,
	db *sql.DB,
	batch *GroupEntry,
	mhRepo history.MigrateHistoryRepository,
	directionDo bool,
) (err error) {
	if batch.UseTX {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		for _, file := range batch.Files {
			script := []string{string(file.data)}
			err = migrateOneScriptFn(ctx, tx, script, file, mhRepo, directionDo, "+")
			if err != nil {
				return err
			}
		}

		err = tx.Commit()
		if err != nil {
			return err
		}
		return nil

	} else {
		for _, file := range batch.Files {
			script, _ := stmts.SplitSQLStatements(string(file.data))
			err = migrateOneScriptFn(ctx, db, script, file, mhRepo, directionDo, "N")
			if err != nil {
				return err
			}
		}
	}

	return nil
}
