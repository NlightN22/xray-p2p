package commandmap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestGenerateWritesCompactCommandMap(t *testing.T) {
	root := &cobra.Command{
		Use:   "xp2p",
		Short: "Test root",
	}
	root.PersistentFlags().StringP("config", "c", "", "path to configuration file")

	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "Manage server",
	}
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List users",
	}
	listCmd.Flags().StringP("host", "H", "", "public host")
	_ = listCmd.MarkFlagRequired("host")
	serverCmd.AddCommand(listCmd)
	root.AddCommand(serverCmd)

	dir := t.TempDir()
	if err := Generate(root, dir); err != nil {
		t.Fatalf("generate command map: %v", err)
	}

	rootDoc := readDoc(t, filepath.Join(dir, "xp2p.md"))
	if !strings.Contains(rootDoc, "- --help, -h show help for command") {
		t.Fatalf("root doc missing global help option:\n%s", rootDoc)
	}
	if !strings.Contains(rootDoc, "Subcommands: list") {
		t.Fatalf("root doc missing top-level child subcommands:\n%s", rootDoc)
	}
	if strings.Contains(rootDoc, "xp2p server list") {
		t.Fatalf("root doc should not expand nested commands:\n%s", rootDoc)
	}

	serverDoc := readDoc(t, filepath.Join(dir, "xp2p_server.md"))
	if !strings.Contains(serverDoc, "xp2p server list") {
		t.Fatalf("server doc missing nested command:\n%s", serverDoc)
	}
	if !strings.Contains(serverDoc, "--host, -H <host> (required) public host") {
		t.Fatalf("server doc missing required flag:\n%s", serverDoc)
	}
	if strings.Contains(serverDoc, "--help") {
		t.Fatalf("server doc should not include local help flag:\n%s", serverDoc)
	}
}

func readDoc(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
