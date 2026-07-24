package clientcmd

import (
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

func newClientDebugCmd(_ commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "debug",
		Short: "Debug helpers",
	}
	cmd.AddCommand(newClientDebugBundleCmd())
	return cmd
}

func newClientDebugBundleCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "bundle",
		Short: "Create a debug bundle archive",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := strings.TrimSpace(output)
			if out == "" {
				format := configbundle.DefaultArchiveFormat()
				cwd, err := os.Getwd()
				if err != nil {
					logging.Error("xp2p client debug bundle: resolve working directory failed", "err", err)
					return exitError{code: 1}
				}
				out = filepath.Join(cwd, configbundle.DefaultDebugArchiveName("client", format, time.Now()))
			} else if _, err := configbundle.DetectArchiveFormat(out); err != nil {
				logging.Error("xp2p client debug bundle: unsupported archive format", "err", err)
				return exitError{code: 2}
			}

			if err := configbundle.CreateDebugBundle("client", config.ConfigRoot(), config.LogRoot(), out); err != nil {
				logging.Error("xp2p client debug bundle: failed", "err", err)
				return exitError{code: 1}
			}
			logging.Info("xp2p client debug bundle: archive created", "path", out)
			return clioutput.SetResult(cmd, archiveResult{Path: out})
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&output, "output", "o", "", "archive output path")
	return cmd
}
