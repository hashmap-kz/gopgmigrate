package version

import (
	"fmt"
	"regexp"
	"strconv"
)

// MigrationFile is the in-memory representation of a single SQL file on disk.
type MigrationFile struct {
	Vers int64
	Path string
	Base string
	Data []byte
	Hash string
}

// File naming convention:
//
//	{rev}-{name}.do.sql        versioned, transactional
//	{rev}-{name}.notx.do.sql   versioned, non-transactional
//	{rev}-{name}.r.do.sql      repeatable, transactional
//	{rev}-{name}.rnotx.do.sql  repeatable, non-transactional
//	{rev}-{name}.undo.sql      rollback (always transactional)
//
// rev  = exactly 7 zero-padded digits (e.g. 0000001, 0000042, 1234567)
// name  = one or more characters (any filename-safe chars)
var (
	// matches: 0000001-create-users.do.sql  0000042-vacuum.notx.do.sql
	//          0000003-fn-users.r.do.sql    0000007-fn-users.rnotx.do.sql
	doRegex = regexp.MustCompile(`^(\d{7})-(.+)\.(r|rnotx|notx)?\.?do\.sql$`)

	// matches: 0000001-create-users.undo.sql
	undoRegex = regexp.MustCompile(`^(\d{7})-(.+)\.undo\.sql$`)

	// repeatable: .r.do.sql or .rnotx.do.sql
	repeatableRegex = regexp.MustCompile(`^(\d{7})-(.+)\.(r|rnotx)\.do\.sql$`)

	// non-transactional: .notx.do.sql or .rnotx.do.sql
	notxRegex = regexp.MustCompile(`^(\d{7})-(.+)\.(notx|rnotx)\.do\.sql$`)

	// schema.table path validator
	postgresqlSchemaTablePathRegex = regexp.MustCompile(`(?i)^[a-z_][a-z0-9_$]{0,62}\.[a-z_][a-z0-9_$]{0,62}$`)
)

// --- version parsing ---

func ParseVersionDo(basename string) (int64, error) {
	return parseVersion(basename, doRegex)
}

func ParseVersionUndo(basename string) (int64, error) {
	return parseVersion(basename, undoRegex)
}

func parseVersion(basename string, re *regexp.Regexp) (int64, error) {
	m := re.FindStringSubmatch(basename)
	if len(m) < 2 {
		return -1, fmt.Errorf("not a recognised migration filename: %q", basename)
	}
	v, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil || v < 0 {
		return -1, fmt.Errorf("invalid version in filename %q", basename)
	}
	return v, nil
}

// --- file classification ---

// IsDoFile reports whether base is a valid apply-direction filename.
func IsDoFile(base string) bool {
	return doRegex.MatchString(base)
}

// IsUndoFile reports whether base is a valid rollback filename.
func IsUndoFile(base string) bool {
	return undoRegex.MatchString(base)
}

// IsRepeatable reports whether f is a repeatable migration (.r.do.sql or .rnotx.do.sql).
func IsRepeatable(f MigrationFile) bool {
	return repeatableRegex.MatchString(f.Base)
}

// IsNonTransactional reports whether f runs outside a transaction (.notx.do.sql or .rnotx.do.sql).
func IsNonTransactional(f MigrationFile) bool {
	return notxRegex.MatchString(f.Base)
}

// IsTransactional reports whether f runs inside a transaction.
func IsTransactional(f MigrationFile) bool {
	return !IsNonTransactional(f)
}

// --- regex accessors (for resolver layer) ---

func DoRegex() *regexp.Regexp   { return doRegex }
func UndoRegex() *regexp.Regexp { return undoRegex }

// --- misc ---

func IsSchemaTablePath(s string) bool {
	return postgresqlSchemaTablePathRegex.MatchString(s)
}
