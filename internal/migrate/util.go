package migrate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
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

func parseVersion(basename string) (int64, error) {
	versionStr := strings.Split(basename, "-")[0]
	if versionStr == "" {
		return -1, fmt.Errorf("unexpected empty version for file: %s", basename)
	}
	return strconv.ParseInt(versionStr, 10, 64)
}
