package servercmd

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/configbundle"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

type serverExportOptions struct {
	ConfigRoot string
	Output     string
}

type serverImportOptions struct {
	ConfigRoot string
	Input      string
}

func newServerExportCmd(cfg commandConfig) *cobra.Command {
	var opts serverExportOptions
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export server configuration bundle",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerExport(cfg(), opts)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&opts.ConfigRoot, "config-root", "C", "", "configuration root to export")
	flags.StringVarP(&opts.Output, "output", "o", "", "archive output path")
	return cmd
}

func newServerImportCmd(cfg commandConfig) *cobra.Command {
	var opts serverImportOptions
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import server configuration bundle",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerImport(cfg(), opts)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&opts.ConfigRoot, "config-root", "C", "", "configuration root to import into")
	flags.StringVarP(&opts.Input, "input", "i", "", "archive input path")
	_ = cmd.MarkFlagRequired("input")
	return cmd
}

func runServerExport(_ config.Config, opts serverExportOptions) int {
	root := strings.TrimSpace(opts.ConfigRoot)
	if root == "" {
		root = config.ConfigRoot()
	}

	output := strings.TrimSpace(opts.Output)
	if output == "" {
		format := configbundle.DefaultArchiveFormat()
		cwd, err := os.Getwd()
		if err != nil {
			logging.Error("xp2p server export: resolve working directory failed", "err", err)
			return 1
		}
		output = filepath.Join(cwd, configbundle.DefaultArchiveName("server", format, time.Now()))
	} else if _, err := configbundle.DetectArchiveFormat(output); err != nil {
		logging.Error("xp2p server export: unsupported archive format", "err", err)
		return 2
	}

	if err := configbundle.ExportConfigRoot(root, output); err != nil {
		logging.Error("xp2p server export: failed", "err", err)
		return 1
	}
	logging.Info("xp2p server export: archive created", "path", output)
	return 0
}

func runServerImport(_ config.Config, opts serverImportOptions) int {
	root := strings.TrimSpace(opts.ConfigRoot)
	if root == "" {
		root = config.ConfigRoot()
	}
	input := strings.TrimSpace(opts.Input)
	if input == "" {
		logging.Error("xp2p server import: input archive is required")
		return 2
	}

	if err := configbundle.ImportConfigRoot(root, input); err != nil {
		logging.Error("xp2p server import: failed", "err", err)
		return 1
	}
	logging.Info("xp2p server import: archive applied", "path", input)
	logging.Warn("xp2p server import: verify service status after import")
	return 0
}
