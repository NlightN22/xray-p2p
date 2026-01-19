package common

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
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

// PromptChoice displays a list of options and waits for the user to pick one.
func PromptChoice(question string, options []string) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("no options available")
	}
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s\n", question)
		for idx, option := range options {
			fmt.Printf("  %d) %s\n", idx+1, option)
		}
		fmt.Printf("Select option [1-%d]: ", len(options))
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		answer := strings.TrimSpace(line)
		if answer == "" {
			continue
		}
		choice, err := strconv.Atoi(answer)
		if err != nil || choice < 1 || choice > len(options) {
			fmt.Println("Please enter a valid number.")
			continue
		}
		return options[choice-1], nil
	}
}

// ValidateRFC3986Unreserved ensures the value only uses RFC 3986 unreserved characters.
func ValidateRFC3986Unreserved(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("value is empty")
	}
	for idx, r := range value {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		switch r {
		case '-', '.', '_', '~':
			continue
		default:
			if r > 127 {
				return fmt.Errorf("invalid non-ASCII character at position %d", idx+1)
			}
			return fmt.Errorf("invalid character %q at position %d", r, idx+1)
		}
	}
	return nil
}
