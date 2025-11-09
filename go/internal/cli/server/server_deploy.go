package servercmd

import (
	"context"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	deploylink "github.com/NlightN22/xray-p2p/go/internal/deploy/link"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

type serverDeployOptions struct {
	Listen  string
	Link    string
	Once    bool
	Timeout time.Duration
}

func runServerDeploy(ctx context.Context, cfg config.Config, opts serverDeployOptions) int {
	listenAddr := strings.TrimSpace(opts.Listen)
	if listenAddr == "" {
		listenAddr = ":62025"
	}

	var expected deploylink.EncryptedLink
	rawLink := strings.TrimSpace(opts.Link)
	if rawLink != "" {
		var err error
		expected, err = parseDeployLink(rawLink)
		if err != nil {
			logging.Error("xp2p server deploy: invalid --link", "err", err)
			return 2
		}
	}

	if rawLink == "" {
		logging.Error("xp2p server deploy: --link is required")
		return 2
	}

	logging.Info("xp2p server deploy: starting listener", "listen", listenAddr, "once", opts.Once)

	srv := deployServer{
		ListenAddr: listenAddr,
		Expected:   expected,
		Once:       opts.Once,
		Timeout:    opts.Timeout,
		Cfg:        cfg,
	}
	if srv.Timeout <= 0 {
		srv.Timeout = 10 * time.Minute
	}
	if err := srv.Run(ctx); err != nil {
		logging.Error("xp2p server deploy: listener failed", "err", err)
		return 1
	}
	logging.Info("xp2p server deploy: stopped")
	return 0
}
