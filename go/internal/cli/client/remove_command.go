package clientcmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func runClientRemove(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p client remove", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "client installation directory")
	configDir := fs.String("config-dir", "", "client configuration directory name")
	keepFiles := fs.Bool("keep-files", false, "keep installation files")
	ignoreMissing := fs.Bool("ignore-missing", false, "do not fail if installation is absent")
	removeAll := fs.Bool("all", false, "remove all endpoints and configuration")
	quiet := fs.Bool("quiet", false, "do not prompt for removal")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p client remove: failed to parse arguments", "err", err)
		return 2
	}

	targetArgs := fs.Args()
	if *removeAll {
		if len(targetArgs) > 0 {
			logging.Error("xp2p client remove: --all does not accept positional arguments", "args", targetArgs)
			return 2
		}
	} else {
		if len(targetArgs) == 0 {
			logging.Error("xp2p client remove: specify <hostname|tag> or --all")
			return 2
		}
		if len(targetArgs) > 1 {
			logging.Error("xp2p client remove: too many arguments", "args", targetArgs)
			return 2
		}
		if *keepFiles || *ignoreMissing {
			logging.Error("xp2p client remove: --keep-files and --ignore-missing can only be used with --all")
			return 2
		}
	}

	installDir := firstNonEmpty(*path, cfg.Client.InstallDir)
	configDirName := firstNonEmpty(*configDir, cfg.Client.ConfigDir)

	if *removeAll {
		if !*quiet {
			ok, promptErr := promptYesNoFunc(fmt.Sprintf("Remove all client configuration from %s (%s)?", installDir, configDirName))
			if promptErr != nil {
				logging.Error("xp2p client remove: prompt failed", "err", promptErr)
				return 1
			}
			if !ok {
				logging.Info("xp2p client remove aborted by user")
				return 1
			}
		}

		opts := client.RemoveOptions{
			InstallDir:    installDir,
			ConfigDir:     configDirName,
			KeepFiles:     *keepFiles,
			IgnoreMissing: *ignoreMissing,
		}

		if err := clientRemoveFunc(ctx, opts); err != nil {
			logging.Error("xp2p client remove failed", "err", err)
			return 1
		}

		logging.Info("xp2p client removed", "install_dir", opts.InstallDir, "config_dir", opts.ConfigDir)
		return 0
	}

	target := strings.TrimSpace(targetArgs[0])
	if target == "" {
		logging.Error("xp2p client remove: specify <hostname|tag> or --all")
		return 2
	}

	if !*quiet {
		ok, promptErr := promptYesNoFunc(fmt.Sprintf("Remove client endpoint %s from %s?", target, installDir))
		if promptErr != nil {
			logging.Error("xp2p client remove: prompt failed", "err", promptErr)
			return 1
		}
		if !ok {
			logging.Info("xp2p client remove aborted by user")
			return 1
		}
	}

	endpointOpts := client.RemoveEndpointOptions{
		InstallDir: installDir,
		ConfigDir:  configDirName,
		Target:     target,
	}
	if err := clientRemoveEndpointFunc(ctx, endpointOpts); err != nil {
		logging.Error("xp2p client remove failed", "err", err)
		return 1
	}

	logging.Info("xp2p client endpoint removed", "target", target, "config_dir", endpointOpts.ConfigDir)
	return 0
}
