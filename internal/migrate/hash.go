package migrate

import (
	"crypto/sha256"
	"encoding/hex"
)

// computeHash computes SHA256 hash of a file
func computeHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}
