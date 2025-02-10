package migrate

import (
	"fmt"
	"path/filepath"
)

type AppliedHistoryItem struct {
	MhName string
	MhHash string
}

type AppliedHistory map[string]AppliedHistoryItem

// applied

func checkAppliedHistoryWithLocalFiles(appliedMigrations AppliedHistory, localFiles []migrationFile) error {
	return checkHistoryTableIsSyncedWithLocalFiles(appliedMigrations, localFiles)
}

func checkHistoryTableIsSyncedWithLocalFiles(appliedMigrations AppliedHistory, localFiles []migrationFile) error {
	for k := range appliedMigrations {
		if !appliedMigrationPresentLocally(k, localFiles) {
			return fmt.Errorf("detected applied migration not resolved locally: %s", k)
		}
	}
	return nil
}

func appliedMigrationPresentLocally(appliedScriptBasename string, localFiles []migrationFile) bool {
	for _, f := range localFiles {
		if appliedScriptBasename == f.base {
			return true
		}
	}
	return false
}

// to apply

func getVersionedMigrationsToApply(appliedMigrations AppliedHistory, localFiles []migrationFile) ([]migrationFile, error) {
	var toApply []migrationFile
	for _, file := range localFiles {
		// twice check a file given
		isVersioned := versionedMigrationRegexDo.MatchString(file.base)
		if !isVersioned {
			continue
		}

		existing := findHist(file.base, appliedMigrations)

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
					return nil, fmt.Errorf("hash mismatch, check migration script: %s", filepath.ToSlash(file.path))
				}
			}
		}
	}
	return toApply, nil
}

func findHist(base string, appliedMigrations AppliedHistory) *AppliedHistoryItem {
	if existing, ok := appliedMigrations[base]; ok {
		return &existing
	}
	return nil
}

func isRepeatable(file migrationFile) bool {
	return repeatableMigrationRegexDo.MatchString(file.base)
}
