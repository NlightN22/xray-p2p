package servercmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
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
			path, err := exportServerBundle(cfg(), opts)
			if err != nil {
				return err
			}
			return clioutput.SetResult(cmd, archiveResult{Path: path})
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
			path, err := importServerBundle(cfg(), opts)
			if err != nil {
				return err
			}
			return clioutput.SetResult(cmd, struct {
				Status string `json:"status"`
				Path   string `json:"path"`
			}{Status: "completed", Path: path})
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&opts.ConfigRoot, "config-root", "C", "", "configuration root to import into")
	flags.StringVarP(&opts.Input, "input", "i", "", "archive input path")
	_ = cmd.MarkFlagRequired("input")
	return cmd
}

func runServerExport(_ config.Config, opts serverExportOptions) int {
	_, err := exportServerBundle(config.Config{}, opts)
	if err == nil {
		return 0
	}
	var codeErr exitError
	if errors.As(err, &codeErr) {
		return codeErr.code
	}
	return 1
}

type archiveResult struct {
	Path string `json:"path"`
}

func exportServerBundle(_ config.Config, opts serverExportOptions) (string, error) {
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
			return "", fmt.Errorf("resolve working directory: %w", err)
		}
		output = filepath.Join(cwd, configbundle.DefaultArchiveName("server", format, time.Now()))
	} else if _, err := configbundle.DetectArchiveFormat(output); err != nil {
		logging.Error("xp2p server export: unsupported archive format", "err", err)
		return "", fmt.Errorf("unsupported archive format: %w", err)
	}

	if err := configbundle.ExportRoleConfigRoot("server", root, output); err != nil {
		logging.Error("xp2p server export: failed", "err", err)
		return "", fmt.Errorf("export server configuration: %w", err)
	}
	logging.Info("xp2p server export: archive created", "path", output)
	return output, nil
}

func runServerImport(_ config.Config, opts serverImportOptions) int {
	_, err := importServerBundle(config.Config{}, opts)
	if err == nil {
		return 0
	}
	var codeErr exitError
	if errors.As(err, &codeErr) {
		return codeErr.code
	}
	return 1
}

func importServerBundle(_ config.Config, opts serverImportOptions) (string, error) {
	root := strings.TrimSpace(opts.ConfigRoot)
	if root == "" {
		root = config.ConfigRoot()
	}
	input := strings.TrimSpace(opts.Input)
	if input == "" {
		logging.Error("xp2p server import: input archive is required")
		return "", exitError{code: 2}
	}

	if err := configbundle.ImportRoleConfigRoot("server", root, input); err != nil {
		logging.Error("xp2p server import: failed", "err", err)
		return "", fmt.Errorf("import server configuration: %w", err)
	}
	logging.Info("xp2p server import: archive applied", "path", input)
	logging.Warn("xp2p server import: verify service status after import")
	return input, nil
}
