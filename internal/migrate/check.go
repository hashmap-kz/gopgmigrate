package migrate

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/jackc/pgx/v5"
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

func CheckHistory(conn *pgx.Conn, files *MigrationCtx) error {
	// TODO: simplify, optimize

	schemaMigrations, err := getAppliedMigrationsInternal(conn, schemaDirName)
	if err != nil {
		return err
	}
	repeatableMigrations, err := getAppliedMigrationsInternal(conn, repeatableDirName)
	if err != nil {
		return err
	}
	dataMigrations, err := getAppliedMigrationsInternal(conn, dataDirName)
	if err != nil {
		return err
	}

	err = checkHistoryTableIsSyncedWithLocalFiles(schemaMigrations, files.schema)
	if err != nil {
		return err
	}
	err = checkHistoryTableIsSyncedWithLocalFiles(repeatableMigrations, files.repeatable)
	if err != nil {
		return err
	}
	err = checkHistoryTableIsSyncedWithLocalFiles(dataMigrations, files.data)
	if err != nil {
		return err
	}

	return err
}

func checkHistoryTableIsSyncedWithLocalFiles(migrations map[string]struct{}, mf []migrationFile) error {
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

func getAppliedMigrationsInternal(conn *pgx.Conn, mode string) (map[string]struct{}, error) {
	query := fmt.Sprintf("SELECT name FROM %s where mode = $1", defaultHistoryTableName)
	rows, err := conn.Query(context.Background(), query, mode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	migrations := make(map[string]struct{})
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		migrations[version] = struct{}{}
	}

	return migrations, nil
}
