package migrate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

func getFiles(migrationDirectory string, noTxPatterns map[string]*regexp.Regexp) ([]MigrationFile, error) {
	var err error

	err = checkMigrationDirectoryDoesNotContainStrayFiles(migrationDirectory)
	if err != nil {
		return nil, err
	}

	versioned, err := getFilesInAPathV2(migrationDirectory, versionedMigrationRegexDo)
	if err != nil {
		return nil, err
	}

	err = checkVersionedMigrations(versioned, noTxPatterns)
	if err != nil {
		return nil, err
	}

	return versioned, nil
}

// getFilesInAPath walks path, collects all *.sql files
func getFilesInAPathV2(folder string, reg *regexp.Regexp) ([]MigrationFile, error) {
	var files []MigrationFile
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
			files = append(files, MigrationFile{
				Vers: vers,
				Path: path,
				Base: base,
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
		return files[i].Base < files[j].Base
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

func checkVersionedMigrations(versioned []MigrationFile, noTxPatterns map[string]*regexp.Regexp) error {
	var err error

	err = checkFilesAreUniqueByVersion(versioned)
	if err != nil {
		return err
	}

	err = checkPossibleNoTx(versioned, noTxPatterns)
	if err != nil {
		return err
	}

	return nil
}

func checkPossibleNoTx(versioned []MigrationFile, noTxPatterns map[string]*regexp.Regexp) error {
	if len(noTxPatterns) == 0 {
		return nil
	}
	for _, elem := range versioned {
		// is already no-transactional file
		if versionedMigrationRegexNtx.MatchString(elem.Base) {
			continue
		}
		warnings := checkThatFileIsPossibleShouldNotUseTx(string(elem.data), noTxPatterns)
		if len(warnings) > 0 {
			for _, w := range warnings {
				slog.Error("notx-statement-detected", slog.String("cause", w))
			}
			slog.Error("notx-statement-detected", slog.String("cause", "This may not necessarily be an error; it could be commented-out code that was matched by a pattern."))
			slog.Error("notx-statement-detected", slog.String("cause", "This is handled before any migration runs to prevent execution errors."))
			slog.Error("notx-statement-detected", slog.String("cause", "Statements that cannot run inside a transaction should be moved to separate files."))
			slog.Error("notx-statement-detected", slog.String("cause", "Consider renaming this file with one of the 'ntx' suffix."))

			return fmt.Errorf("check statements in the file: [%s]",
				filepath.ToSlash(elem.Path),
			)
		}
	}
	return nil
}

func checkThatFileIsPossibleShouldNotUseTx(sqlContent string, noTxPatterns map[string]*regexp.Regexp) []string {
	var warnings []string
	for name, pattern := range noTxPatterns {
		if pattern.MatchString(sqlContent) {
			warnings = append(warnings, fmt.Sprintf("Warning: Detected %s pattern", name))
		}
	}
	return warnings
}

func checkFilesAreUniqueByVersion(versioned []MigrationFile) error {
	seenVersions := map[int64]bool{}
	for _, f := range versioned {
		if _, ok := seenVersions[f.Vers]; ok {
			return fmt.Errorf("%s is used a version that already in use",
				filepath.ToSlash(f.Path),
			)
		}
		seenVersions[f.Vers] = true
	}
	return nil
}

// computeHash computes SHA256 hash of a file
func computeHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}
