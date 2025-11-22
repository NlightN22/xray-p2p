package clientcmd

import (
	"context"
	"flag"
	"os"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func runClientInstall(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p client install", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "client installation directory")
	configDir := fs.String("config-dir", "", "client configuration directory name")
	hostFlag := fs.String("host", "", "remote server host")
	portFlag := fs.String("port", "", "remote server port")
	userEmail := fs.String("user", "", "Trojan user email")
	password := fs.String("password", "", "Trojan password")
	serverName := fs.String("sni", "", "TLS server name (SNI)")
	link := fs.String("link", "", "Trojan client link (trojan://...)")
	allowInsecure := fs.Bool("allow-insecure", false, "allow insecure TLS (skip verification)")
	strictTLS := fs.Bool("strict-tls", false, "enforce TLS verification")
	force := fs.Bool("force", false, "replace existing endpoint configuration")

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
	hostProvided := false
	passwordProvided := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "user":
			userFlagProvided = true
		case "host":
			hostProvided = true
		case "password":
			passwordProvided = true
		}
	})

	if linkValue == "" {
		var missing []string
		if !hostProvided || strings.TrimSpace(*hostFlag) == "" {
			missing = append(missing, "--host")
		}
		if !userFlagProvided || strings.TrimSpace(*userEmail) == "" {
			missing = append(missing, "--user")
		}
		if !passwordProvided || strings.TrimSpace(*password) == "" {
			missing = append(missing, "--password")
		}
		if len(missing) > 0 {
			logging.Error(
				"xp2p client install: missing required arguments when --link is not provided",
				"arguments", strings.Join(missing, ", "),
			)
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

	serverAddressValue = firstNonEmpty(*hostFlag, serverAddressValue)
	serverPortValue = firstNonEmpty(*portFlag, serverPortValue)
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
