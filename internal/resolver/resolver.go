package resolver

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"gopgmigrate/internal/naming"
)

func GetFiles(
	migrationDirectory string,
	reg *regexp.Regexp,
	noTxPatterns map[string]*regexp.Regexp,
) ([]naming.MigrationFile, error) {
	var err error

	err = checkMigrationDirectoryDoesNotContainStrayFiles(migrationDirectory)
	if err != nil {
		return nil, err
	}

	versioned, err := getFilesInAPathV2(migrationDirectory, reg)
	if err != nil {
		return nil, err
	}

	err = checkPossibleNoTx(versioned, noTxPatterns)
	if err != nil {
		return nil, err
	}

	return versioned, nil
}

// getFilesInAPath walks path, collects all *.sql files
func getFilesInAPathV2(folder string, reg *regexp.Regexp) ([]naming.MigrationFile, error) {
	var files []naming.MigrationFile
	err := filepath.WalkDir(folder, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Append any file we found, filter it later
		base := filepath.Base(path)
		if !d.IsDir() && filepath.Ext(path) == ".sql" && reg.MatchString(base) {
			//TODO: use os.Root()
			//nolint:gosec
			sql, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			v, err := naming.ParseMigrationName(base)
			if err != nil {
				return err
			}
			files = append(files, naming.MigrationFile{
				Vers: v.Revision,
				Path: path,
				Base: base,
				Data: sql,
				Hash: computeHash(sql),
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Sort by base (ASC)
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
			m, err := naming.ParseMigrationName(filepath.Base(path))
			if err != nil {
				fmt.Println(m)
				strayFiles = append(strayFiles, filepath.ToSlash(path))
			}
		}
		return nil
	})
	return strayFiles, err
}

// routine around versioned migrations

func checkPossibleNoTx(versioned []naming.MigrationFile, noTxPatterns map[string]*regexp.Regexp) error {
	if len(noTxPatterns) == 0 {
		return nil
	}
	for _, elem := range versioned {
		// is already no-transactional file
		if naming.IsNonTransaction(elem.Base) {
			continue
		}
		warnings := checkThatFileIsPossibleShouldNotUseTx(string(elem.Data), noTxPatterns)
		if len(warnings) > 0 {
			for _, w := range warnings {
				slog.Error("notx-statement-detected", slog.String("cause", w))
			}
			slog.Error("notx-statement-detected",
				slog.String("cause", "This may not necessarily be an error; it could be commented-out code that was matched by a pattern."),
			)
			slog.Error("notx-statement-detected",
				slog.String("cause", "This is handled before any migration runs to prevent execution errors."),
			)
			slog.Error("notx-statement-detected",
				slog.String("cause", "Statements that cannot run inside a transaction should be moved to separate files."),
			)
			slog.Error("notx-statement-detected",
				slog.String("cause", "Consider renaming this file with one of the 'ntx' suffix."),
			)

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

func computeHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}
