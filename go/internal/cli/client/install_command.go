package clientcmd

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/preflight"
)

func runClientInstall(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p client install", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "client installation directory")
	configDir := fs.String("config-dir", "", "client configuration directory name")
	modeFlag := fs.String("mode", "", "target client mode (proxy or tun; also supports tun:split or tun:full)")
	hostFlag := fs.String("host", "", "remote server host")
	portFlag := fs.String("port", "", "remote server port")
	userEmail := fs.String("user", "", "user email")
	password := fs.String("password", "", "user password")
	serverName := fs.String("sni", "", "TLS server name (SNI)")
	link := fs.String("link", "", "client connection link")
	profile := fs.String("profile", "", "client tunnel profile")
	profileShort := fs.String("r", "", "client tunnel profile")
	allowInsecure := fs.Bool("allow-insecure", false, "allow insecure TLS (skip verification)")
	strictTLS := fs.Bool("strict-tls", false, "enforce TLS verification")
	force := fs.Bool("force", false, "replace existing endpoint configuration")
	tunMode := fs.String("tun-mode", "", "TUN routing mode: split or full")
	_ = fs.Bool("quiet", false, "do not prompt during JSON execution")

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
	var linkData installLink
	if linkValue != "" {
		var err error
		linkData, err = parseInstallLink(linkValue)
		if err != nil {
			logging.Error("xp2p client install: invalid --link", "err", err)
			return 2
		}
	}

	userFlagProvided := false
	hostProvided := false
	passwordProvided := false
	modeProvided := false
	allowInsecureRequested := false
	strictTLSRequested := false
	tunModeProvided := false
	profileProvided := false
	profileShortProvided := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "user":
			userFlagProvided = true
		case "host":
			hostProvided = true
		case "password":
			passwordProvided = true
		case "mode":
			modeProvided = true
		case "allow-insecure":
			allowInsecureRequested = true
		case "strict-tls":
			strictTLSRequested = true
		case "tun-mode":
			tunModeProvided = true
		case "profile":
			profileProvided = true
		case "r":
			profileShortProvided = true
		}
	})

	profileValue := strings.TrimSpace(*profile)
	profileShortValue := strings.TrimSpace(*profileShort)
	if profileProvided && profileShortProvided && !strings.EqualFold(profileValue, profileShortValue) {
		logging.Error("xp2p client install: --profile conflicts with -r")
		return 2
	}
	if profileShortProvided {
		profileValue = profileShortValue
	}
	if linkValue != "" && (profileProvided || profileShortProvided) && strings.TrimSpace(linkData.Profile) != "" && !strings.EqualFold(profileValue, linkData.Profile) {
		logging.Error("xp2p client install: --profile conflicts with profile from --link")
		return 2
	}

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
	pinnedPeerCertSHA256 := ""
	verifyPeerCertByName := ""
	var alpnValues []string

	if linkValue != "" {
		serverAddressValue = linkData.ServerAddress
		serverPortValue = linkData.ServerPort
		passwordValue = linkData.Password
		userValue = linkData.User
		allowInsecureValue = linkData.AllowInsecure
		pinnedPeerCertSHA256 = linkData.PinnedPeerSHA256
		verifyPeerCertByName = linkData.VerifyPeerName
		alpnValues = linkData.ALPN
		if linkData.ServerNameSet {
			serverNameValue = linkData.ServerName
		}
	}
	if pinnedPeerCertSHA256 != "" {
		allowInsecureValue = false
	}

	serverAddressValue = firstNonEmpty(*hostFlag, serverAddressValue)
	serverPortValue = firstNonEmpty(*portFlag, serverPortValue)
	userValue = firstNonEmpty(*userEmail, userValue)
	passwordValue = firstNonEmpty(*password, passwordValue)
	serverNameValue = firstNonEmpty(*serverName, serverNameValue)

	allowOverride := allowInsecureRequested || strictTLSRequested
	if linkValue != "" && linkData.AllowInsecureSet {
		allowOverride = true
	}

	opts := client.InstallOptions{
		InstallDir:            installDir,
		ConfigDir:             configDirName,
		ServerAddress:         serverAddressValue,
		ServerPort:            serverPortValue,
		User:                  userValue,
		Password:              passwordValue,
		ServerName:            serverNameValue,
		ALPN:                  alpnValues,
		AllowInsecure:         allowInsecureValue,
		PinnedPeerCertSHA256:  pinnedPeerCertSHA256,
		VerifyPeerCertByName:  verifyPeerCertByName,
		Profile:               firstNonEmpty(profileValue, linkData.Profile),
		Protocol:              linkData.Protocol,
		Transport:             linkData.Transport,
		Security:              linkData.Security,
		Flow:                  linkData.Flow,
		HeartbeatMode:         map[bool]string{true: "auto", false: "required"}[linkValue != ""],
		AllowInsecureOverride: allowOverride,
		Force:                 *force,
		TunEnabled:            cfg.Client.TunEnabled,
		TunEnabledSet:         true,
		TunName:               cfg.Client.TunName,
		TunMTU:                cfg.Client.TunMTU,
		TunAddr:               cfg.Client.TunAddr,
		TunMode:               cfg.Client.TunMode,
		TunModeSet:            false,
	}
	if _, err := normalizeProfileSelection(opts.Profile); err != nil {
		logging.Error("xp2p client install: invalid --profile", "err", err)
		return 2
	}

	mode, err := parseTargetClientMode(*modeFlag)
	if err != nil {
		logging.Error("xp2p client install: invalid --mode", "err", err)
		return 2
	}
	if modeProvided && mode.set {
		opts.TunEnabled = mode.tunEnabled
		opts.TunEnabledSet = true
	}
	if mode.set && !mode.tunEnabled && tunModeProvided {
		logging.Error("xp2p client install: --tun-mode is only valid with --mode tun")
		return 2
	}
	if mode.tunModeSet && tunModeProvided {
		logging.Error("xp2p client install: --tun-mode conflicts with --mode tun:...")
		return 2
	}
	if mode.tunModeSet {
		opts.TunMode = mode.tunMode
		opts.TunModeSet = true
	}
	if normalized, set, err := normalizeTunModeFlag("--tun-mode", *tunMode, tunModeProvided); err != nil {
		logging.Error("xp2p client install: invalid --tun-mode", "err", err)
		return 2
	} else if set {
		opts.TunMode = normalized
		opts.TunModeSet = true
	}

	if opts.TunEnabled {
		wintunPath := filepath.Join(installDir, layout.BinDirName, "wintun.dll")
		if err := tunPreflightCheckFunc(ctx, preflight.TunConfig{
			Enabled:       true,
			Name:          opts.TunName,
			Addr:          opts.TunAddr,
			MTU:           opts.TunMTU,
			Mode:          opts.TunMode,
			WintunDLLPath: wintunPath,
		}); err != nil {
			logging.Error("xp2p client install: tun preflight failed", "err", err)
			return 1
		}
	}
	if *allowInsecure {
		opts.AllowInsecure = true
		opts.AllowInsecureOverride = true
	}
	if *strictTLS {
		opts.AllowInsecure = false
		opts.AllowInsecureOverride = true
	}

	if opts.TunModeSet && !*force {
		configPath := config.ConfigPath(layout.ClientConfigFileName)
		if info, err := os.Stat(configPath); err == nil && !info.IsDir() {
			existing, loadErr := config.Load(config.Options{
				Path:         configPath,
				AllowInvalid: true,
			})
			if loadErr != nil {
				logging.Warn("xp2p client install: failed to load existing config for tun mode check", "err", loadErr)
			} else if current := strings.TrimSpace(existing.Client.TunMode); current != "" && !strings.EqualFold(current, opts.TunMode) {
				logging.Error("xp2p client install: tun mode conflict (use --force to override)", "current", current, "requested", opts.TunMode)
				return 1
			}
		} else if err != nil && !os.IsNotExist(err) {
			logging.Error("xp2p client install: unable to stat existing config", "err", err)
			return 1
		}
	}

	if err := clientInstallFunc(ctx, opts); err != nil {
		logging.Error("xp2p client install failed", "err", err)
		return 1
	}

	logging.Info("xp2p client installed", "install_dir", opts.InstallDir, "config_dir", opts.ConfigDir)
	if clioutput.EnabledContext(ctx) {
		if err := clioutput.SetResultContext(ctx, struct {
			Status     string `json:"status"`
			InstallDir string `json:"install_dir"`
			ConfigDir  string `json:"config_dir"`
			Host       string `json:"host"`
			Port       string `json:"port"`
			User       string `json:"user"`
		}{
			Status: "completed", InstallDir: opts.InstallDir, ConfigDir: opts.ConfigDir,
			Host: opts.ServerAddress, Port: opts.ServerPort, User: opts.User,
		}); err != nil {
			logging.Error("xp2p client install: publish JSON result failed", "err", err)
			return 1
		}
	}
	return 0
}
