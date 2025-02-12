package migrate

import (
	"fmt"
	"regexp"
)

var regexPatterns = map[string]*regexp.Regexp{
	"CopyFromStdin":                        regexp.MustCompile(`(?i)COPY( .*)? FROM STDIN`),
	"CreateDatabaseTablespaceSubscription": regexp.MustCompile(`(?i)(CREATE|DROP) (DATABASE|TABLESPACE|SUBSCRIPTION)`),
	"AlterSystem":                          regexp.MustCompile(`(?i)ALTER SYSTEM`),
	"CreateIndexConcurrently":              regexp.MustCompile(`(?i)(CREATE|DROP)( UNIQUE)? INDEX CONCURRENTLY`),
	"Reindex":                              regexp.MustCompile(`(?i)REINDEX( VERBOSE)? (SCHEMA|DATABASE|SYSTEM)`),
	"Vacuum":                               regexp.MustCompile(`(?i)VACUUM`),
	"DiscardAll":                           regexp.MustCompile(`(?i)DISCARD ALL`),
	"AlterTypeAddValue":                    regexp.MustCompile(`(?i)ALTER TYPE( .*)? ADD VALUE`),
}

func checkThatFileIsPossibleShouldNotUseTx(sqlContent string) []string {
	var warnings []string
	for name, pattern := range regexPatterns {
		if pattern.MatchString(sqlContent) {
			warnings = append(warnings, fmt.Sprintf("Warning: Detected %s pattern", name))
		}
	}
	return warnings
}
