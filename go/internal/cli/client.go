package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

var (
	clientInstallFunc = client.Install
	clientRemoveFunc  = client.Remove
	clientRunFunc     = client.Run
)

func runClient(ctx context.Context, cfg config.Config, args []string) int {
	if len(args) == 0 {
		printClientUsage()
		return 1
	}

	cmd := strings.ToLower(args[0])
	switch cmd {
	case "install":
		return runClientInstall(ctx, cfg, args[1:])
	case "remove":
		return runClientRemove(ctx, cfg, args[1:])
	case "run":
		return runClientRun(ctx, cfg, args[1:])
	case "-h", "--help", "help":
		printClientUsage()
		return 0
	default:
		logging.Error("xp2p client: unknown command", "subcommand", args[0])
		printClientUsage()
		return 1
	}
}

func runClientInstall(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p client install", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "client installation directory")
	configDir := fs.String("config-dir", "", "client configuration directory name")
	serverAddress := fs.String("server-address", "", "remote server address")
	serverPort := fs.String("server-port", "", "remote server port")
	password := fs.String("password", "", "Trojan password")
	serverName := fs.String("server-name", "", "TLS server name")
	allowInsecure := fs.Bool("allow-insecure", false, "allow insecure TLS (skip verification)")
	strictTLS := fs.Bool("strict-tls", false, "enforce TLS verification")
	force := fs.Bool("force", false, "overwrite existing installation")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p client install: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() > 0 {
		logging.Error("xp2p client install: unexpected arguments", "args", fs.Args())
		return 2
	}

	opts := client.InstallOptions{
		InstallDir:    firstNonEmpty(*path, cfg.Client.InstallDir),
		ConfigDir:     firstNonEmpty(*configDir, cfg.Client.ConfigDir),
		ServerAddress: firstNonEmpty(*serverAddress, cfg.Client.ServerAddress),
		ServerPort:    firstNonEmpty(*serverPort, cfg.Client.ServerPort),
		Password:      firstNonEmpty(*password, cfg.Client.Password),
		ServerName:    firstNonEmpty(*serverName, cfg.Client.ServerName),
		AllowInsecure: cfg.Client.AllowInsecure,
		Force:         *force,
	}
	if *allowInsecure {
		opts.AllowInsecure = true
	}
	if *strictTLS {
		opts.AllowInsecure = false
	}

	if err := clientInstallFunc(ctx, opts); err != nil {
		logging.Error("xp2p client install failed", "err", err)
		return 1
	}

	logging.Info("xp2p client installed", "install_dir", opts.InstallDir, "config_dir", opts.ConfigDir)
	return 0
}

func runClientRemove(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p client remove", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "client installation directory")
	keepFiles := fs.Bool("keep-files", false, "keep installation files")
	ignoreMissing := fs.Bool("ignore-missing", false, "do not fail if installation is absent")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p client remove: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() > 0 {
		logging.Error("xp2p client remove: unexpected arguments", "args", fs.Args())
		return 2
	}

	opts := client.RemoveOptions{
		InstallDir:    firstNonEmpty(*path, cfg.Client.InstallDir),
		KeepFiles:     *keepFiles,
		IgnoreMissing: *ignoreMissing,
	}

	if err := clientRemoveFunc(ctx, opts); err != nil {
		logging.Error("xp2p client remove failed", "err", err)
		return 1
	}

	logging.Info("xp2p client removed", "install_dir", opts.InstallDir)
	return 0
}

