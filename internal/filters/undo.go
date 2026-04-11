package filters

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	"gopgmigrate/internal/history"
	"gopgmigrate/internal/resolver"

	"gopgmigrate/internal/naming"
)

func GetMigrationsForUndo(
	ctx context.Context,
	db *sql.DB,
	migrationDirectory string,
	repo history.MigrateHistoryRepository,
	howMuch int,
) ([]naming.MigrationFile, error) {
	allLocalFiles, err := resolver.GetFiles(migrationDirectory, naming.VersionedMigrationRegexUndo(), repo.GetNoTxPatterns())
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
func getVersionedMigrationsToUndo(files []naming.MigrationFile, hist []history.MigrateHistory, much int) ([]naming.MigrationFile, error) {
	if much > len(hist) {
		return nil, fmt.Errorf("rollback-count is greater that the whole history")
	}

	// Sort history by base (DESC)
	sort.Slice(hist, func(i, j int) bool {
		return hist[i].Name > hist[j].Name
	})

	// create a slice of CNT after sort is applied
	cnt := xMin(len(files), much)
	hist = hist[:cnt]

	// collect UNDO scripts
	resultFiles := []naming.MigrationFile{}
	for _, elem := range hist {
		script, found, err := findCorrespondingUndoScript(files, elem)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("cannot find undo script for %s", elem.Name)
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

func findCorrespondingUndoScript(undoScripts []naming.MigrationFile, doScript history.MigrateHistory) (naming.MigrationFile, bool, error) {
	versionDo, err := naming.ParseVersionDo(doScript.Name)
	if err != nil {
		return naming.MigrationFile{}, false, err
	}
	for _, elem := range undoScripts {
		versionUndo, err := naming.ParseVersionUndo(elem.Base)
		if err != nil {
			return naming.MigrationFile{}, false, err
		}
		if versionUndo == versionDo {
			return elem, true, nil
		}
	}
	return naming.MigrationFile{}, false, nil
}

func xMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
