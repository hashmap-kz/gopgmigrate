package migrate

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// getFilesInAPath walks path, collects all *.sql files
func getFilesInAPath(folder string) ([]migrationFile, error) {
	var files []migrationFile
	err := filepath.WalkDir(folder, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".sql") {
			files = append(files, migrationFile{
				path: path,
				base: filepath.Base(path),
				dir:  filepath.Dir(path),
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

// GetFiles walks given directory recursively, sort result by basename
func GetFiles(folder string) (*migrationCtx, error) {
	schemaFiles, err := getFilesInAPath(filepath.Join(folder, schemaDirName))
	if err != nil {
		return nil, err
	}
	repeatableFiles, err := getFilesInAPath(filepath.Join(folder, repeatableDirName))
	if err != nil {
		return nil, err
	}
	dataFiles, err := getFilesInAPath(filepath.Join(folder, dataDirName))
	if err != nil {
		return nil, err
	}
	return &migrationCtx{
		schema:     schemaFiles,
		repeatable: repeatableFiles,
		data:       dataFiles,
	}, nil
}
