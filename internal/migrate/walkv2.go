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
	var err error

	err = checkMigrationDirectoryDoesNotContainStrayFiles(migrationDirectory)
	if err != nil {
		return nil, err
	}

	versioned, err := getFilesInAPathV2(migrationDirectory, versionedMigrationRegexDo)
	if err != nil {
		return nil, err
	}

	repeatable, err := getFilesInAPathV2(migrationDirectory, repeatableMigrationRegex)
	if err != nil {
		return nil, err
	}

	err = checkVersionedMigrations(versioned)
	if err != nil {
		return nil, err
	}

	return &MigrationCtx{
		versioned:  versioned,
		repeatable: repeatable,
	}, nil
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

// stray files checking routine

func checkMigrationDirectoryDoesNotContainStrayFiles(migrationDirectory string) error {
	strayFiles, err := getAllStrayFiles(migrationDirectory)
	if err != nil {
		return err
	}
	if len(strayFiles) > 0 {
		for _, sf := range strayFiles {
			slog.Error("stray-file", slog.String("path", sf))
		}
		return fmt.Errorf("stray files are not allowed")
	}
	return nil
}

func getAllStrayFiles(directory string) ([]string, error) {
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

// routine around versioned migrations

func checkVersionedMigrations(versioned []migrationFile) error {
	var err error

	err = checkFilesAreUniqueByVersion(versioned)
	if err != nil {
		return err
	}
	err = checkVersionsAreSequential(versioned)
	if err != nil {
		return err
	}

	return nil
}

func checkFilesAreUniqueByVersion(versioned []migrationFile) error {
	seenVersions := map[int64]bool{}
	for _, f := range versioned {
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

func checkVersionsAreSequential(versioned []migrationFile) error {
	if len(versioned) < 2 {
		return nil
	}
	for i := 1; i < len(versioned); i++ {
		curVer, err := parseVersion(versioned[i].base)
		if err != nil {
			return err
		}
		prevVer, err := parseVersion(versioned[i-1].base)
		if err != nil {
			return err
		}
		if curVer != prevVer+1 {
			return fmt.Errorf("versions are not sequential, check %s and %s",
				filepath.ToSlash(versioned[i-1].path),
				filepath.ToSlash(versioned[i].path),
			)
		}
	}
	return nil
}
