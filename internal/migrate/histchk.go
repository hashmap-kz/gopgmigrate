package migrate

import (
	"fmt"
)

func checkHistory(appliedNames map[string]bool, files *MigrationCtx) error {
	var err error

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
