package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"regexp"

	"gopgmigrate/internal/history"
)

func GetPendingMigrations(
	ctx context.Context,
	db *sql.DB,
	migrationDirectory string,
	noTxPatterns map[string]*regexp.Regexp,
	mhRepo history.MigrateHistoryRepository,
) ([]MigrationFile, error) {
	localFiles, err := getFiles(migrationDirectory, noTxPatterns)
	if err != nil {
		return nil, err
	}

	hist, err := mhRepo.ListAll(ctx, db)
	if err != nil {
		return nil, err
	}

	err = checkAppliedHistoryWithLocalFiles(hist, localFiles)
	if err != nil {
		return nil, err
	}

	return getVersionedMigrationsToApply(hist, localFiles)
}

// applied

func checkAppliedHistoryWithLocalFiles(appliedMigrations []history.MigrateHistory, localFiles []MigrationFile) error {
	for _, k := range appliedMigrations {
		if !appliedMigrationPresentLocally(k.MhName, localFiles) {
			return fmt.Errorf("detected applied migration not resolved locally: %s", k.MhName)
		}
	}
	return nil
}

func appliedMigrationPresentLocally(appliedScriptBasename string, localFiles []MigrationFile) bool {
	for _, f := range localFiles {
		if appliedScriptBasename == f.Base {
			return true
		}
	}
	return false
}

// to apply

func getVersionedMigrationsToApply(appliedMigrations []history.MigrateHistory, localFiles []MigrationFile) ([]MigrationFile, error) {
	var toApply []MigrationFile
	for _, file := range localFiles {
		// twice check a file given
		isVersioned := versionedMigrationRegexDo.MatchString(file.Base)
		if !isVersioned {
			continue
		}

		existing := findHist(file.Base, appliedMigrations)

		if isRepeatable(file) {
			// apply only if changed
			if existing == nil || existing.MhHash != file.hash {
				toApply = append(toApply, file)
			}
		} else {
			// check hash, skip applied
			if existing == nil {
				toApply = append(toApply, file)
			} else {
				if existing.MhHash != file.hash {
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
