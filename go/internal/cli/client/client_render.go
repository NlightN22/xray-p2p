package clientcmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func newClientRenderCmd(_ commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render compiled runtime artifacts",
	}
	cmd.AddCommand(newClientRenderXrayCmd())
	return cmd
}

func newClientRenderXrayCmd() *cobra.Command {
	var live bool
	var desired bool
	var output string
	cmd := &cobra.Command{
		Use:   "xray",
		Short: "Render xray.json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if live == desired {
				logging.Error("xp2p client render xray: use exactly one of --live or --desired")
				return exitError{code: 2}
			}
			out := strings.TrimSpace(output)
			if out == "" {
				out = "-"
			}

			var data []byte
			if desired {
				configPath, err := config.DesiredConfigPathForRole(apply.RoleClient)
				if err != nil {
					logging.Error("xp2p client render xray: resolve desired config failed", "err", err)
					return exitError{code: 1}
				}
				extensionsDir, err := config.DesiredExtensionsDirForRole(apply.RoleClient)
				if err != nil {
					logging.Error("xp2p client render xray: resolve extensions dir failed", "err", err)
					return exitError{code: 1}
				}
				compiled, err := client.CompileDesiredXrayJSON(configPath, extensionsDir)
				if err != nil {
					logging.Error("xp2p client render xray: compile failed", "err", err)
					return exitError{code: 1}
				}
				data = compiled
			} else {
				livePath, err := config.LiveXrayPath(apply.RoleClient)
				if err != nil {
					logging.Error("xp2p client render xray: resolve live artifacts failed", "err", err)
					return exitError{code: 1}
				}
				contents, err := os.ReadFile(livePath)
				if err != nil {
					if errors.Is(err, os.ErrNotExist) {
						logging.Error("xp2p client render xray: live artifacts not found", "path", livePath)
						return exitError{code: 1}
					}
					logging.Error("xp2p client render xray: read live artifacts failed", "err", err)
					return exitError{code: 1}
				}
				data = contents
			}

			if out == "-" {
				_, _ = os.Stdout.Write(data)
				return nil
			}
			if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				logging.Error("xp2p client render xray: create output directory failed", "err", err)
				return exitError{code: 1}
			}
			if err := os.WriteFile(out, data, 0o644); err != nil {
				logging.Error("xp2p client render xray: write output failed", "err", err)
				return exitError{code: 1}
			}
			logging.Info("xp2p client render xray: output written", "path", out)
			return nil
		},
	}
	flags := cmd.Flags()
	flags.BoolVarP(&live, "live", "l", false, "render live runtime artifacts")
	flags.BoolVarP(&desired, "desired", "d", false, "compile Desired inputs without applying")
	flags.StringVarP(&output, "output", "o", "-", "output path ('-' for stdout)")
	return cmd
}
