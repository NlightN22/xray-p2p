package clientcmd

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

type clientExportOptions struct {
	ConfigRoot string
	Output     string
}

type clientImportOptions struct {
	ConfigRoot string
	Input      string
}

func newClientExportCmd(cfg commandConfig) *cobra.Command {
	var opts clientExportOptions
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export client configuration bundle",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runClientExport(cfg(), opts)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&opts.ConfigRoot, "config-root", "C", "", "configuration root to export")
	flags.StringVarP(&opts.Output, "output", "o", "", "archive output path")
	return cmd
}

func newClientImportCmd(cfg commandConfig) *cobra.Command {
	var opts clientImportOptions
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import client configuration bundle",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runClientImport(cfg(), opts)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&opts.ConfigRoot, "config-root", "C", "", "configuration root to import into")
	flags.StringVarP(&opts.Input, "input", "i", "", "archive input path")
	_ = cmd.MarkFlagRequired("input")
	return cmd
}

func runClientExport(_ config.Config, opts clientExportOptions) int {
	root := strings.TrimSpace(opts.ConfigRoot)
	if root == "" {
		root = config.ConfigRoot()
	}

	output := strings.TrimSpace(opts.Output)
	if output == "" {
		format := configbundle.DefaultArchiveFormat()
		cwd, err := os.Getwd()
		if err != nil {
			logging.Error("xp2p client export: resolve working directory failed", "err", err)
			return 1
		}
		output = filepath.Join(cwd, configbundle.DefaultArchiveName("client", format, time.Now()))
	} else if _, err := configbundle.DetectArchiveFormat(output); err != nil {
		logging.Error("xp2p client export: unsupported archive format", "err", err)
		return 2
	}

	if err := configbundle.ExportConfigRoot(root, output); err != nil {
		logging.Error("xp2p client export: failed", "err", err)
		return 1
	}
	logging.Info("xp2p client export: archive created", "path", output)
	return 0
}

func runClientImport(_ config.Config, opts clientImportOptions) int {
	root := strings.TrimSpace(opts.ConfigRoot)
	if root == "" {
		root = config.ConfigRoot()
	}
	input := strings.TrimSpace(opts.Input)
	if input == "" {
		logging.Error("xp2p client import: input archive is required")
		return 2
	}

	if err := configbundle.ImportConfigRoot(root, input); err != nil {
		logging.Error("xp2p client import: failed", "err", err)
		return 1
	}
	logging.Info("xp2p client import: archive applied", "path", input)
	return 0
}
