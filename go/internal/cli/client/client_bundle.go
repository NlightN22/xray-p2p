package clientcmd

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
			path, err := exportClientBundle(cfg(), opts)
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

func newClientImportCmd(cfg commandConfig) *cobra.Command {
	var opts clientImportOptions
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import client configuration bundle",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := importClientBundle(cfg(), opts)
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

func runClientExport(_ config.Config, opts clientExportOptions) int {
	_, err := exportClientBundle(config.Config{}, opts)
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

func exportClientBundle(_ config.Config, opts clientExportOptions) (string, error) {
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
			return "", fmt.Errorf("resolve working directory: %w", err)
		}
		output = filepath.Join(cwd, configbundle.DefaultArchiveName("client", format, time.Now()))
	} else if _, err := configbundle.DetectArchiveFormat(output); err != nil {
		logging.Error("xp2p client export: unsupported archive format", "err", err)
		return "", fmt.Errorf("unsupported archive format: %w", err)
	}

	if err := configbundle.ExportRoleConfigRoot("client", root, output); err != nil {
		logging.Error("xp2p client export: failed", "err", err)
		return "", fmt.Errorf("export client configuration: %w", err)
	}
	logging.Info("xp2p client export: archive created", "path", output)
	return output, nil
}

func runClientImport(_ config.Config, opts clientImportOptions) int {
	_, err := importClientBundle(config.Config{}, opts)
	if err == nil {
		return 0
	}
	var codeErr exitError
	if errors.As(err, &codeErr) {
		return codeErr.code
	}
	return 1
}

func importClientBundle(_ config.Config, opts clientImportOptions) (string, error) {
	root := strings.TrimSpace(opts.ConfigRoot)
	if root == "" {
		root = config.ConfigRoot()
	}
	input := strings.TrimSpace(opts.Input)
	if input == "" {
		logging.Error("xp2p client import: input archive is required")
		return "", exitError{code: 2}
	}

	if err := configbundle.ImportRoleConfigRoot("client", root, input); err != nil {
		logging.Error("xp2p client import: failed", "err", err)
		return "", fmt.Errorf("import client configuration: %w", err)
	}
	logging.Info("xp2p client import: archive applied", "path", input)
	logging.Warn("xp2p client import: verify service status after import")
	return input, nil
}
