package common

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// FirstNonEmpty returns the first non-empty string (ignoring surrounding whitespace).
func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// PromptYesNo displays a yes/no question and waits for user confirmation.
func PromptYesNo(question string) (bool, error) {
	fmt.Printf("%s [Y/n]: ", question)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	answer := strings.TrimSpace(strings.ToLower(line))
	switch answer {
	case "", "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		fmt.Println("Please answer 'y' or 'n'.")
		return PromptYesNo(question)
	}
}
