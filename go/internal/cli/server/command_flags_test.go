package servercmd

import (
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func TestServerBundleFlags(t *testing.T) {
	cmd := NewCommand(func() config.Config { return config.Config{} })
	exportCmd, _, err := cmd.Find([]string{"export"})
	if err != nil {
		t.Fatalf("find export command: %v", err)
	}
	if flag := exportCmd.Flags().Lookup("config-root"); flag == nil || flag.Shorthand != "C" {
		t.Fatalf("expected export --config-root shorthand -C")
	}
	if flag := exportCmd.Flags().Lookup("output"); flag == nil || flag.Shorthand != "o" {
		t.Fatalf("expected export --output shorthand -o")
	}

	importCmd, _, err := cmd.Find([]string{"import"})
	if err != nil {
		t.Fatalf("find import command: %v", err)
	}
	if flag := importCmd.Flags().Lookup("config-root"); flag == nil || flag.Shorthand != "C" {
		t.Fatalf("expected import --config-root shorthand -C")
	}
	if flag := importCmd.Flags().Lookup("input"); flag == nil || flag.Shorthand != "i" {
		t.Fatalf("expected import --input shorthand -i")
	}
}

func TestServerCertSetQuietFlag(t *testing.T) {
	cmd := NewCommand(func() config.Config { return config.Config{} })
	certSetCmd, _, err := cmd.Find([]string{"cert", "set"})
	if err != nil {
		t.Fatalf("find cert set command: %v", err)
	}
	if flag := certSetCmd.Flags().Lookup("quiet"); flag == nil || flag.Shorthand != "q" {
		t.Fatalf("expected cert set --quiet shorthand -q")
	}
}
