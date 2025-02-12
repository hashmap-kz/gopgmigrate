package migrate

import "fmt"

type BatchEntries struct {
	Files []MigrationFile
	UseTX bool
}

func parseFilesIntoBatchEntries(input []MigrationFile) ([]BatchEntries, error) {
	var batches []BatchEntries
	var currentBatch BatchEntries

	for i, file := range input {
		fileIsTx := isTx(file)

		// Start a new batch if current batch is empty or if transactional status changes
		if len(currentBatch.Files) == 0 || isTx(currentBatch.Files[len(currentBatch.Files)-1]) == fileIsTx {
			currentBatch.Files = append(currentBatch.Files, file)
		} else {
			// Store the current batch before starting a new one
			batches = append(batches, currentBatch)
			currentBatch = BatchEntries{
				Files: []MigrationFile{file},
				UseTX: fileIsTx, // Set UseTX based on the new batch's first file
			}
		}

		// Set UseTX for the first file in a batch
		if len(currentBatch.Files) == 1 {
			currentBatch.UseTX = fileIsTx
		}

		// Store the last batch at the end
		if i == len(input)-1 {
			batches = append(batches, currentBatch)
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

	return batches, nil
}

func isTx(cur MigrationFile) bool {
	res := !versionedMigrationRegexNtx.MatchString(cur.Base)
	return res
}
