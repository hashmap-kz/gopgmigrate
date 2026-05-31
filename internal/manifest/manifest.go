package manifest

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Mode controls how a migration file is executed.
type Mode string

const (
	ModeDefault        Mode = ""                 // versioned: runs once, in a transaction
	ModeNoTx           Mode = "no-tx"            // versioned: runs once, outside a transaction
	ModeRepeatable     Mode = "repeatable"       // re-runs when checksum changes, in a transaction
	ModeRepeatableNoTx Mode = "repeatable-no-tx" // re-runs when checksum changes, outside a transaction
)

const defaultTable = "schema_migrations"

// File is a single SQL file within an entry.
// Path is the dir-relative forward-slash path used for history storage and display.
// AbsPath is the absolute path used for file I/O.
type File struct {
	Path    string
	AbsPath string
}

// Entry is a single migration file ready for execution.
type Entry struct {
	ID       string
	Revision int64
	Files    []File
	Mode     Mode
}

// Manifest is the result of scanning a migrations directory.
type Manifest struct {
	Table   string
	Entries []Entry
}

// StrayFilesError is returned when Scan finds files that do not match
// the migration naming convention. All offending paths are collected before
// returning so the caller sees the full list in one pass.
type StrayFilesError struct {
	Files []string
}

func (e *StrayFilesError) Error() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "scan: %d stray file(s) found (files must match {0000000}-{name}.{ext}.sql):", len(e.Files))
	for _, f := range e.Files {
		sb.WriteString("\n  ")
		sb.WriteString(f)
	}
	return sb.String()
}

// migrationRE matches filenames of the form {7digits}-{name}.{kind}.sql.
// Alternation is ordered longest-first so rnotx is tested before notx.
var migrationRE = regexp.MustCompile(`^(\d{7})-(.+?)\.(rnotx|notx|r|up)\.sql$`)

type parsedFilename struct {
	revision int64
	stem     string
	mode     Mode
}

// parseFilename extracts the revision, stem, and mode from a migration filename.
// Returns the parsed result and ok=true when the name matches the convention.
func parseFilename(name string) (parsedFilename, bool) {
	m := migrationRE.FindStringSubmatch(name)
	if m == nil {
		return parsedFilename{}, false
	}
	rev, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return parsedFilename{}, false
	}
	return parsedFilename{
		revision: rev,
		stem:     m[1] + "-" + m[2],
		mode:     kindToMode(m[3]),
	}, true
}

func kindToMode(kind string) Mode {
	switch kind {
	case "rnotx":
		return ModeRepeatableNoTx
	case "notx":
		return ModeNoTx
	case "r":
		return ModeRepeatable
	default:
		return ModeDefault
	}
}

type scannedFile struct {
	rev  int64
	stem string
	path string // dir-relative forward-slash path
	abs  string
	mode Mode
}

// Scan walks dir recursively for SQL migration files matching the
// {0000000}-{name}.{ext}.sql naming convention. Every file that does not
// match is collected into a StrayFilesError returned after the full walk.
// On success, entries are sorted by revision; duplicate revisions are an error.
func Scan(dir string) (*Manifest, error) {
	var files []scannedFile
	var stray []string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, ok := parseFilename(d.Name())
		if !ok {
			rel, relErr := filepath.Rel(dir, path)
			if relErr != nil {
				rel = path
			}
			stray = append(stray, filepath.ToSlash(rel))
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		files = append(files, scannedFile{
			rev:  info.revision,
			stem: info.stem,
			path: filepath.ToSlash(rel),
			abs:  path,
			mode: info.mode,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan %q: %w", dir, err)
	}

	if len(stray) > 0 {
		return nil, &StrayFilesError{Files: stray}
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].rev < files[j].rev
	})

	for i := 1; i < len(files); i++ {
		if files[i].rev == files[i-1].rev {
			return nil, fmt.Errorf("scan: duplicate revision %07d: %q and %q",
				files[i].rev, files[i-1].path, files[i].path)
		}
	}

	entries := make([]Entry, len(files))
	for i, f := range files {
		entries[i] = Entry{
			ID:       f.stem,
			Revision: f.rev,
			Files:    []File{{Path: f.path, AbsPath: f.abs}},
			Mode:     f.mode,
		}
	}

	return &Manifest{
		Table:   defaultTable,
		Entries: entries,
	}, nil
}

// Checksum computes SHA256 of a file's contents.
func Checksum(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("checksum %q: %w", path, err)
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum), nil
}

// ReadFile returns the contents of a SQL file as a string.
func ReadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %q: %w", path, err)
	}
	return string(data), nil
}
