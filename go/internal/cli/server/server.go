package servercmd

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/deploy/spec"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/netutil"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

var (
	serverInstallFunc    = server.Install
	serverRemoveFunc     = server.Remove
	serverRunFunc        = server.Run
	serverUserAddFunc    = server.AddUser
	serverUserRemoveFunc = server.RemoveUser
	detectPublicHostFunc = netutil.DetectPublicHost
	serverSetCertFunc    = server.SetCertificate
	serverUserLinkFunc   = server.GetUserLink
	serverUserListFunc   = server.ListUsers
)

var promptYesNoFunc = promptYesNo
var serverUsageBlocks = []string{
	`install [--path PATH] [--config-dir NAME] [--port PORT] [--cert FILE] [--key FILE]
          [--host HOST] [--deploy-file FILE] [--force]`,
	`remove  [--path PATH] [--keep-files] [--ignore-missing]`,
	`run     [--path PATH] [--config-dir NAME] [--quiet] [--auto-install]
          [--xray-log-file FILE]`,
	`user    add/remove [...]`,
	`cert    set [--path PATH] [--config-dir NAME|PATH] [--cert FILE] [--key FILE]
          [--host HOST] [--force]`,
}

func Execute(ctx context.Context, cfg config.Config, args []string) int {
	return runServer(ctx, cfg, args)
}

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
	case "run":
		return runServerRun(ctx, cfg, args[1:])
	case "user":
		return runServerUser(ctx, cfg, args[1:])
	case "cert":
		return runServerCert(ctx, cfg, args[1:])
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
	configDir := fs.String("config-dir", "", "server configuration directory name")
	port := fs.String("port", "", "server listener port")
	cert := fs.String("cert", "", "TLS certificate file to deploy")
	key := fs.String("key", "", "TLS private key file to deploy")
	host := fs.String("host", "", "public host name or IP for generated configuration")
	deployFile := fs.String("deploy-file", "", "path to deployment manifest (deployment.json)")
	force := fs.Bool("force", false, "overwrite existing installation")

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

	manifestPath := strings.TrimSpace(*deployFile)
	if manifestPath != "" {
		file, err := os.Open(manifestPath)
		if err != nil {
			logging.Error("xp2p server install: open deploy manifest failed", "path", manifestPath, "err", err)
			return 1
		}
		manifest, err := spec.Read(file)
		file.Close()
		if err != nil {
			logging.Error("xp2p server install: read deploy manifest failed", "path", manifestPath, "err", err)
			return 1
		}
		logging.Info("xp2p server install: using deploy manifest", "remote_host", manifest.RemoteHost, "version", manifest.XP2PVersion)
		if strings.TrimSpace(*host) == "" {
			if err := netutil.ValidateHost(manifest.RemoteHost); err != nil {
				logging.Error("xp2p server install: invalid host in deploy manifest", "host", manifest.RemoteHost, "err", err)
				return 1
			}
			*host = strings.TrimSpace(manifest.RemoteHost)
		}
	}

	portValue := resolveInstallPort(cfg, *port)

	hostValue, autoDetected, err := determineInstallHost(ctx, *host, cfg.Server.Host)
	if err != nil {
		logging.Error("xp2p server install: failed to resolve public host", "err", err)
		return 1
	}
	if autoDetected {
		logging.Info("xp2p server install: detected public host", "host", hostValue)
	}

	opts := server.InstallOptions{
		InstallDir:      firstNonEmpty(*path, cfg.Server.InstallDir),
		ConfigDir:       firstNonEmpty(*configDir, cfg.Server.ConfigDir),
		Port:            portValue,
		CertificateFile: firstNonEmpty(*cert, cfg.Server.CertificateFile),
		KeyFile:         firstNonEmpty(*key, cfg.Server.KeyFile),
		Host:            hostValue,
		Force:           *force,
	}

	if err := serverInstallFunc(ctx, opts); err != nil {
		logging.Error("xp2p server install failed", "err", err)
		return 1
	}

	logging.Info("xp2p server installed", "install_dir", opts.InstallDir, "config_dir", opts.ConfigDir)
	return 0
}

