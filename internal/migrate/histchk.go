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

func checkHistory(appliedNames AppliedHistory, files *MigrationCtx) error {
	var err error

	all := files.repeatable
	all = append(all, files.versioned...)

	err = checkHistoryTableIsSyncedWithLocalFiles(appliedNames, all)
	if err != nil {
		return err
	}
	return nil
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

		// skip applied
		existing := findHist(file.base, hist)
		if existing != nil {
			if existing.MhHash != computeHash(file.data) {
				return nil, fmt.Errorf("hash mismatch, check migration script: %s", filepath.ToSlash(file.path))
			}
			continue
		}

		toApply = append(toApply, file)
	}
	return toApply, nil
}

func getRepeatableMigrationsToApply(files []migrationFile, hist AppliedHistory) ([]migrationFile, error) {
	var toApply []migrationFile
	for _, file := range files {
		// twice check a file given
		isRepeatable := repeatableMigrationRegex.MatchString(file.base)
		if !isRepeatable {
			continue
		}
		newHash := computeHash(file.data)

		// Apply only if changed
		existing := findHist(file.base, hist)
		if existing == nil || existing.MhHash != newHash {
			toApply = append(toApply, file)
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
