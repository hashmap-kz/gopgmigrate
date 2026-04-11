package filters

import (
	"context"
	"fmt"
	"path/filepath"

	"gopgmigrate/internal/history"
	"gopgmigrate/internal/resolver"

	"gopgmigrate/internal/naming"
)

func GetMigrationsForApply(
	_ context.Context,
	hist []history.MigrateHistory,
	migrationDirectory string,
) ([]naming.MigrationFile, error) {
	allLocalFiles, err := resolver.GetFiles(
		migrationDirectory,
		naming.MigrationRegex(),
		getNoTxPatterns(),
	)
	if err != nil {
		return nil, err
	}

	upFiles := filterMigrationFiles(allLocalFiles, func(f naming.MigrationFile) bool {
		return naming.IsVersioned(f.Base)
	})

	if err := checkAppliedHistoryWithLocalFiles(hist, upFiles); err != nil {
		return nil, err
	}

	toApply, err := getVersionedMigrationsToApply(hist, upFiles)
	return toApply, err
}

func checkAppliedHistoryWithLocalFiles(appliedMigrations []history.MigrateHistory, localFiles []naming.MigrationFile) error {
	for _, k := range appliedMigrations {
		if !appliedMigrationPresentLocally(k.Name, localFiles) {
			return fmt.Errorf("detected applied migration not resolved locally: %s", k.Name)
		}
	}
	return nil
}

func appliedMigrationPresentLocally(appliedScriptBasename string, localFiles []naming.MigrationFile) bool {
	for _, f := range localFiles {
		if appliedScriptBasename == f.Base {
			return true
		}
	}
	return false
}

func getVersionedMigrationsToApply(
	appliedMigrations []history.MigrateHistory,
	localFiles []naming.MigrationFile,
) ([]naming.MigrationFile, error) {
	var toApply []naming.MigrationFile
	for _, file := range localFiles {
		// twice check a file given
		isVersioned := naming.IsVersioned(file.Base)
		if !isVersioned {
			continue
		}

		existing := findHist(file.Base, appliedMigrations)

		if naming.IsRepeatable(file) {
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
