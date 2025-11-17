package tagprompt

import (
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
