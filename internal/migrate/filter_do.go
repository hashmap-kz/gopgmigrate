package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"

	"gopgmigrate/internal/resolve"
	"gopgmigrate/internal/vers"

	"gopgmigrate/internal/history"
)

func getMigrationsForApply(
	ctx context.Context,
	db *sql.DB,
	migrationDirectory string,
	repo history.MigrateHistoryRepository,
) ([]vers.MigrationFile, error) {
	allLocalFiles, err := resolve.GetFiles(migrationDirectory, vers.VersionedMigrationRegexDo(), repo.GetNoTxPatterns())
	if err != nil {
		return nil, err
	}

	hist, err := repo.ListAll(ctx, db)
	if err != nil {
		return nil, err
	}

	err = checkAppliedHistoryWithLocalFiles(hist, allLocalFiles)
	if err != nil {
		return nil, err
	}

	return getVersionedMigrationsToApply(hist, allLocalFiles)
}

func checkAppliedHistoryWithLocalFiles(appliedMigrations []history.MigrateHistory, localFiles []vers.MigrationFile) error {
	for _, k := range appliedMigrations {
		if !appliedMigrationPresentLocally(k.MhName, localFiles) {
			return fmt.Errorf("detected applied migration not resolved locally: %s", k.MhName)
		}
	}
	return nil
}

func appliedMigrationPresentLocally(appliedScriptBasename string, localFiles []vers.MigrationFile) bool {
	for _, f := range localFiles {
		if appliedScriptBasename == f.Base {
			return true
		}
	}
	return false
}

func getVersionedMigrationsToApply(appliedMigrations []history.MigrateHistory, localFiles []vers.MigrationFile) ([]vers.MigrationFile, error) {
	var toApply []vers.MigrationFile
	for _, file := range localFiles {
		// twice check a file given
		isVersioned := vers.IsVersioned(file.Base)
		if !isVersioned {
			continue
		}

		existing := findHist(file.Base, appliedMigrations)

		if vers.IsRepeatable(file) {
			// apply only if changed
			if existing == nil || existing.MhHash != file.Hash {
				toApply = append(toApply, file)
			}
		} else {
			// check hash, skip applied
			if existing == nil {
				toApply = append(toApply, file)
			} else {
				if existing.MhHash != file.Hash {
					return nil, fmt.Errorf("hash mismatch, check migration script: %s", filepath.ToSlash(file.Path))
				}
			}
		}
	}
	return toApply, nil
}

// utils

func findHist(base string, appliedMigrations []history.MigrateHistory) *history.MigrateHistory {
	for _, elem := range appliedMigrations {
		if elem.MhName == base {
			return &elem
		}
	}
	return nil
}
