package clientcmd

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	clishared "github.com/NlightN22/xray-p2p/go/internal/cli/common"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/netutil"
)

func parseDeployFlags(cfg config.Config, args []string) (deployOptions, error) {
	fs := flag.NewFlagSet("xp2p client deploy", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	hostFlag := fs.String("host", "", "deploy host name or address")
	deployPort := fs.String("port", "62025", "deploy port")
	installDir := fs.String("install-dir", "", "server install directory override")
	trojanUser := fs.String("user", "", "Trojan user identifier (email)")
	trojanPassword := fs.String("password", "", "Trojan user password (auto-generated when omitted)")
	trojanPort := fs.String("trojan-port", "", "Trojan service port")
	tunMode := fs.String("tun-mode", "", "TUN routing mode (split or full)")
	force := fs.Bool("force", false, "allow changing existing tun mode")

	if err := fs.Parse(args); err != nil {
		return deployOptions{}, err
	}
	if fs.NArg() > 0 {
		return deployOptions{}, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	host := strings.TrimSpace(*hostFlag)
	if host == "" || strings.HasPrefix(host, "-") {
		return deployOptions{}, fmt.Errorf("--host is required")
	}
	if err := netutil.ValidateHost(host); err != nil {
		return deployOptions{}, fmt.Errorf("--host: %v", err)
	}

	serverHostValue := firstNonEmpty(cfg.Server.Host, host)
	serverPortValue := normalizeServerPort(cfg, *trojanPort)

	userValue := strings.TrimSpace(firstNonEmpty(*trojanUser, cfg.Client.User))
	if userValue == "" {
		userValue = fmt.Sprintf("deploy-%d@local", time.Now().Unix())
	}

	passwordValue := strings.TrimSpace(*trojanPassword)
	if passwordValue == "" {
		passwordValue = strings.TrimSpace(cfg.Client.Password)
	}
	if passwordValue == "" && userValue != "" {
		gen, err := generateSecret(18)
		if err != nil {
			return deployOptions{}, fmt.Errorf("generate password: %w", err)
		}
		passwordValue = gen
	}
	if err := clishared.ValidateRFC3986Unreserved(passwordValue); err != nil {
		return deployOptions{}, fmt.Errorf("invalid password: %w", err)
	}

	installDirSet := false
	tunModeSet := false
	fs.Visit(func(flag *flag.Flag) {
		if flag.Name == "install-dir" {
			installDirSet = true
		}
		if flag.Name == "tun-mode" {
			tunModeSet = true
		}
	})

	tunModeValue := strings.TrimSpace(*tunMode)
	if tunModeSet {
		switch strings.ToLower(tunModeValue) {
		case "split", "full":
			tunModeValue = strings.ToLower(tunModeValue)
		default:
			return deployOptions{}, fmt.Errorf("--tun-mode must be split or full")
		}
	}

	return deployOptions{
		manifest: manifestOptions{
			remoteHost:     host,
			installDir:     strings.TrimSpace(*installDir),
			installDirSet:  installDirSet,
			trojanPort:     serverPortValue,
			trojanUser:     strings.TrimSpace(userValue),
			trojanPassword: strings.TrimSpace(passwordValue),
			tunMode:        tunModeValue,
			tunModeSet:     tunModeSet,
			force:          *force,
		},
		runtime: runtimeOptions{
			remoteHost: host,
			deployPort: strings.TrimSpace(*deployPort),
			serverHost: serverHostValue,
		},
	}, nil
}
