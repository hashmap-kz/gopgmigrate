package manifest

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

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

// Entry is a single item in the migrations list.
type Entry struct {
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
	seen := make(map[string]struct{})
	entries := make([]Entry, 0, len(raws))

	for i, r := range raws {
		if len(r.Files) == 0 {
			return nil, fmt.Errorf("manifest: entry %d: 'files' must not be empty", i)
		}

		if r.Mode != ModeDefault &&
			r.Mode != ModeAtomic &&
			r.Mode != ModeNoTx &&
			r.Mode != ModeRepeatable {
			return nil, fmt.Errorf("manifest: entry %d: unknown mode %q", i, r.Mode)
		}

		if r.Mode == ModeRepeatable && len(r.Files) > 1 {
			return nil, fmt.Errorf("manifest: entry %d: repeatable mode supports only one file per entry", i)
		}

		resolved := make([]string, len(r.Files))
		for j, f := range r.Files {
			p := filepath.Join(dir, f)
			if _, dup := seen[p]; dup {
				return nil, fmt.Errorf("manifest: entry %d: duplicate path %q", i, p)
			}
			seen[p] = struct{}{}
			resolved[j] = p
		}

		entries = append(entries, Entry{
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
