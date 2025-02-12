package migrate

import "fmt"

type BatchEntries struct {
	Files []MigrationFile
	UseTX bool
}

func ParseFilesIntoBatchEntries(input []MigrationFile) ([]*BatchEntries, error) {
	var batches []*BatchEntries
	var current []MigrationFile

	for i, file := range input {
		// Start a new batch if current batch is empty or if transactional status changes
		if len(current) == 0 || isTx(current[len(current)-1]) == isTx(file) {
			current = append(current, file)
		} else {
			// Store the current batch before starting a new one
			batches = append(batches, &BatchEntries{Files: current})
			current = []MigrationFile{file}
		}
		// Store the last batch at the end
		if i == len(input)-1 {
			batches = append(batches, &BatchEntries{Files: current})
		}
	}

	// Check that we skip nothing
	total := 0
	for _, elem := range batches {
		total = total + len(elem.Files)
	}
	if total != len(input) {
		return nil, fmt.Errorf("error splitting files into batches")
	}

	// Assign TX flags
	for _, elem := range batches {
		if len(elem.Files) > 0 {
			elem.UseTX = isTx(elem.Files[0])
		}
	}

	return batches, nil
}

func isTx(cur MigrationFile) bool {
	res := !versionedMigrationRegexNtx.MatchString(cur.Base)
	return res
}
