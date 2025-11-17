package tagprompt

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Entry represents a selectable outbound binding.
type Entry struct {
	Tag  string
	Host string
}

// Options control how Select renders prompts.
type Options struct {
	Header string
	Reader io.Reader
}

var (
	// ErrEmpty indicates that no entries are available for selection.
	ErrEmpty = errors.New("tagprompt: no entries available")
	// ErrAborted indicates that the user cancelled the prompt.
	ErrAborted = errors.New("tagprompt: selection aborted")
)

// Select renders entries and prompts the user to choose one.
func Select(entries []Entry, opts Options) (Entry, error) {
	if len(entries) == 0 {
		return Entry{}, ErrEmpty
	}

	header := strings.TrimSpace(opts.Header)
	if header == "" {
		header = "Available entries:"
	}
	fmt.Println(header)
	for idx, entry := range entries {
		host := strings.TrimSpace(entry.Host)
		if host == "" {
			host = "-"
		}
		fmt.Printf("%d) %s\t%s\n", idx+1, entry.Tag, host)
	}

	reader := opts.Reader
	if reader == nil {
		reader = os.Stdin
	}
	buf := bufio.NewReader(reader)
	for {
		fmt.Print("Select outbound tag (enter number): ")
		line, err := buf.ReadString('\n')
		trimmed := strings.TrimSpace(line)
		if err != nil && trimmed == "" {
			if errors.Is(err, io.EOF) {
				return Entry{}, ErrAborted
			}
			return Entry{}, err
		}
		if trimmed == "" {
			return Entry{}, ErrAborted
		}

		idx, convErr := strconv.Atoi(trimmed)
		if convErr != nil || idx < 1 || idx > len(entries) {
			fmt.Println("Invalid selection. Enter a number from the list.")
			if err != nil && errors.Is(err, io.EOF) {
				return Entry{}, ErrAborted
			}
			continue
		}
		return entries[idx-1], nil
	}
}
