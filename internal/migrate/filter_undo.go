package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	"gopgmigrate/internal/history"
)

func getMigrationsForUndo(
	ctx context.Context,
	db *sql.DB,
	migrationDirectory string,
	repo history.MigrateHistoryRepository,
	howMuch int,
) ([]MigrationFile, error) {
	allLocalFiles, err := getFiles(migrationDirectory, versionedMigrationRegexUndo, repo.GetNoTxPatterns())
	if err != nil {
		return nil, err
	}

	hist, err := repo.ListAll(ctx, db)
	if err != nil {
		return nil, err
	}

	return getVersionedMigrationsToUndo(allLocalFiles, hist, howMuch)
}

// TODO: this is a prototype, working ONLY one-by-one (when the latest applied script HAS corresponding undo-script)
func getVersionedMigrationsToUndo(files []MigrationFile, hist []history.MigrateHistory, much int) ([]MigrationFile, error) {
	if much > len(hist) {
		return nil, fmt.Errorf("rollback-count is greater that the whole history")
	}

	// Sort history by base (DESC)
	sort.Slice(hist, func(i, j int) bool {
		return hist[i].MhName > hist[j].MhName
	})

	// create a slice of CNT after sort is applied
	cnt := xMin(len(files), much)
	hist = hist[:cnt]

	// collect UNDO scripts
	resultFiles := []MigrationFile{}
	for _, elem := range hist {
		script, found, err := findCorrespondingUndoScript(files, elem)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("cannot find undo script for %s", elem.MhName)
		}
		resultFiles = append(resultFiles, script)
	}

	if len(resultFiles) != len(hist) {
		return nil, fmt.Errorf("cannot rollback, not all applied scripts have corresponding undo scripts")
	}

	// Sort result-files by base (DESC)
	sort.Slice(resultFiles, func(i, j int) bool {
		return resultFiles[i].Base > resultFiles[j].Base
	})

	return resultFiles, nil
}

func findCorrespondingUndoScript(undoScripts []MigrationFile, doScript history.MigrateHistory) (MigrationFile, bool, error) {
	versionDo, err := parseVersionDo(doScript.MhName)
	if err != nil {
		return MigrationFile{}, false, err
	}
	for _, elem := range undoScripts {
		versionUndo, err := parseVersionUndo(elem.Base)
		if err != nil {
			return MigrationFile{}, false, err
		}
		if versionUndo == versionDo {
			return elem, true, nil
		}
	}
	return MigrationFile{}, false, nil
}

func xMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
