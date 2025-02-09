package migrate

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

func GetFiles(migrationDirectory string) ([]migrationFile, error) {
	var err error

	err = checkMigrationDirectoryDoesNotContainStrayFiles(migrationDirectory)
	if err != nil {
		return nil, err
	}

	versioned, err := getFilesInAPathV2(migrationDirectory, versionedMigrationRegexDo)
	if err != nil {
		return nil, err
	}

	err = checkVersionedMigrations(versioned)
	if err != nil {
		return nil, err
	}

	return versioned, nil
}

// getFilesInAPath walks path, collects all *.sql files
func getFilesInAPathV2(folder string, reg *regexp.Regexp) ([]migrationFile, error) {
	var files []migrationFile
	err := filepath.WalkDir(folder, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Append any file we found, filter it later
		base := filepath.Base(path)
		if !d.IsDir() && filepath.Ext(path) == ".sql" && reg.MatchString(base) {
			sql, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			vers, err := parseVersionDo(base)
			if err != nil {
				return err
			}
			files = append(files, migrationFile{
				vers: vers,
				path: path,
				base: base,
				data: sql,
				hash: computeHash(sql),
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
			isOk := versionedMigrationRegexDo.MatchString(base) ||
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
	err = checkFirstVersionStartsWithZeroOneOne(versioned)
	if err != nil {
		return err
	}
	err = checkPossibleNoTx(versioned)
	if err != nil {
		return err
	}

	return nil
}

func checkPossibleNoTx(versioned []migrationFile) error {
	for _, elem := range versioned {
		// is already no-transactional file
		if versionedMigrationRegexNtx.MatchString(elem.base) {
			continue
		}
		warnings := checkThatFileIsPossibleShouldNotUseTx(string(elem.data))
		if len(warnings) > 0 {
			for _, w := range warnings {
				slog.Error("notx-statement-detected", slog.String("w", w))
			}
			slog.Error("notx-statement-detected", slog.String("cause", "This may not necessarily be an error; it could be commented-out code that was matched by a pattern."))
			slog.Error("notx-statement-detected", slog.String("cause", "This is handled before any migration runs; otherwise, the database itself would reject to apply this file."))
			slog.Error("notx-statement-detected", slog.String("cause", "Statements that cannot run inside a transaction should be moved to separate files."))
			slog.Error("notx-statement-detected", slog.String("cause", "Consider renaming this file with one of the 'ntx' suffix."))
			return fmt.Errorf("check statements in the file: [%s]",
				filepath.ToSlash(elem.path),
			)
		}
	}
	return nil
}

func checkFirstVersionStartsWithZeroOneOne(versioned []migrationFile) error {
	if len(versioned) == 0 {
		return nil
	}
	first := versioned[0]
	isOk := first.vers == 0 || first.vers == 1
	if !isOk {
		return fmt.Errorf("first migration should begin with 0 or 1, got: %d, check: %s",
			first.vers,
			filepath.ToSlash(first.path),
		)
	}
	return nil
}

func checkFilesAreUniqueByVersion(versioned []migrationFile) error {
	seenVersions := map[int64]bool{}
	for _, f := range versioned {
		if _, ok := seenVersions[f.vers]; ok {
			return fmt.Errorf("%s is used a version that already in use",
				filepath.ToSlash(f.path),
			)
		}
		seenVersions[f.vers] = true
	}
	return nil
}

func checkVersionsAreSequential(versioned []migrationFile) error {
	if len(versioned) < 2 {
		return nil
	}
	for i := 1; i < len(versioned); i++ {
		curr := versioned[i]
		prev := versioned[i-1]
		if curr.vers != prev.vers+1 {
			return fmt.Errorf("versions are not sequential, check %s and %s",
				filepath.ToSlash(prev.path),
				filepath.ToSlash(curr.path),
			)
		}
	}
	return nil
}
