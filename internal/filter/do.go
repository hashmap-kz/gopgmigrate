package filter

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"

	"gopgmigrate/internal/resolver"
	"gopgmigrate/internal/version"

	"gopgmigrate/internal/history"
)

func GetMigrationsForApply(
	ctx context.Context,
	db *sql.DB,
	migrationDirectory string,
	repo history.MigrateHistoryRepository,
) ([]version.MigrationFile, error) {
	allLocalFiles, err := resolver.GetFiles(migrationDirectory, version.VersionedMigrationRegexDo(), repo.GetNoTxPatterns())
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

func checkAppliedHistoryWithLocalFiles(appliedMigrations []history.MigrateHistory, localFiles []version.MigrationFile) error {
	for _, k := range appliedMigrations {
		if !appliedMigrationPresentLocally(k.Name, localFiles) {
			return fmt.Errorf("detected applied migration not resolved locally: %s", k.Name)
		}
	}
	return nil
}

func appliedMigrationPresentLocally(appliedScriptBasename string, localFiles []version.MigrationFile) bool {
	for _, f := range localFiles {
		if appliedScriptBasename == f.Base {
			return true
		}
	}
	return false
}

func getVersionedMigrationsToApply(
	appliedMigrations []history.MigrateHistory,
	localFiles []version.MigrationFile,
) ([]version.MigrationFile, error) {
	var toApply []version.MigrationFile
	for _, file := range localFiles {
		// twice check a file given
		isVersioned := version.IsVersioned(file.Base)
		if !isVersioned {
			continue
		}

		existing := findHist(file.Base, appliedMigrations)

		if version.IsRepeatable(file) {
			// apply only if changed
			if existing == nil || existing.Hash != file.Hash {
				toApply = append(toApply, file)
			}
		} else {
			// check hash, skip applied
			if existing == nil {
				toApply = append(toApply, file)
			} else {
				if existing.Hash != file.Hash {
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
		if elem.Name == base {
			return &elem
		}
	}
	return nil
}