func runServerRemove(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p server remove", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "server installation directory")
	keepFiles := fs.Bool("keep-files", false, "keep installation files")
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

func runServerRun(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p server run", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "server installation directory")
	configDir := fs.String("config-dir", "", "server configuration directory name")
	autoInstall := fs.Bool("auto-install", false, "install server assets when missing without prompting")
	quiet := fs.Bool("quiet", false, "suppress interactive prompts")
	xrayLogFile := fs.String("xray-log-file", "", "append xray stderr output to file")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p server run: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() > 0 {
		logging.Error("xp2p server run: unexpected arguments", "args", fs.Args())
		return 2
	}

	installDir := firstNonEmpty(*path, cfg.Server.InstallDir)
	if installDir == "" {
		logging.Error("xp2p server run: installation directory is required")
		return 1
	}

	configDirName := firstNonEmpty(*configDir, cfg.Server.ConfigDir)
	configDirPath, err := resolveConfigDirPath(installDir, configDirName)
	if err != nil {
		logging.Error("xp2p server run: resolve config directory", "err", err)
		return 1
	}

	if err := ensureServerAssets(ctx, cfg, installDir, configDirName, configDirPath, *autoInstall, *quiet); err != nil {
		logging.Error("xp2p server run: prerequisites failed", "err", err)
		return 1
	}

	cancelDiagnostics := startDiagnostics(ctx, cfg.Server.Port)
	if cancelDiagnostics != nil {
		defer cancelDiagnostics()
	}

	if err := serverRunFunc(ctx, server.RunOptions{
		InstallDir:   installDir,
		ConfigDir:    configDirName,
		ErrorLogPath: strings.TrimSpace(*xrayLogFile),
	}); err != nil {
		logging.Error("xp2p server run failed", "err", err)
		return 1
	}

	return 0
}

func ensureServerAssets(ctx context.Context, cfg config.Config, installDir, configDirName, configDirPath string, autoInstall, quiet bool) error {
	present, err := serverAssetsPresent(installDir, configDirPath)
	if err != nil {
		return err
	}
	if present {
		return nil
	}

	if autoInstall {
		return performInstall(ctx, cfg, installDir, configDirName)
	}

	if quiet {
		return errors.New("installation not found and --quiet supplied (use --auto-install)")
	}

	ok, err := promptYesNoFunc(fmt.Sprintf("Install xray-core into %s?", installDir))
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("installation required to run server")
	}

	return performInstall(ctx, cfg, installDir, configDirName)
}

func performInstall(ctx context.Context, cfg config.Config, installDir, configDirName string) error {
	hostValue, autoDetected, err := determineInstallHost(ctx, "", cfg.Server.Host)
	if err != nil {
		return fmt.Errorf("xp2p server install: detect host: %w", err)
	}
	if autoDetected {
		logging.Info("xp2p server install: detected public host", "host", hostValue)
	}

	opts := server.InstallOptions{
		InstallDir: installDir,
		ConfigDir:  configDirName,
		Port:       resolveInstallPort(cfg, ""),
		Host:       hostValue,
	}
	if cfg.Server.CertificateFile != "" {
		opts.CertificateFile = cfg.Server.CertificateFile
	}
	if cfg.Server.KeyFile != "" {
		opts.KeyFile = cfg.Server.KeyFile
	}
	return serverInstallFunc(ctx, opts)
}

func resolveInstallPort(cfg config.Config, flagPort string) string {
	portValue := strings.TrimSpace(flagPort)
	if portValue != "" {
		return portValue
	}

	cfgPort := strings.TrimSpace(cfg.Server.Port)
	if cfgPort != "" && cfgPort != server.DefaultPort {
		return cfgPort
	}

	return strconv.Itoa(server.DefaultTrojanPort)
}

func serverAssetsPresent(installDir, configDirPath string) (bool, error) {
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

func promptYesNo(question string) (bool, error) {
	fmt.Printf("%s [Y/n]: ", question)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	answer := strings.TrimSpace(strings.ToLower(line))
	if answer == "" || answer == "y" || answer == "yes" {
		return true, nil
	}
	if answer == "n" || answer == "no" {
		return false, nil
	}
	fmt.Println("Please answer 'y' or 'n'.")
	return promptYesNo(question)
}

func resolveConfigDirPath(installDir, configDir string) (string, error) {
	cfgDir := strings.TrimSpace(configDir)
	if cfgDir == "" {
		cfgDir = server.DefaultServerConfigDir
	}
	if filepath.IsAbs(cfgDir) {
		return cfgDir, nil
	}
	return filepath.Join(installDir, cfgDir), nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func determineInstallHost(ctx context.Context, explicit, fallback string) (string, bool, error) {
	host := firstNonEmpty(explicit, fallback)
	host = strings.TrimSpace(host)
	if host != "" {
		if err := netutil.ValidateHost(host); err != nil {
			return "", false, err
		}
		return host, false, nil
	}
	value, err := detectPublicHostFunc(ctx)
	if err != nil {
		return "", false, err
	}
	value = strings.TrimSpace(value)
	if err := netutil.ValidateHost(value); err != nil {
		return "", false, fmt.Errorf("invalid host %q: %w", value, err)
	}
	return value, true, nil
}

func printServerUsage() {
	fmt.Print(Usage())
}

// Usage returns detailed help text for xp2p server commands.
func Usage() string {
	var b strings.Builder
	b.WriteString("xp2p server commands:\n")
	for _, block := range serverUsageBlocks {
		lines := strings.Split(block, "\n")
		for _, line := range lines {
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	return b.String()
}

// RootUsage returns the subset of usage lines suitable for the root help output.
func RootUsage() string {
	var b strings.Builder
	prefix := "  xp2p server "
	continuation := strings.Repeat(" ", len(prefix))
	for _, block := range serverUsageBlocks {
		lines := strings.Split(block, "\n")
		b.WriteString(prefix)
		b.WriteString(lines[0])
		b.WriteString("\n")
		for _, line := range lines[1:] {
			b.WriteString(continuation)
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	return b.String()
}
