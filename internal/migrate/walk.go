package migrate

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

// GetFiles walks given directory recursively, sort result by basename
func GetFiles(folder string) (*MigrationCtx, error) {
	schemaFiles, err := getFilesInAPath(filepath.Join(folder, schemaDirName), versionedMigrationRegexDo)
	if err != nil {
		return nil, err
	}
	repeatableFiles, err := getFilesInAPath(filepath.Join(folder, repeatableDirName), repeatableMigrationRegex)
	if err != nil {
		return nil, err
	}
	dataFiles, err := getFilesInAPath(filepath.Join(folder, dataDirName), versionedMigrationRegexDo)
	if err != nil {
		return nil, err
	}
	return &MigrationCtx{
		schema:     schemaFiles,
		repeatable: repeatableFiles,
		data:       dataFiles,
	}, nil
}

// getFilesInAPath walks path, collects all *.sql files
func getFilesInAPath(folder string, reg *regexp.Regexp) ([]migrationFile, error) {
	var files []migrationFile
	err := filepath.WalkDir(folder, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".sql" && reg.MatchString(filepath.Base(path)) {
			sql, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files = append(files, migrationFile{
				path: path,
				base: filepath.Base(path),
				dir:  filepath.Dir(path),
				data: sql,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Sort by base (Ascending)
	sort.Slice(files, func(i, j int) bool {
		return files[i].base < files[j].base
	})
	return files, nil
}
