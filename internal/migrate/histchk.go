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

func checkHistory(appliedNames AppliedHistory, files []migrationFile) error {
	return checkHistoryTableIsSyncedWithLocalFiles(appliedNames, files)
}

func checkHistoryTableIsSyncedWithLocalFiles(migrations AppliedHistory, mf []migrationFile) error {
	for k := range migrations {
		if !found(k, mf) {
			return fmt.Errorf("detected applied migration not resolved locally: %s", k)
		}
	}
	return nil
}

func found(k string, mf []migrationFile) bool {
	for _, f := range mf {
		if k == f.base {
			return true
		}
	}
	return false
}

// to apply

func getVersionedMigrationsToApply(files []migrationFile, hist AppliedHistory) ([]migrationFile, error) {
	var toApply []migrationFile
	for _, file := range files {
		// twice check a file given
		isVersioned := versionedMigrationRegexDo.MatchString(file.base)
		if !isVersioned {
			continue
		}

		existing := findHist(file.base, hist)

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

func findHist(base string, hist AppliedHistory) *AppliedHistoryItem {
	if existing, ok := hist[base]; ok {
		return &existing
	}
	return nil
}

func isRepeatable(file migrationFile) bool {
	return repeatableMigrationRegexDo.MatchString(file.base)
}
