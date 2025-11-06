package clientcmd

import (
    "context"
    "errors"
    "flag"
    "fmt"
    "io"
    "strings"

    "github.com/NlightN22/xray-p2p/go/internal/config"
    "github.com/NlightN22/xray-p2p/go/internal/logging"
    "github.com/NlightN22/xray-p2p/go/internal/netutil"
)

type manifestOptions struct {
    remoteHost     string
    installDir     string
    trojanPort     string
    trojanUser     string
    trojanPassword string
}

type runtimeOptions struct {
    remoteHost string
    deployPort string
    serverHost string
}

type deployOptions struct {
    manifest manifestOptions
    runtime  runtimeOptions
}

func runClientDeploy(ctx context.Context, cfg config.Config, args []string) int {
    opts, err := parseDeployFlags(cfg, args)
    if err != nil {
        if errors.Is(err, flag.ErrHelp) {
            return 0
        }
        logging.Error("xp2p client deploy: argument parsing failed", "err", err)
        return 2
    }

    // Build and print deploy link, then TODO: handshake with server deploy
    link := buildDeployLink(opts)
    logging.Info("xp2p client deploy: link generated", "link", link)
    logging.Info("xp2p client deploy: waiting for server deploy to accept this link (TODO)")
    // TODO: Implement deploy link handshake protocol
    return 0
}

func parseDeployFlags(cfg config.Config, args []string) (deployOptions, error) {
    fs := flag.NewFlagSet("xp2p client deploy", flag.ContinueOnError)
    fs.SetOutput(io.Discard)

    remoteHost := fs.String("remote-host", "", "deploy host name or address")
    deployPort := fs.String("deploy-port", "62025", "deploy port (default 62025)")
    trojanUser := fs.String("user", "", "Trojan user identifier (email)")
    trojanPassword := fs.String("password", "", "Trojan user password (auto-generated when omitted)")
    trojanPort := fs.String("trojan-port", "", "Trojan service port")

    if err := fs.Parse(args); err != nil {
        return deployOptions{}, err
    }
    if fs.NArg() > 0 {
        return deployOptions{}, fmt.Errorf("unexpected arguments: %v", fs.Args())
    }

    host := strings.TrimSpace(*remoteHost)
    if host == "" || strings.HasPrefix(host, "-") {
        return deployOptions{}, fmt.Errorf("--remote-host is required")
    }
    if err := netutil.ValidateHost(host); err != nil {
        return deployOptions{}, fmt.Errorf("--remote-host: %v", err)
    }

    serverHostValue := firstNonEmpty(cfg.Server.Host, host)
    serverPortValue := normalizeServerPort(cfg, *trojanPort)

    userValue := strings.TrimSpace(firstNonEmpty(*trojanUser, cfg.Client.User))
    // optional: user/password can be empty; server may generate

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

    return deployOptions{
        manifest: manifestOptions{
            remoteHost:     host,
            installDir:     strings.TrimSpace(cfg.Server.InstallDir),
            trojanPort:     serverPortValue,
            trojanUser:     strings.TrimSpace(userValue),
            trojanPassword: strings.TrimSpace(passwordValue),
        },
        runtime: runtimeOptions{
            remoteHost: host,
            deployPort: strings.TrimSpace(*deployPort),
            serverHost: serverHostValue,
        },
    }, nil
}

// buildDeployLink composes a basic xp2p+deploy link.
func buildDeployLink(opts deployOptions) string {
    // TODO: include token and options as needed
    host := strings.TrimSpace(opts.runtime.remoteHost)
    port := strings.TrimSpace(opts.runtime.deployPort)
    if port == "" {
        port = "62025"
    }
    return fmt.Sprintf("xp2p+deploy://%s:%s?v=1", host, port)
}
