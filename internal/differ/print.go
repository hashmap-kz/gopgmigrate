package differ

import (
	"fmt"
	"strings"
)

const (
	ansiReset = "\033[0m"
	ansiRed   = "\033[31m"
	ansiGreen = "\033[32m"
	ansiCyan  = "\033[36m"
	ansiBold  = "\033[1m"
)

// printSection prints a titled diff block to stdout. When color is true,
// added lines are green, removed lines are red, and hunk headers are cyan.
func printSection(title, diff string, color bool) {
	const hrPlain = "------------------------------------------------------------------------"

	if color {
		fmt.Printf("\n%s%s%s\n%s\n", ansiBold, title, ansiReset, hrPlain)
	} else {
		fmt.Printf("\n%s\n%s\n", title, hrPlain)
	}

	trimmed := strings.TrimSpace(diff)
	if trimmed == "" {
		fmt.Println("no changes")
		return
	}

	if !color {
		fmt.Print(diff)
		if !strings.HasSuffix(diff, "\n") {
			fmt.Println()
		}
		return
	}

	lines := strings.Split(diff, "\n")
	// drop the trailing empty element that Split produces for a newline-terminated string
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			fmt.Printf("%s%s%s\n", ansiBold+ansiCyan, line, ansiReset)
		case strings.HasPrefix(line, "@@"):
			fmt.Printf("%s%s%s\n", ansiCyan, line, ansiReset)
		case strings.HasPrefix(line, "+"):
			fmt.Printf("%s%s%s\n", ansiGreen, line, ansiReset)
		case strings.HasPrefix(line, "-"):
			fmt.Printf("%s%s%s\n", ansiRed, line, ansiReset)
		default:
			fmt.Println(line)
		}
	}
}
