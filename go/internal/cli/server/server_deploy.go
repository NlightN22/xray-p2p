package servercmd

import (
	"context"
	"flag"
	"io"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func runServerDeploy(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p server deploy", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	listen := fs.String("listen", ":62025", "deploy listen address")
	link := fs.String("link", "", "deploy link (xp2p+deploy://...)")
	once := fs.Bool("once", true, "stop after a single deploy")
	timeout := fs.Duration("timeout", 10*time.Minute, "idle shutdown timeout")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	expected, _ := parseDeployLink(*link)
	logging.Info("xp2p server deploy: starting listener", "listen", *listen, "once", *once)

	srv := deployServer{
		ListenAddr: *listen,
		Expected:   expected,
		Once:       *once,
		Timeout:    *timeout,
		Cfg:        cfg,
	}
	if err := srv.Run(ctx); err != nil {
		logging.Error("xp2p server deploy: listener failed", "err", err)
		return 1
	}
	logging.Info("xp2p server deploy: stopped")
	return 0
}
