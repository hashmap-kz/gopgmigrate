package migrate

import (
	"fmt"
	"path/filepath"
)

// CheckMigrationDirectory checks that the migration-directory structure is conforming for all rules
func CheckMigrationDirectory(folder string) error {
	requiredDirs := []string{schemaDirName, repeatableDirName, dataDirName}
	for _, dir := range requiredDirs {
		fullPath := filepath.Join(folder, dir)
		if !directoryExists(fullPath) {
			return fmt.Errorf("%s directory does not exist in: %s", dir, folder)
		}
	}
	return nil
}
