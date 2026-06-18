package root

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/NlightN22/xray-p2p/go/internal/cli/commandmap"
)

func newDocsCommand(rootCmd *cobra.Command) *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Generate CLI reference documentation",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path := strings.TrimSpace(dir)
			if path == "" {
				return fmt.Errorf("--dir is required")
			}
			if err := os.MkdirAll(path, 0o755); err != nil {
				return fmt.Errorf("create docs directory: %w", err)
			}
			return doc.GenMarkdownTree(rootCmd, path)
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", "", "destination directory for generated docs")
	_ = cmd.MarkFlagRequired("dir")
	cmd.AddCommand(newCommandMapCommand(rootCmd))
	return cmd
}

func newCommandMapCommand(rootCmd *cobra.Command) *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "command-map",
		Short: "Generate compact command map documentation",
		RunE: func(_ *cobra.Command, _ []string) error {
			path := strings.TrimSpace(dir)
			if path == "" {
				return fmt.Errorf("--dir is required")
			}
			return commandmap.Generate(rootCmd, path)
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", "", "destination directory for generated command map")
	_ = cmd.MarkFlagRequired("dir")
	return cmd
}
