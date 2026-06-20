package servercmd

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
)

func newServerProfileCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile [trojan-tls|vless-tls-vision]",
		Short: "Show or switch the server tunnel profile",
		Args:  cobra.RangeArgs(0, 1),
		ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return []string{string(tunnel.ProfileTrojanTLS), string(tunnel.ProfileVLESSTLSVision)}, cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			code := runServerProfile(commandContext(cmd), cfg(), args)
			return errorForCode(code)
		},
	}
	return cmd
}

func runServerProfile(ctx context.Context, cfg config.Config, args []string) int {
	if len(args) == 0 {
		profile := strings.TrimSpace(cfg.Server.Profile)
		if profile == "" {
			profile = string(tunnel.ProfileTrojanTLS)
		}
		logging.Info("xp2p server profile: current profile", "profile", profile)
		return 0
	}
	result, err := serverSetProfileFunc(ctx, server.SetProfileOptions{Profile: args[0]})
	if err != nil {
		logging.Error("xp2p server profile: update failed", "err", err)
		return 1
	}
	logging.Info("xp2p server profile updated", "profile", result.Profile, "apply", result.Apply)
	return 0
}
