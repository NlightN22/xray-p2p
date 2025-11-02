package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

var (
	serverInstallFunc = server.Install
	serverRemoveFunc  = server.Remove
)

func runServer(ctx context.Context, cfg config.Config, args []string) int {
	if len(args) == 0 {
		printServerUsage()
		return 1
	}

	cmd := strings.ToLower(args[0])
	switch cmd {
	case "install":
		return runServerInstall(ctx, cfg, args[1:])
	case "remove":
		return runServerRemove(ctx, cfg, args[1:])
	case "-h", "--help", "help":
		printServerUsage()
		return 0
	default:
		logging.Error("xp2p server: unknown command", "subcommand", args[0])
		printServerUsage()
		return 1
	}
}

func runServerInstall(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p server install", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "server installation directory")
	port := fs.String("port", "", "server listener port")
	mode := fs.String("mode", "", "service start mode (auto|manual)")
	cert := fs.String("cert", "", "TLS certificate file to deploy")
	key := fs.String("key", "", "TLS private key file to deploy")
	force := fs.Bool("force", false, "overwrite existing installation")
	start := fs.Bool("start", true, "start service after successful install")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p server install: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() > 0 {
		logging.Error("xp2p server install: unexpected arguments", "args", fs.Args())
		return 2
	}

	portValue := strings.TrimSpace(*port)
	if portValue == "" {
		cfgPort := strings.TrimSpace(cfg.Server.Port)
		if cfgPort != "" && cfgPort != server.DefaultPort {
			portValue = cfgPort
		}
	}
	if portValue == "" {
		portValue = strconv.Itoa(server.DefaultTrojanPort)
	}

	opts := server.InstallOptions{
		InstallDir:      firstNonEmpty(*path, cfg.Server.InstallDir),
		Port:            portValue,
		Mode:            firstNonEmpty(*mode, cfg.Server.Mode),
		CertificateFile: firstNonEmpty(*cert, cfg.Server.CertificateFile),
		KeyFile:         firstNonEmpty(*key, cfg.Server.KeyFile),
		Force:           *force,
		StartService:    *start,
	}

	if err := serverInstallFunc(ctx, opts); err != nil {
		logging.Error("xp2p server install failed", "err", err)
		return 1
	}

	logging.Info("xp2p server installed", "install_dir", opts.InstallDir)
	return 0
}

func runServerRemove(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p server remove", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "server installation directory")
	keepFiles := fs.Bool("keep-files", false, "keep installation files (service only)")
	ignoreMissing := fs.Bool("ignore-missing", false, "do not fail if service or files are absent")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p server remove: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() > 0 {
		logging.Error("xp2p server remove: unexpected arguments", "args", fs.Args())
		return 2
	}

	opts := server.RemoveOptions{
		InstallDir:    firstNonEmpty(*path, cfg.Server.InstallDir),
		KeepFiles:     *keepFiles,
		IgnoreMissing: *ignoreMissing,
	}

	if err := serverRemoveFunc(ctx, opts); err != nil {
		logging.Error("xp2p server remove failed", "err", err)
		return 1
	}

	logging.Info("xp2p server removed", "install_dir", opts.InstallDir)
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func printServerUsage() {
	fmt.Print(`xp2p server commands:
  install [--path PATH] [--port PORT] [--mode auto|manual] [--cert FILE] [--key FILE] [--force] [--start]
  remove [--path PATH] [--keep-files] [--ignore-missing]
`)
}
