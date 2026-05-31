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

// File is a single SQL file within an entry.
// Path is the manifest-relative forward-slash path used for history storage and display.
// AbsPath is the resolved absolute path used for file I/O.
type File struct {
	Path    string
	AbsPath string
}

// Entry is a single item in the migrations list.
type Entry struct {
	ID          string
	Files       []File
	Mode        Mode
	Description string
}

// Manifest is the parsed, validated, path-resolved manifest.
type Manifest struct {
	Table   string
	Entries []Entry
}

// rawRootManifest is the top-level manifest file.
// It may have either includes or migrations, but not both.
type rawRootManifest struct {
	Manifest struct {
		Table string `json:"table,omitempty"`
	} `json:"manifest,omitempty"`
	Includes   []string   `json:"includes,omitempty"`
	Migrations []rawEntry `json:"migrations,omitempty"`
}

// rawLeafManifest is an included manifest file.
// It may only contain migrations. Includes and manifest-level config are forbidden.
type rawLeafManifest struct {
	Manifest struct {
		Table string `json:"table,omitempty"`
	} `json:"manifest,omitempty"` // forbidden: validated after parse
	Includes   []string   `json:"includes,omitempty"` // forbidden: validated after parse
	Migrations []rawEntry `json:"migrations"`
}

type rawEntry struct {
	ID          string   `json:"id"`
	Files       []string `json:"files"`
	Mode        Mode     `json:"mode,omitempty"`
	Description string   `json:"description,omitempty"`
}

// Load parses and validates a manifest YAML file.
// All file paths in each file are resolved relative to that file's location.
func Load(manifestPath string) (*Manifest, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("manifest: read %q: %w", manifestPath, err)
	}

	var root rawRootManifest
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("manifest: parse %q: %w", manifestPath, err)
	}

	if len(root.Includes) > 0 && len(root.Migrations) > 0 {
		return nil, fmt.Errorf("manifest: %q: 'includes' and 'migrations' cannot both be present", manifestPath)
	}

	table := root.Manifest.Table
	if table == "" {
		table = "schema_migrations"
	}

	rootDir := filepath.Dir(manifestPath)

	var entries []Entry

	if len(root.Includes) > 0 {
		for _, inc := range root.Includes {
			leafPath := filepath.Join(rootDir, inc)
			leafRaws, err := loadLeaf(leafPath)
			if err != nil {
				return nil, err
			}
			leafEntries, err := normalise(leafRaws, filepath.Dir(leafPath))
			if err != nil {
				return nil, fmt.Errorf("manifest: %s: %w", inc, err)
			}
			entries = append(entries, leafEntries...)
		}
		if err := validateGlobal(entries); err != nil {
			return nil, err
		}
	} else {
		entries, err = normalise(root.Migrations, rootDir)
		if err != nil {
			return nil, err
		}
	}

	return &Manifest{
		Table:   table,
		Entries: entries,
	}, nil
}

func loadLeaf(leafPath string) ([]rawEntry, error) {
	data, err := os.ReadFile(leafPath)
	if err != nil {
		return nil, fmt.Errorf("manifest: read included file %q: %w", leafPath, err)
	}

	var leaf rawLeafManifest
	if err := yaml.Unmarshal(data, &leaf); err != nil {
		return nil, fmt.Errorf("manifest: parse included file %q: %w", leafPath, err)
	}

	if len(leaf.Includes) > 0 {
		return nil, fmt.Errorf("manifest: included file %q cannot have 'includes'", leafPath)
	}
	if leaf.Manifest.Table != "" {
		return nil, fmt.Errorf("manifest: included file %q cannot have 'manifest' section", leafPath)
	}

	return leaf.Migrations, nil
}

// validateGlobal checks id and path uniqueness across entries from multiple included files.
func validateGlobal(entries []Entry) error {
	seenIDs := make(map[string]struct{})
	seenPaths := make(map[string]struct{})
	for _, e := range entries {
		if _, dup := seenIDs[e.ID]; dup {
			return fmt.Errorf("manifest: id %q is not unique across included files", e.ID)
		}
		seenIDs[e.ID] = struct{}{}
		for _, f := range e.Files {
			if _, dup := seenPaths[f.AbsPath]; dup {
				return fmt.Errorf("manifest: duplicate path %q across included files", f.Path)
			}
			seenPaths[f.AbsPath] = struct{}{}
		}
	}
	return nil
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

		seenBasenames := make(map[string]string) // basename -> first rel path
		resolved := make([]File, len(r.Files))
		for j, f := range r.Files {
			abs := filepath.Join(dir, f)
			rel := filepath.ToSlash(f)
			if _, dup := seenPaths[abs]; dup {
				return nil, fmt.Errorf("manifest: entry %d: duplicate path %q", n, rel)
			}
			seenPaths[abs] = struct{}{}
			base := filepath.Base(abs)
			if prev, dup := seenBasenames[base]; dup {
				return nil, fmt.Errorf("manifest: entry %d: files %q and %q produce the same migration_id (duplicate basename %q)", n, prev, rel, base)
			}
			seenBasenames[base] = rel
			resolved[j] = File{Path: rel, AbsPath: abs}
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
