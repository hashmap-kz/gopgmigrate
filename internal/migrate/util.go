package migrate

import (
	"bufio"
	"strings"
)

func scanLines(from string) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(from))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}
