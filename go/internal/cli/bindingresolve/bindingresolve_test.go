package bindingresolve

import (
	"errors"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

type panicReader struct{}

func (panicReader) Read([]byte) (int, error) {
	panic("reader should not be used")
}

func TestResolveSingleCandidateSkipsPrompt(t *testing.T) {
	want := redirect.Binding{Tag: "proxy-a", Host: "edge-a"}
	got, err := Resolve([]redirect.Binding{want}, Options{Reader: panicReader{}})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if got != want {
		t.Fatalf("Resolve = %+v, want %+v", got, want)
	}
}

func TestResolveMultipleCandidatesPrompts(t *testing.T) {
	candidates := []redirect.Binding{
		{Tag: "proxy-a", Host: "edge-a"},
		{Tag: "proxy-b", Host: "edge-b"},
	}
	got, err := Resolve(candidates, Options{Reader: strings.NewReader("2\n")})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if got != candidates[1] {
		t.Fatalf("Resolve = %+v, want %+v", got, candidates[1])
	}
}

func TestResolveMultipleCandidatesQuiet(t *testing.T) {
	_, err := Resolve([]redirect.Binding{
		{Tag: "proxy-a", Host: "edge-a"},
		{Tag: "proxy-b", Host: "edge-b"},
	}, Options{Quiet: true, Reader: panicReader{}})
	if !errors.Is(err, ErrAmbiguous) {
		t.Fatalf("Resolve error = %v, want ErrAmbiguous", err)
	}
}

func TestResolveExplicitHost(t *testing.T) {
	got, err := Resolve([]redirect.Binding{
		{Tag: "proxy-a", Host: "edge-a"},
		{Tag: "proxy-b", Host: "edge-b"},
	}, Options{Host: "EDGE-B", Reader: panicReader{}})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if got.Tag != "proxy-b" || got.Host != "edge-b" {
		t.Fatalf("Resolve = %+v, want proxy-b/edge-b", got)
	}
}
