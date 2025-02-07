package migrate

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
)

// computeHash computes SHA256 hash of a file
func computeHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

// directoryExists checks that a given path exists and it's a directory
func directoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
