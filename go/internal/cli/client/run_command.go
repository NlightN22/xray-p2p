package clientcmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func runClientRun(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p client run", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "client installation directory")
	configDir := fs.String("config-dir", "", "client configuration directory name")
	quiet := fs.Bool("quiet", false, "do not prompt for installation")
	autoInstall := fs.Bool("auto-install", false, "install automatically if missing")
	logFile := fs.String("xray-log-file", "", "file to append xray-core stderr output")
	hbEnabled := fs.Bool("heartbeat", true, "enable background heartbeat probes")
	hbInterval := fs.Duration("heartbeat-interval", 2*time.Second, "frequency of heartbeat probes")
	hbTimeout := fs.Duration("heartbeat-timeout", 2*time.Second, "timeout per heartbeat probe")
	hbPort := fs.String("heartbeat-port", cfg.Server.Port, "diagnostics service port to probe")
	hbSocks := fs.String("heartbeat-socks", cfg.Client.SocksAddress, "SOCKS5 proxy for heartbeat (optional)")

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

	cancelDiagnostics := startDiagnostics(ctx, cfg.Server.Port)
	if cancelDiagnostics != nil {
		defer cancelDiagnostics()
	}

	opts := client.RunOptions{
		InstallDir:   installDir,
		ConfigDir:    configDirName,
		ErrorLogPath: strings.TrimSpace(*logFile),
		Heartbeat: client.HeartbeatOptions{
			Enabled:      *hbEnabled,
			Interval:     *hbInterval,
			Timeout:      *hbTimeout,
			Port:         firstNonEmpty(strings.TrimSpace(*hbPort), cfg.Server.Port),
			SocksAddress: firstNonEmpty(strings.TrimSpace(*hbSocks), cfg.Client.SocksAddress),
		},
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
		User:          cfg.Client.User,
		Password:      cfg.Client.Password,
		ServerName:    cfg.Client.ServerName,
		AllowInsecure: cfg.Client.AllowInsecure,
	}
	return clientInstallFunc(ctx, opts)
}

func clientAssetsPresent(installDir, configDirPath string) (bool, error) {
	binaryName := "xray.exe"
	if runtime.GOOS != "windows" {
		binaryName = "xray"
	}
	binPath := filepath.Join(installDir, layout.BinDirName, binaryName)
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
