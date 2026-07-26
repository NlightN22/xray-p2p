package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
	"github.com/NlightN22/xray-p2p/go/internal/ha"
	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	xnethttp "github.com/NlightN22/xray-p2p/go/internal/nethttp"
)

const (
	// DefaultPort is the well known port used by xp2p helper services.
	DefaultPort = "62022"
)

// Options controls background server behaviour.
type Options struct {
	Port                string
	InstallDir          string
	CertPath            string
	KeyPath             string
	LiveDir             string
	TLSDir              string
	ListenAddr          string
	Quiet               bool
	ResourceLogInterval time.Duration
}

// StartBackground launches the HTTPS control endpoint. It is shut down
// automatically when the supplied context is cancelled.
func StartBackground(ctx context.Context, opts Options) (*BackgroundServer, error) {
	var hbStore *heartbeat.Store

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
	haStore, err := LoadHAReplication(config.ConfigPath(layout.ServerConfigFileName))
	if err != nil {
		return nil, fmt.Errorf("load HA replication state: %w", err)
	}

	certPath, keyPath, err := resolveControlTLS(opts)
	if err != nil {
		return nil, err
	}
	liveDir, err := resolveControlLiveDir(opts)
	if err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("start HTTPS control listener %s: %w", listenAddr, err)
	}
	srv := xnethttp.NewServer(controlplane.NewHandler(controlplane.HandlerOptions{
		LoadRuntime: func() (controlplane.Runtime, error) {
			return controlplane.LoadRuntimeFile(filepath.Join(liveDir, layout.RuntimeMetaFileName))
		},
		Heartbeat: hbStore,
		HAStore:   haStore,
		ReloadHA: func(store *ha.Store) error {
			fresh, err := LoadHAReplication(config.ConfigPath(layout.ServerConfigFileName))
			if err != nil {
				return err
			}
			return store.Refresh(fresh.Peers(), fresh.Committed())
		},
		Acknowledge: func(userLabel string, generation int) error {
			return AcknowledgeCredential(context.Background(), userLabel, generation)
		},
	}), xnethttp.ServerOptions{})
	return startOwnedHTTPServer(ctx, ln, srv, certPath, keyPath, "HTTPS control server", opts.ResourceLogInterval), nil
}

// StartStandaloneDiagnostics launches only the public readiness and ping endpoints.
func StartStandaloneDiagnostics(ctx context.Context, opts Options) (*BackgroundServer, error) {
	certPath, keyPath, err := resolveStandaloneTLS(opts)
	if err != nil {
		return nil, err
	}
	listenAddr := strings.TrimSpace(opts.ListenAddr)
	if listenAddr == "" {
		port := strings.TrimSpace(opts.Port)
		if port == "" {
			port = DefaultPort
		}
		listenAddr = ":" + port
	}
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("start HTTPS diagnostics listener %s: %w", listenAddr, err)
	}
	srv := xnethttp.NewServer(controlplane.NewDiagnosticsHandler(controlplane.DiagnosticsOptions{}), xnethttp.ServerOptions{})
	return startOwnedHTTPServer(ctx, ln, srv, certPath, keyPath, "HTTPS diagnostics server", opts.ResourceLogInterval), nil
}

func resolveStandaloneTLS(opts Options) (string, string, error) {
	certPath := strings.TrimSpace(opts.CertPath)
	keyPath := strings.TrimSpace(opts.KeyPath)
	if certPath != "" || keyPath != "" {
		if certPath == "" || keyPath == "" {
			return "", "", errors.New("diagnostics HTTPS TLS certificate and key are required")
		}
		return certPath, keyPath, nil
	}
	if strings.TrimSpace(opts.TLSDir) == "" {
		return "", "", errors.New("temporary diagnostics TLS directory is required")
	}
	return ensureControlTLS(opts)
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
	if certPath == "" && keyPath == "" {
		certPath, keyPath, err := ensureControlTLS(opts)
		if err != nil {
			return "", "", err
		}
		return certPath, keyPath, nil
	}
	if certPath == "" || keyPath == "" {
		return "", "", errors.New("control HTTPS TLS certificate and key are required")
	}
	return certPath, keyPath, nil
}

func ensureControlTLS(opts Options) (string, string, error) {
	port := strings.TrimSpace(opts.Port)
	if port == "" {
		port = DefaultPort
	}
	dir := strings.TrimSpace(opts.TLSDir)
	if dir == "" {
		dir = filepath.Join(config.ConfigRoot(), "tls", "control")
	}
	certPath := filepath.Join(dir, "control-"+port+".crt")
	keyPath := filepath.Join(dir, "control-"+port+".key")
	if controlTLSFileExists(certPath) && controlTLSFileExists(keyPath) {
		return certPath, keyPath, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", fmt.Errorf("create control tls dir: %w", err)
	}
	if err := generateSelfSignedCertificate(controlTLSHost(opts), certPath, keyPath); err != nil {
		return "", "", fmt.Errorf("generate control TLS certificate: %w", err)
	}
	return certPath, keyPath, nil
}

func controlTLSHost(opts Options) string {
	host := strings.TrimSpace(opts.ListenAddr)
	if host == "" {
		return "127.0.0.1"
	}
	if strings.HasPrefix(host, ":") {
		return "127.0.0.1"
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if host == "" || host == "0.0.0.0" || host == "::" {
		return "127.0.0.1"
	}
	return host
}

func controlTLSFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func resolveControlLiveDir(opts Options) (string, error) {
	liveDir := strings.TrimSpace(opts.LiveDir)
	if liveDir != "" {
		return liveDir, nil
	}
	return config.LiveRoleDir(apply.RoleServer)
}
