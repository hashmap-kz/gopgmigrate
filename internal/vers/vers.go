package vers

import (
	"fmt"
	"regexp"
	"strconv"
)

type MigrationFile struct {
	Vers int64
	Path string
	Base string
	Data []byte
	Hash string
}

var (
	// example: 00003-users.do.sql
	// example: 00004-fn_list_users.r.sql
	VersionedMigrationRegexDo = regexp.MustCompile(`^(\d{5})-(.*)(?:\.ntx)?\.(do|r)\.sql$`)

	// example: 00003-users.undo.sql
	// example: 00004-fn_list_users.undo.sql
	VersionedMigrationRegexUndo = regexp.MustCompile(`^(\d{5})-(.*)(?:\.ntx)?\.(undo)\.sql$`)

	// example: 00004-fn_list_users.r.sql
	repeatableMigrationRegexDo = regexp.MustCompile(`^(\d{5})-(.*)(?:\.ntx)?\.(r)\.sql$`)

	// example: 00003-vacuum-users.ntx.do.sql
	// example: 00004-fn_alter_system_1.ntx.r.sql
	versionedMigrationRegexNtx = regexp.MustCompile(`^(\d{5})-(.*)\.ntx\.(do|r)\.sql$`)

	// create schema m$yschema1;
	// create table m$yschema1.m$table (id int);
	postgresqlSchemaTablePathRegex = regexp.MustCompile(`(?i)^[a-z_][a-z0-9_$]{0,62}\.[a-z_][a-z0-9_$]{0,62}$`)
)

func ParseVersionDo(basename string) (int64, error) {
	return ParseVersionByRegex(basename, VersionedMigrationRegexDo)
}

func ParseVersionUndo(basename string) (int64, error) {
	return ParseVersionByRegex(basename, VersionedMigrationRegexUndo)
}

func ParseVersionByRegex(basename string, re *regexp.Regexp) (int64, error) {
	if !re.MatchString(basename) {
		return -1, fmt.Errorf("not a versioned migration filename: %s", basename)
	}

	matches := re.FindStringSubmatch(basename)
	if len(matches) != 4 {
		return -1, fmt.Errorf("not a versioned migration filename: %s", basename)
	}

	versionStr := matches[1]
	if versionStr == "" {
		return -1, fmt.Errorf("unexpected empty version for file: %s", basename)
	}

	parsedResult, err := strconv.ParseInt(versionStr, 10, 64)
	if err != nil {
		return -1, err
	}
	if parsedResult < 0 {
		return -1, fmt.Errorf("not a versioned migration filename: %s", basename)
	}

	return parsedResult, nil
}

func IsSchemaTablePath(what string) bool {
	return postgresqlSchemaTablePathRegex.MatchString(what)
}

func IsTx(file MigrationFile) bool {
	res := !versionedMigrationRegexNtx.MatchString(file.Base)
	return res
}

func IsRepeatable(file MigrationFile) bool {
	return repeatableMigrationRegexDo.MatchString(file.Base)
}

func IsVersioned(base string) bool {
	return VersionedMigrationRegexDo.MatchString(base)
}

func IsUndo(base string) bool {
	return VersionedMigrationRegexUndo.MatchString(base)
}

func IsNonTransaction(base string) bool {
	return versionedMigrationRegexNtx.MatchString(base)
}

func IsOurRegex(r ...*regexp.Regexp) bool {
	for _, elem := range r {
		isOk := elem == VersionedMigrationRegexDo || elem == VersionedMigrationRegexUndo
		if isOk {
			return true
		}
	}
	return false
}
