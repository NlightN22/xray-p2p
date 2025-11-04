package clientcmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
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

func Execute(ctx context.Context, cfg config.Config, args []string) int {
	return runClient(ctx, cfg, args)
}

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
	case "deploy":
		return runClientDeploy(ctx, cfg, args[1:])
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
	userEmail := fs.String("user", "", "Trojan user email")
	password := fs.String("password", "", "Trojan password")
	serverName := fs.String("server-name", "", "TLS server name")
	link := fs.String("link", "", "Trojan client link (trojan://...)")
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

	linkValue := strings.TrimSpace(*link)
	var linkData trojanLink
	if linkValue != "" {
		var err error
		linkData, err = parseTrojanLink(linkValue)
		if err != nil {
			logging.Error("xp2p client install: invalid --link", "err", err)
			return 2
		}
	}

	userFlagProvided := false
	serverAddressProvided := false
	passwordProvided := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "user":
			userFlagProvided = true
		case "server-address":
			serverAddressProvided = true
		case "password":
			passwordProvided = true
		}
	})

	if linkValue == "" {
		if !serverAddressProvided || strings.TrimSpace(*serverAddress) == "" {
			logging.Error("xp2p client install: --server-address is required when --link is not provided")
			return 2
		}
		if !userFlagProvided || strings.TrimSpace(*userEmail) == "" {
			logging.Error("xp2p client install: --user is required when --link is not provided")
			return 2
		}
		if !passwordProvided || strings.TrimSpace(*password) == "" {
			logging.Error("xp2p client install: --password is required when --link is not provided")
			return 2
		}
	}

	installDir := firstNonEmpty(*path, cfg.Client.InstallDir)
	configDirName := firstNonEmpty(*configDir, cfg.Client.ConfigDir)

	serverAddressValue := cfg.Client.ServerAddress
	serverPortValue := cfg.Client.ServerPort
	userValue := cfg.Client.User
	passwordValue := cfg.Client.Password
	serverNameValue := cfg.Client.ServerName
	allowInsecureValue := cfg.Client.AllowInsecure

	if linkValue != "" {
		serverAddressValue = linkData.ServerAddress
		serverPortValue = linkData.ServerPort
		passwordValue = linkData.Password
		userValue = linkData.User
		allowInsecureValue = linkData.AllowInsecure
		if linkData.ServerNameSet {
			serverNameValue = linkData.ServerName
		}
	}

	serverAddressValue = firstNonEmpty(*serverAddress, serverAddressValue)
	serverPortValue = firstNonEmpty(*serverPort, serverPortValue)
	userValue = firstNonEmpty(*userEmail, userValue)
	passwordValue = firstNonEmpty(*password, passwordValue)
	serverNameValue = firstNonEmpty(*serverName, serverNameValue)

	opts := client.InstallOptions{
		InstallDir:    installDir,
		ConfigDir:     configDirName,
		ServerAddress: serverAddressValue,
		ServerPort:    serverPortValue,
		User:          userValue,
		Password:      passwordValue,
		ServerName:    serverNameValue,
		AllowInsecure: allowInsecureValue,
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

	cancelDiagnostics := startDiagnostics(ctx, cfg.Server.Port)
	if cancelDiagnostics != nil {
		defer cancelDiagnostics()
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
		User:          cfg.Client.User,
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
  install [--path PATH] [--config-dir NAME]
          (--link URL | --server-address HOST --user EMAIL --password SECRET)
          [--server-port PORT] [--server-name NAME]
          [--allow-insecure|--strict-tls] [--force]
  deploy  --remote-host HOST [--ssh-user NAME] [--ssh-port PORT]
          [--server-host HOST] [--server-port PORT]
          [--user EMAIL] [--password SECRET] [--install-dir PATH]
          [--config-dir NAME] [--local-install PATH] [--local-config NAME]
          [--save-link FILE]
  remove  [--path PATH] [--keep-files] [--ignore-missing]
  run     [--path PATH] [--config-dir NAME] [--quiet] [--auto-install]
          [--xray-log-file FILE]
          (requires client server address and password configured)
`)
}

type trojanLink struct {
	ServerAddress string
	ServerPort    string
	User          string
	Password      string
	ServerName    string
	ServerNameSet bool
	AllowInsecure bool
}

func parseTrojanLink(raw string) (trojanLink, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return trojanLink{}, fmt.Errorf("trojan link is empty")
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return trojanLink{}, fmt.Errorf("parse trojan link: %w", err)
	}
	if !strings.EqualFold(parsed.Scheme, "trojan") {
		return trojanLink{}, fmt.Errorf("unsupported scheme %q (expected trojan)", parsed.Scheme)
	}

	address := parsed.Hostname()
	if address == "" {
		return trojanLink{}, fmt.Errorf("missing host in trojan link")
	}

	portValue := parsed.Port()
	if portValue == "" {
		return trojanLink{}, fmt.Errorf("missing port in trojan link")
	}
	if _, err := strconv.Atoi(portValue); err != nil {
		return trojanLink{}, fmt.Errorf("invalid port %q in trojan link", portValue)
	}

	if parsed.User == nil {
		return trojanLink{}, fmt.Errorf("missing password in trojan link")
	}
	password := ""
	if pwd, ok := parsed.User.Password(); ok {
		password = strings.TrimSpace(pwd)
	} else {
		password = strings.TrimSpace(parsed.User.Username())
	}
	if password == "" {
		return trojanLink{}, fmt.Errorf("empty password in trojan link")
	}

	user, err := decodeTrojanUser(parsed)
	if err != nil {
		return trojanLink{}, err
	}

	query := parsed.Query()
	allowInsecure := false
	if rawAllow := strings.TrimSpace(query.Get("allowInsecure")); rawAllow != "" {
		val, convErr := parseBoolFlag(rawAllow)
		if convErr != nil {
			return trojanLink{}, fmt.Errorf("invalid allowInsecure value %q", rawAllow)
		}
		allowInsecure = val
	}

	security := strings.ToLower(strings.TrimSpace(query.Get("security")))
	serverName := ""
	serverNameSet := false
	switch security {
	case "none":
		serverName = ""
		serverNameSet = true
		allowInsecure = false
	default:
		serverName = strings.TrimSpace(query.Get("sni"))
		if serverName == "" {
			serverName = address
		}
		serverNameSet = true
	}

	return trojanLink{
		ServerAddress: address,
		ServerPort:    portValue,
		User:          user,
		Password:      password,
		ServerName:    serverName,
		ServerNameSet: serverNameSet,
		AllowInsecure: allowInsecure,
	}, nil
}

func parseBoolFlag(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value %q", value)
	}
}

func decodeTrojanUser(u *url.URL) (string, error) {
	fragment := strings.TrimSpace(u.Fragment)
	if fragment != "" {
		decoded, err := url.PathUnescape(fragment)
		if err != nil {
			return "", fmt.Errorf("decode trojan link user: %w", err)
		}
		decoded = strings.TrimSpace(decoded)
		if decoded != "" {
			return decoded, nil
		}
	}

	candidates := []string{
		"email",
		"user",
		"username",
		"name",
		"remark",
		"remarks",
		"peer",
	}
	query := u.Query()
	for _, key := range candidates {
		if val := strings.TrimSpace(query.Get(key)); val != "" {
			return val, nil
		}
	}

	if strings.Contains(u.RawQuery, "&") && !strings.Contains(u.RawPath, "#") && !strings.Contains(u.Fragment, "#") {
		return "", fmt.Errorf("trojan link missing user/email (wrap the URL in quotes or escape '&' on Windows)")
	}
	return "", fmt.Errorf("trojan link missing user/email (expected #email or email query parameter)")
}
