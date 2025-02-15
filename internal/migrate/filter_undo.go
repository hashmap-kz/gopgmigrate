package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	"gopgmigrate/internal/resolve"
	"gopgmigrate/internal/vers"

	"gopgmigrate/internal/history"
)

func getMigrationsForUndo(
	ctx context.Context,
	db *sql.DB,
	migrationDirectory string,
	repo history.MigrateHistoryRepository,
	howMuch int,
) ([]vers.MigrationFile, error) {
	allLocalFiles, err := resolve.GetFiles(migrationDirectory, vers.VersionedMigrationRegexUndo, repo.GetNoTxPatterns())
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
func getVersionedMigrationsToUndo(files []vers.MigrationFile, hist []history.MigrateHistory, much int) ([]vers.MigrationFile, error) {
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
	resultFiles := []vers.MigrationFile{}
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

func findCorrespondingUndoScript(undoScripts []vers.MigrationFile, doScript history.MigrateHistory) (vers.MigrationFile, bool, error) {
	versionDo, err := vers.ParseVersionDo(doScript.MhName)
	if err != nil {
		return vers.MigrationFile{}, false, err
	}
	for _, elem := range undoScripts {
		versionUndo, err := vers.ParseVersionUndo(elem.Base)
		if err != nil {
			return vers.MigrationFile{}, false, err
		}
		if versionUndo == versionDo {
			return elem, true, nil
		}
	}
	return vers.MigrationFile{}, false, nil
}

func xMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
