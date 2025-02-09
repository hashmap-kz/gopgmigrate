package migrate

import (
	"fmt"

	"gopgmigrate/internal/migrate_history"
)

func checkHistory(hist []migrate_history.MigrateHistory, files *MigrationCtx) error {
	var err error

	appliedNames := map[string]bool{}
	for _, elem := range hist {
		appliedNames[elem.MhName] = true
	}

	all := files.repeatable
	all = append(all, files.versioned...)

	err = checkHistoryTableIsSyncedWithLocalFiles(appliedNames, all)
	if err != nil {
		return err
	}
	return nil
}

func checkHistoryTableIsSyncedWithLocalFiles(migrations map[string]bool, mf []migrationFile) error {
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
