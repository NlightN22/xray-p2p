package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
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
		fmt.Fprintf(os.Stderr, "xp2p server: unknown command %q\n\n", args[0])
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
		fmt.Fprintf(os.Stderr, "xp2p server install: %v\n", err)
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "xp2p server install: unexpected arguments: %v\n", fs.Args())
		return 2
	}

	opts := server.InstallOptions{
		InstallDir:      firstNonEmpty(*path, cfg.Server.InstallDir),
		Port:            firstNonEmpty(*port, cfg.Server.Port),
		Mode:            firstNonEmpty(*mode, cfg.Server.Mode),
		CertificateFile: firstNonEmpty(*cert, cfg.Server.CertificateFile),
		KeyFile:         firstNonEmpty(*key, cfg.Server.KeyFile),
		Force:           *force,
		StartService:    *start,
	}

	if err := serverInstallFunc(ctx, opts); err != nil {
		fmt.Fprintf(os.Stderr, "xp2p server install: %v\n", err)
		return 1
	}

	fmt.Fprintf(os.Stdout, "xp2p server installed at %s\n", opts.InstallDir)
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
		fmt.Fprintf(os.Stderr, "xp2p server remove: %v\n", err)
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "xp2p server remove: unexpected arguments: %v\n", fs.Args())
		return 2
	}

	opts := server.RemoveOptions{
		InstallDir:    firstNonEmpty(*path, cfg.Server.InstallDir),
		KeepFiles:     *keepFiles,
		IgnoreMissing: *ignoreMissing,
	}

	if err := serverRemoveFunc(ctx, opts); err != nil {
		fmt.Fprintf(os.Stderr, "xp2p server remove: %v\n", err)
		return 1
	}

	fmt.Fprintln(os.Stdout, "xp2p server removed")
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
