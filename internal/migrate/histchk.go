package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"

	"gopgmigrate/internal/history"
)

type AppliedHistoryItem struct {
	MhName string
	MhHash string
}

type AppliedHistory map[string]AppliedHistoryItem

func GetPendingMigrations(
	ctx context.Context,
	conn *sql.DB,
	localFiles []MigrationFile,
	mhRepo history.MigrateHistoryRepository,
) ([]MigrationFile, error) {
	appliedMigrations, err := fetchHistory(ctx, conn, localFiles, mhRepo)
	if err != nil {
		return nil, err
	}
	return getVersionedMigrationsToApply(appliedMigrations, localFiles)
}

// applied

func fetchHistory(
	ctx context.Context,
	conn *sql.DB,
	localFiles []MigrationFile,
	mhRepo history.MigrateHistoryRepository,
) (AppliedHistory, error) {
	var err error

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	err = mhRepo.CreateHistoryTable(ctx, tx)
	if err != nil {
		return nil, err
	}

	// check that all applied migrations are present in files list
	migrateHistory, err := mhRepo.ListAll(ctx, tx)
	if err != nil {
		return nil, err
	}

	appliedMigrations := createAppliedHistoryIndex(migrateHistory)
	err = checkAppliedHistoryWithLocalFiles(appliedMigrations, localFiles)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return appliedMigrations, nil
}

func checkAppliedHistoryWithLocalFiles(appliedMigrations AppliedHistory, localFiles []MigrationFile) error {
	return checkHistoryTableIsSyncedWithLocalFiles(appliedMigrations, localFiles)
}

func checkHistoryTableIsSyncedWithLocalFiles(appliedMigrations AppliedHistory, localFiles []MigrationFile) error {
	for k := range appliedMigrations {
		if !appliedMigrationPresentLocally(k, localFiles) {
			return fmt.Errorf("detected applied migration not resolved locally: %s", k)
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

func getVersionedMigrationsToApply(appliedMigrations AppliedHistory, localFiles []MigrationFile) ([]MigrationFile, error) {
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

func findHist(base string, appliedMigrations AppliedHistory) *AppliedHistoryItem {
	if existing, ok := appliedMigrations[base]; ok {
		return &existing
	}
	return nil
}

func isRepeatable(file MigrationFile) bool {
	return repeatableMigrationRegexDo.MatchString(file.Base)
}

func createAppliedHistoryIndex(hist []history.MigrateHistory) AppliedHistory {
	r := AppliedHistory{}
	for _, elem := range hist {
		r[elem.MhName] = AppliedHistoryItem{
			MhName: elem.MhName,
			MhHash: elem.MhHash,
		}
	}
	return r
}
