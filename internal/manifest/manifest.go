package manifest

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"sigs.k8s.io/yaml"
)

// Mode controls how a migration entry is executed.
type Mode string

const (
	ModeDefault    Mode = ""           // one tx per file
	ModeAtomic     Mode = "atomic"     // one tx across all files
	ModeNoTx       Mode = "no-tx"      // no transaction
	ModeRepeatable Mode = "repeatable" // reruns when checksum changes, one tx per file
)

var validID = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// Entry is a single item in the migrations list.
type Entry struct {
	ID          string
	Files       []string
	Mode        Mode
	Description string
}

type rawManifest struct {
	Manifest struct {
		Table string `json:"table,omitempty"`
	} `json:"manifest,omitempty"`
	Migrations []rawEntry `json:"migrations"`
}

type rawEntry struct {
	ID          string   `json:"id"`
	Files       []string `json:"files"`
	Mode        Mode     `json:"mode,omitempty"`
	Description string   `json:"description,omitempty"`
}

// Manifest is the parsed, validated, path-resolved manifest.
type Manifest struct {
	Table   string
	Entries []Entry
}

// Load parses and validates a manifest YAML file.
// All file paths are resolved relative to the manifest file location.
func Load(manifestPath string) (*Manifest, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("manifest: read %q: %w", manifestPath, err)
	}

	var raw rawManifest
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("manifest: parse %q: %w", manifestPath, err)
	}

	table := raw.Manifest.Table
	if table == "" {
		table = "schema_migrations"
	}

	dir := filepath.Dir(manifestPath)

	entries, err := normalise(raw.Migrations, dir)
	if err != nil {
		return nil, err
	}

	return &Manifest{
		Table:   table,
		Entries: entries,
	}, nil
}

func normalise(raws []rawEntry, dir string) ([]Entry, error) {
	seenPaths := make(map[string]struct{})
	seenIDs := make(map[string]int) // id -> 1-based entry number
	entries := make([]Entry, 0, len(raws))

	for i, r := range raws {
		n := i + 1 // 1-based entry number for error messages

		if r.ID == "" {
			return nil, fmt.Errorf("manifest: entry %d: 'id' is required", n)
		}
		if !validID.MatchString(r.ID) {
			return nil, fmt.Errorf("manifest: entry %d: id %q contains invalid characters (allowed: [a-zA-Z0-9._-])", n, r.ID)
		}
		if prev, dup := seenIDs[r.ID]; dup {
			return nil, fmt.Errorf("manifest: entry %d: id %q is not unique (already declared at entry %d)", n, r.ID, prev)
		}
		seenIDs[r.ID] = n

		if len(r.Files) == 0 {
			return nil, fmt.Errorf("manifest: entry %d: 'files' must not be empty", n)
		}

		if r.Mode != ModeDefault &&
			r.Mode != ModeAtomic &&
			r.Mode != ModeNoTx &&
			r.Mode != ModeRepeatable {
			return nil, fmt.Errorf("manifest: entry %d: unknown mode %q", n, r.Mode)
		}

		seenBasenames := make(map[string]string) // basename -> first file path
		resolved := make([]string, len(r.Files))
		for j, f := range r.Files {
			p := filepath.Join(dir, f)
			if _, dup := seenPaths[p]; dup {
				return nil, fmt.Errorf("manifest: entry %d: duplicate path %q", n, p)
			}
			seenPaths[p] = struct{}{}
			base := filepath.Base(p)
			if prev, dup := seenBasenames[base]; dup {
				return nil, fmt.Errorf("manifest: entry %d: files %q and %q produce the same migration_id (duplicate basename %q)", n, prev, p, base)
			}
			seenBasenames[base] = p
			resolved[j] = p
		}

		entries = append(entries, Entry{
			ID:          r.ID,
			Files:       resolved,
			Mode:        r.Mode,
			Description: r.Description,
		})
	}
	return entries, nil
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
