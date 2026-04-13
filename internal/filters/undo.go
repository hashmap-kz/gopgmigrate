package filters

import (
	"context"
	"fmt"
	"sort"

	"gopgmigrate/internal/history"
	"gopgmigrate/internal/resolver"

	"gopgmigrate/internal/naming"
)

func GetMigrationsForUndo(
	_ context.Context,
	hist []history.MigrateHistory,
	migrationDirectory string,
	howMuch int,
) ([]naming.MigrationFile, error) {
	allLocalFiles, err := resolver.GetFiles(
		migrationDirectory,
		naming.MigrationRegex(),
		getNoTxPatterns(),
	)
	if err != nil {
		return nil, err
	}

	undoFiles := filterMigrationFiles(allLocalFiles, func(f naming.MigrationFile) bool {
		return naming.IsDown(f.Base)
	})

	toUndo, err := getVersionedMigrationsToUndo(undoFiles, hist, howMuch)
	if err != nil {
		return nil, err
	}

	if err := checkFilesAreUniqueByVersion(toUndo); err != nil {
		return nil, err
	}

	return toUndo, err
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
		script, found, err := findCorrespondingUndoScript(files, &elem)
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

func findCorrespondingUndoScript(
	undoScripts []naming.MigrationFile,
	doScript *history.MigrateHistory,
) (naming.MigrationFile, bool, error) {
	versionDo, err := naming.ParseVersion(doScript.Name)
	if err != nil {
		return naming.MigrationFile{}, false, err
	}

	undoByVersion := make(map[int64]naming.MigrationFile, len(undoScripts))
	for _, elem := range undoScripts {
		versionUndo, err := naming.ParseVersion(elem.Base)
		if err != nil {
			return naming.MigrationFile{}, false, err
		}
		undoByVersion[versionUndo] = elem
	}

	found, ok := undoByVersion[versionDo]
	if !ok {
		return naming.MigrationFile{}, false, nil
	}

	return found, true, nil
}

func xMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
