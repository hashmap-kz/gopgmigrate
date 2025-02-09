package migrate

import (
	"fmt"
	"regexp"
	"strconv"
)

var (
	// example: 00003-users.do.sql
	versionedMigrationRegexDo = regexp.MustCompile(`^(\d{5})-([a-zA-Z0-9_-]+)\.(do|dontx|r|rntx)\.sql$`)

	// example: 00003-users.undo.sql
	versionedMigrationRegexUndo = regexp.MustCompile(`^(\d{5})-([a-zA-Z0-9_-]+)\.(undo|undontx)\.sql$`)

	// example: 00009-fn_get_roles.r.sql
	repeatableMigrationRegexDo = regexp.MustCompile(`^(\d{5})-([a-zA-Z0-9_-]+)\.(r|rntx)\.sql$`)
)

func parseVersionDo(basename string) (int64, error) {
	return parseVersionByRegex(basename, versionedMigrationRegexDo)
}

func parseVersionUndo(basename string) (int64, error) {
	return parseVersionByRegex(basename, versionedMigrationRegexUndo)
}

func parseVersionByRegex(basename string, re *regexp.Regexp) (int64, error) {
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
