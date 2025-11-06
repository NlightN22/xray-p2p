package servercmd

import (
    "context"
    "flag"
    "fmt"
    "io"

    "github.com/NlightN22/xray-p2p/go/internal/config"
    "github.com/NlightN22/xray-p2p/go/internal/logging"
)

// TODO: Implement deploy link listener protocol.
func runServerDeploy(ctx context.Context, cfg config.Config, args []string) int {
    fs := flag.NewFlagSet("xp2p server deploy", flag.ContinueOnError)
    fs.SetOutput(io.Discard)

    listen := fs.String("listen", ":62025", "deploy listen address")
    link := fs.String("link", "", "deploy link (xp2p+deploy://...)")
    once := fs.Bool("once", true, "stop after a single deploy")

    if err := fs.Parse(args); err != nil {
        return 2
    }
    logging.Info("xp2p server deploy: stub", "listen", *listen, "once", *once, "link", *link)
    fmt.Println("xp2p server deploy: TODO implement deploy handshake")
    return 0
}

