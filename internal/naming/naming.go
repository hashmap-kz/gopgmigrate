package naming

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// {rev}-{name}.up.sql        versioned, transactional
// {rev}-{name}.r.up.sql      repeatable, transactional
// {rev}-{name}.notx.up.sql   repeatable, transactional
// {rev}-{name}.rnotx.up.sql  repeatable, non-transactional
// {rev}-{name}.down.sql      rollback

// # No ambiguity possible
//
// # all apply files
// find migrations/up -name "*.up.sql"
//
// # repeatable only
// find migrations/up -name "*.r.up.sql" -o -name "*.rnotx.up.sql"
//
// # non-transactional only
// find migrations/up -name "*.notx.up.sql" -o -name "*.rnotx.up.sql"
//
// # repeatable non-transactional only
// find migrations/up -name "*.rnotx.up.sql"

type MigrationFile struct {
	Vers int64
	Path string
	Base string
	Data []byte
	Hash string
}

type MigrationKind string

const (
	MigrationKindUp      MigrationKind = "up"
	MigrationKindRUp     MigrationKind = "r.up"
	MigrationKindNotxUp  MigrationKind = "notx.up"
	MigrationKindRNotxUp MigrationKind = "rnotx.up"
	MigrationKindDown    MigrationKind = "down"
)

type ParsedMigrationName struct {
	Revision int64
	Name     string
	Kind     MigrationKind
}

var (
	// Examples:
	// 0000001-create-users.up.sql
	// 0000002-refresh-user-stats.r.up.sql
	// 0000003-vacuum-big-table.notx.up.sql
	// 0000004-refresh-heavy-view.rnotx.up.sql
	// 0000005-create-users.down.sql
	migrationRegex = regexp.MustCompile(
		`^(\d{7})-([^/]+?)\.(up|r\.up|notx\.up|rnotx\.up|down)\.sql$`,
	)

	// create schema m$yschema1;
	// create table m$yschema1.m$table (id int);
	postgresqlSchemaTablePathRegex = regexp.MustCompile(`(?i)^[a-z_][a-z0-9_$]{0,62}\.[a-z_][a-z0-9_$]{0,62}$`)
)

func MigrationRegex() *regexp.Regexp {
	return migrationRegex
}

func ParseMigrationName(base string) (ParsedMigrationName, error) {
	matches := migrationRegex.FindStringSubmatch(base)
	if matches == nil {
		return ParsedMigrationName{}, fmt.Errorf("invalid migration filename: %s", base)
	}
	if len(matches) != 4 {
		return ParsedMigrationName{}, fmt.Errorf("invalid migration filename: %s", base)
	}

	rev, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return ParsedMigrationName{}, fmt.Errorf("parse revision %q: %w", matches[1], err)
	}

	name := matches[2]
	if err := ValidateMigrationName(name); err != nil {
		return ParsedMigrationName{}, fmt.Errorf("invalid migration filename %q: %w", base, err)
	}

	kind := MigrationKind(matches[3])
	switch kind {
	case MigrationKindUp, MigrationKindRUp, MigrationKindNotxUp, MigrationKindRNotxUp, MigrationKindDown:
	default:
		return ParsedMigrationName{}, fmt.Errorf("invalid migration kind: %s", matches[3])
	}

	return ParsedMigrationName{
		Revision: rev,
		Name:     name,
		Kind:     kind,
	}, nil
}

func ValidateMigrationName(name string) error {
	if name == "" {
		return fmt.Errorf("migration name is empty")
	}
	if strings.ContainsRune(name, '/') {
		return fmt.Errorf("migration name must not contain '/'")
	}
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("migration name must not start with '.'")
	}
	if strings.HasSuffix(name, ".") {
		return fmt.Errorf("migration name must not end with '.'")
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("migration name must not contain '..'")
	}
	return nil
}

func ParseVersionUp(basename string) (int64, error) {
	parsed, err := ParseMigrationName(basename)
	if err != nil {
		return -1, err
	}
	if parsed.Kind == MigrationKindDown {
		return -1, fmt.Errorf("not an up migration filename: %s", basename)
	}
	return parsed.Revision, nil
}

func ParseVersionDown(basename string) (int64, error) {
	parsed, err := ParseMigrationName(basename)
	if err != nil {
		return -1, err
	}
	if parsed.Kind != MigrationKindDown {
		return -1, fmt.Errorf("not a down migration filename: %s", basename)
	}
	return parsed.Revision, nil
}

func ParseVersion(basename string) (int64, error) {
	parsed, err := ParseMigrationName(basename)
	if err != nil {
		return -1, err
	}
	return parsed.Revision, nil
}

func IsSchemaTablePath(what string) bool {
	return postgresqlSchemaTablePathRegex.MatchString(what)
}

func IsTx(file MigrationFile) bool {
	parsed, err := ParseMigrationName(file.Base)
	if err != nil {
		return false
	}

	switch parsed.Kind {
	case MigrationKindUp, MigrationKindRUp, MigrationKindDown:
		return true
	case MigrationKindNotxUp, MigrationKindRNotxUp:
		return false
	default:
		return false
	}
}

func IsRepeatable(file MigrationFile) bool {
	parsed, err := ParseMigrationName(file.Base)
	if err != nil {
		return false
	}
	return parsed.Kind == MigrationKindRUp || parsed.Kind == MigrationKindRNotxUp
}

func IsVersioned(base string) bool {
	parsed, err := ParseMigrationName(base)
	if err != nil {
		return false
	}
	return parsed.Kind != MigrationKindDown
}

func IsUp(base string) bool {
	parsed, err := ParseMigrationName(base)
	return err == nil && parsed.Kind == MigrationKindUp
}

func IsRepeatableUp(base string) bool {
	parsed, err := ParseMigrationName(base)
	return err == nil && parsed.Kind == MigrationKindRUp
}

func IsNonTransactionalUp(base string) bool {
	parsed, err := ParseMigrationName(base)
	return err == nil && parsed.Kind == MigrationKindNotxUp
}

func IsRepeatableNonTransactionalUp(base string) bool {
	parsed, err := ParseMigrationName(base)
	return err == nil && parsed.Kind == MigrationKindRNotxUp
}

func IsDown(base string) bool {
	parsed, err := ParseMigrationName(base)
	return err == nil && parsed.Kind == MigrationKindDown
}

func IsNonTransaction(base string) bool {
	parsed, err := ParseMigrationName(base)
	if err != nil {
		return false
	}
	return parsed.Kind == MigrationKindNotxUp || parsed.Kind == MigrationKindRNotxUp
}
