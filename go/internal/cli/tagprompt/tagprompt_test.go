package tagprompt

import (
	"errors"
	"strings"
	"testing"
)

type panicReader struct{}

func (panicReader) Read([]byte) (int, error) {
	panic("reader should not be used")
}

func TestSelectSingleEntrySkipsPrompt(t *testing.T) {
	entry := Entry{Tag: "auto", Host: "example"}
	selected, err := Select([]Entry{entry}, Options{Reader: panicReader{}})
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}
	if selected != entry {
		t.Fatalf("unexpected selection: got %+v want %+v", selected, entry)
	}
}

func TestSelectMultipleEntries(t *testing.T) {
	entries := []Entry{
		{Tag: "alpha", Host: "edge-a"},
		{Tag: "beta", Host: ""},
		{Tag: "gamma", Host: "edge-g"},
	}
	reader := strings.NewReader("2\n")
	selected, err := Select(entries, Options{Header: "Bindings:", Reader: reader})
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}
	if selected != entries[1] {
		t.Fatalf("unexpected selection (%+v)", selected)
	}
}

func TestSelectAbortAndInvalidInput(t *testing.T) {
	entries := []Entry{
		{Tag: "alpha", Host: ""},
		{Tag: "beta", Host: ""},
	}
	reader := strings.NewReader("\n")
	if _, err := Select(entries, Options{Reader: reader}); !errors.Is(err, ErrAborted) {
		t.Fatalf("expected ErrAborted, got %v", err)
	}

	reader = strings.NewReader("invalid\n1\n")
	selected, err := Select(entries, Options{Reader: reader})
	if err != nil {
		t.Fatalf("Select should accept valid input after invalid: %v", err)
	}
	if selected.Tag != "alpha" {
		t.Fatalf("unexpected selection after retry: %+v", selected)
	}
}

func TestSelectEmptyEntries(t *testing.T) {
	if _, err := Select(nil, Options{}); !errors.Is(err, ErrEmpty) {
		t.Fatalf("expected ErrEmpty for no entries, got %v", err)
	}
}
