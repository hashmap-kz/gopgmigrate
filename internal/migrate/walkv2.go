package migrate

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

func GetFilesV2(migrationDirectory string) (*MigrationCtx, error) {
	strayFiles, err := checkStrayFilesFast(migrationDirectory)
	if err != nil {
		return nil, err
	}
	if len(strayFiles) > 0 {
		for _, sf := range strayFiles {
			slog.Error("stray-file", slog.String("path", sf))
		}
		return nil, fmt.Errorf("stray files are not allowed")
	}

	versioned, err := getFilesInAPathV2(migrationDirectory, versionedMigrationRegexDo)
	if err != nil {
		return nil, err
	}
	repeatable, err := getFilesInAPathV2(migrationDirectory, repeatableMigrationRegex)
	if err != nil {
		return nil, err
	}

	// check versions are correct
	err = checkFilesAreUniqueByVersion(versioned)
	if err != nil {
		return nil, err
	}

	return &MigrationCtx{
		schema:     versioned,
		repeatable: repeatable,
		data:       nil,
	}, nil
}

func checkStrayFilesFast(directory string) ([]string, error) {
	var strayFiles []string
	err := filepath.WalkDir(directory, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			base := filepath.Base(path)
			isOk := repeatableMigrationRegex.MatchString(base) ||
				versionedMigrationRegexDo.MatchString(base) ||
				versionedMigrationRegexUndo.MatchString(base)
			if !isOk {
				strayFiles = append(strayFiles, filepath.ToSlash(path))
			}
		}
		return nil
	})
	return strayFiles, err
}

// getFilesInAPath walks path, collects all *.sql files
func getFilesInAPathV2(folder string, reg *regexp.Regexp) ([]migrationFile, error) {
	var files []migrationFile
	err := filepath.WalkDir(folder, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Append any file we found, filter it later
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

func checkFilesAreUniqueByVersion(files []migrationFile) error {
	seenVersions := map[int64]bool{}
	for _, f := range files {
		version, err := parseVersion(f.base)
		if err != nil {
			return err
		}
		if _, ok := seenVersions[version]; ok {
			return fmt.Errorf("%s is used a version that already in use",
				filepath.ToSlash(f.path),
			)
		}
		seenVersions[version] = true
	}
	return nil
}
