package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

const (
	// DefaultPort is the well known port used by xp2p helper services.
	DefaultPort = "62022"
)

// Options controls background server behaviour.
type Options struct {
	Port       string
	InstallDir string
	CertPath   string
	KeyPath    string
	LiveDir    string
	ListenAddr string
	Quiet      bool
}

// StartBackground launches the HTTPS control endpoint. It is shut down
// automatically when the supplied context is cancelled.
func StartBackground(ctx context.Context, opts Options) error {
	var (
		once    sync.Once
		ln      net.Listener
		hbStore *heartbeat.Store
	)

	port := strings.TrimSpace(opts.Port)
	if port == "" {
		port = DefaultPort
	}
	listenAddr := strings.TrimSpace(opts.ListenAddr)
	if listenAddr == "" {
		listenAddr = ":" + port
	}

	storePath := ""
	storeRoot := strings.TrimSpace(opts.InstallDir)
	if runtime.GOOS == "windows" {
		storeRoot = config.ConfigRoot()
	}
	if storeRoot != "" {
		storePath = filepath.Join(storeRoot, layout.ServerHeartbeatStateFileName)
	}
	hbStore, storeErr := heartbeat.NewStore(storePath)
	if storeErr != nil {
		logging.Warn("heartbeat store disabled", "err", storeErr)
		hbStore, _ = heartbeat.NewStore("")
	}

	shutdown := func() {
		once.Do(func() {
			if ln != nil {
				_ = ln.Close()
			}
		})
	}

	certPath, keyPath, err := resolveControlTLS(opts)
	if err != nil {
		return err
	}
	liveDir, err := resolveControlLiveDir(opts)
	if err != nil {
		return err
	}
	ln, err = net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("start HTTPS control listener %s: %w", listenAddr, err)
	}
	srv := &http.Server{
		Handler: controlplane.NewHandler(controlplane.HandlerOptions{
			LoadRuntime: func() (controlplane.Runtime, error) {
				return controlplane.LoadRuntimeFile(filepath.Join(liveDir, layout.RuntimeMetaFileName))
			},
			Heartbeat: hbStore,
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		err := srv.ServeTLS(ln, certPath, keyPath)
		if err != nil && !errors.Is(err, http.ErrServerClosed) && ctx.Err() == nil {
			logging.Warn("HTTPS control server stopped", "err", err)
		}
	}()
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
		shutdown()
	}()

	return nil
}

func resolveControlTLS(opts Options) (string, string, error) {
	certPath := strings.TrimSpace(opts.CertPath)
	keyPath := strings.TrimSpace(opts.KeyPath)
	if certPath == "" && keyPath == "" {
		if liveDir, err := resolveControlLiveDir(opts); err == nil {
			if meta, metaErr := loadLiveRuntimeMeta(liveDir); metaErr == nil {
				certPath = strings.TrimSpace(meta.CertPath)
				keyPath = strings.TrimSpace(meta.KeyPath)
			}
		}
	}
	if certPath == "" && keyPath == "" && defaultTLSConfigured() {
		certPath = defaultCertPath()
		keyPath = defaultKeyPath()
	}
	if certPath == "" || keyPath == "" {
		return "", "", errors.New("control HTTPS TLS certificate and key are required")
	}
	return certPath, keyPath, nil
}

func resolveControlLiveDir(opts Options) (string, error) {
	liveDir := strings.TrimSpace(opts.LiveDir)
	if liveDir != "" {
		return liveDir, nil
	}
	return config.LiveRoleDir(apply.RoleServer)
}