func runClientRun(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p client run", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "client installation directory")
	configDir := fs.String("config-dir", "", "client configuration directory name")
	quiet := fs.Bool("quiet", false, "do not prompt for installation")
	autoInstall := fs.Bool("auto-install", false, "install automatically if missing")
	logFile := fs.String("xray-log-file", "", "file to append xray-core stderr output")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p client run: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() > 0 {
		logging.Error("xp2p client run: unexpected arguments", "args", fs.Args())
		return 2
	}

	installDir := firstNonEmpty(*path, cfg.Client.InstallDir)
	configDirName := firstNonEmpty(*configDir, cfg.Client.ConfigDir)

	configDirPath, err := resolveClientConfigDirPath(installDir, configDirName)
	if err != nil {
		logging.Error("xp2p client run: resolve config dir failed", "err", err)
		return 1
	}

	installed, err := clientAssetsPresent(installDir, configDirPath)
	if err != nil {
		logging.Error("xp2p client run: installation check failed", "err", err)
		return 1
	}

	if !installed {
		if *autoInstall {
			logging.Info("xp2p client run: installing missing assets", "install_dir", installDir)
			if err := performClientInstall(ctx, cfg, installDir, configDirName); err != nil {
				logging.Error("xp2p client run: auto-install failed", "err", err)
				return 1
			}
		} else {
			if *quiet {
				logging.Error("xp2p client run: installation missing and --quiet specified (use --auto-install)")
				return 1
			}
			ok, promptErr := promptYesNoFunc(fmt.Sprintf("Install client into %s?", installDir))
			if promptErr != nil {
				logging.Error("xp2p client run: prompt failed", "err", promptErr)
				return 1
			}
			if !ok {
				logging.Error("xp2p client run: installation required to proceed")
				return 1
			}
			if err := performClientInstall(ctx, cfg, installDir, configDirName); err != nil {
				logging.Error("xp2p client run: manual install failed", "err", err)
				return 1
			}
		}
	}

	opts := client.RunOptions{
		InstallDir:   installDir,
		ConfigDir:    configDirName,
		ErrorLogPath: strings.TrimSpace(*logFile),
	}

	if err := clientRunFunc(ctx, opts); err != nil {
		logging.Error("xp2p client run failed", "err", err)
		return 1
	}

	return 0
}

func performClientInstall(ctx context.Context, cfg config.Config, installDir, configDirName string) error {
	opts := client.InstallOptions{
		InstallDir:    installDir,
		ConfigDir:     configDirName,
		ServerAddress: cfg.Client.ServerAddress,
		ServerPort:    cfg.Client.ServerPort,
		Password:      cfg.Client.Password,
		ServerName:    cfg.Client.ServerName,
		AllowInsecure: cfg.Client.AllowInsecure,
	}
	return clientInstallFunc(ctx, opts)
}

func clientAssetsPresent(installDir, configDirPath string) (bool, error) {
	binPath := filepath.Join(installDir, "bin", "xray.exe")
	if info, err := os.Stat(binPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("xp2p: stat %s: %w", binPath, err)
	} else if info.IsDir() {
		return false, fmt.Errorf("xp2p: expected file at %s", binPath)
	}

	configInfo, err := os.Stat(configDirPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("xp2p: stat %s: %w", configDirPath, err)
	}
	if !configInfo.IsDir() {
		return false, fmt.Errorf("xp2p: %s is not a directory", configDirPath)
	}

	requiredFiles := []string{"inbounds.json", "logs.json", "outbounds.json", "routing.json"}
	for _, name := range requiredFiles {
		path := filepath.Join(configDirPath, name)
		if _, err := os.Stat(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return false, nil
			}
			return false, fmt.Errorf("xp2p: stat %s: %w", path, err)
		}
	}
	return true, nil
}

func resolveClientConfigDirPath(installDir, configDir string) (string, error) {
	cfgDir := strings.TrimSpace(configDir)
	if cfgDir == "" {
		cfgDir = client.DefaultClientConfigDir
	}
	if filepath.IsAbs(cfgDir) {
		return cfgDir, nil
	}
	return filepath.Join(installDir, cfgDir), nil
}

func printClientUsage() {
	fmt.Print(`xp2p client commands:
  install [--path PATH] [--config-dir NAME] --server-address HOST --password SECRET
          [--server-port PORT] [--server-name NAME]
          [--allow-insecure|--strict-tls] [--force]
  remove  [--path PATH] [--keep-files] [--ignore-missing]
  run     [--path PATH] [--config-dir NAME] [--quiet] [--auto-install]
          [--xray-log-file FILE]
          (requires client server address and password configured)
`)
}
