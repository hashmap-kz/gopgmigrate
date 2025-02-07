package migrate

import (
	"fmt"
	"os"
	"path/filepath"
)

// directoryExists checks that a given path exists and it's a directory
func directoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

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
