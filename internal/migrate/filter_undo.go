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

	// TODO: limits, check is applied, etc...

	hist, err := repo.ListAll(ctx, db)
	if err != nil {
		return nil, err
	}

	return getVersionedMigrationsToUndo(allLocalFiles, hist, howMuch)
}

// TODO: this is a prototype, working ONLY one-by-one (is the latest applied script HAS corresponding undo-script)
func getVersionedMigrationsToUndo(files []MigrationFile, hist []history.MigrateHistory, much int) ([]MigrationFile, error) {
	if much > len(hist) {
		return nil, fmt.Errorf("rollback-count is greater that the whole history")
	}

	// TODO: !!!
	// TODO: get undo migrations ONLY for those scripts that ARE applied at that moment
	// TODO: !!!

	// Sort history by base (DESC)
	sort.Slice(hist, func(i, j int) bool {
		return hist[i].MhName > hist[j].MhName
	})

	// Sort files by base (DESC)
	sort.Slice(files, func(i, j int) bool {
		return files[i].Base > files[j].Base
	})

	// create a slice of CNT after sort is applied
	cnt := min(len(files), much)
	files = files[:cnt]
	hist = hist[:cnt]

	if len(files) != len(hist) {
		return nil, fmt.Errorf("not all rollback scripts present for undo %d steps", much)
	}

	// collect UNDO scripts
	resultFiles := []MigrationFile{}
	for i := 0; i < len(files); i++ {
		fileEntry := files[i]
		histEntry := hist[i]
		if fileEntry.Vers != histEntry.MhVersion {
			return nil, fmt.Errorf("cannot rollback %d steps, latest rollback script is %s, while applied script is %s",
				much,
				fileEntry.Base,
				histEntry.MhName,
			)
		}

		// otherwise, we're able to rollback applied migration
		resultFiles = append(resultFiles, fileEntry)
	}

	return resultFiles, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
