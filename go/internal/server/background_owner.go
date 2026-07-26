package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	xnethttp "github.com/NlightN22/xray-p2p/go/internal/nethttp"
	"github.com/NlightN22/xray-p2p/go/internal/xrayguard"
)

const defaultResourceLogInterval = time.Minute

type BackgroundServer struct {
	cancel   context.CancelFunc
	done     chan struct{}
	server   *xnethttp.Server
	stopOnce sync.Once
	resultMu sync.Mutex
	result   error
}

func startOwnedHTTPServer(
	ctx context.Context,
	listener net.Listener,
	server *xnethttp.Server,
	certPath string,
	keyPath string,
	name string,
	resourceLogInterval time.Duration,
) *BackgroundServer {
	ownerCtx, cancel := context.WithCancel(ctx)
	owner := &BackgroundServer{
		cancel: cancel,
		done:   make(chan struct{}),
		server: server,
	}
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- server.ServeTLS(listener, certPath, keyPath)
	}()
	go owner.run(ownerCtx, listener, serveDone, name, resourceLogInterval)
	return owner
}

func (s *BackgroundServer) run(
	ctx context.Context,
	listener net.Listener,
	serveDone <-chan error,
	name string,
	resourceLogInterval time.Duration,
) {
	defer close(s.done)
	if resourceLogInterval <= 0 {
		resourceLogInterval = defaultResourceLogInterval
	}
	ticker := time.NewTicker(resourceLogInterval)
	defer ticker.Stop()
	growth := resourceGrowthDetector{}
	collector := xrayguard.DefaultOptions().Collector

	for {
		select {
		case serveErr := <-serveDone:
			s.cancel()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), xnethttp.DefaultServerShutdownTimeout)
			shutdownErr := s.server.ShutdownOwned(shutdownCtx, listener)
			cancel()
			s.setResult(errors.Join(ignoreServeCloseError(serveErr), shutdownErr))
			return
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), xnethttp.DefaultServerShutdownTimeout)
			shutdownErr := s.server.ShutdownOwned(shutdownCtx, listener)
			cancel()
			serveErr := <-serveDone
			logServerResources(name+" stopped", s.server.Metrics(), collector)
			s.setResult(errors.Join(shutdownErr, ignoreServeCloseError(serveErr)))
			return
		case <-ticker.C:
			metrics := s.server.Metrics()
			sample := logServerResources(name+" resources", metrics, collector)
			connectionWarning, processWarning := growth.Observe(metrics.Current, sample.FDCount)
			if connectionWarning {
				logging.Warn(name+" connection count is growing monotonically",
					"connections_current", metrics.Current,
					"connections_peak", metrics.Peak,
					"samples", growth.connectionSamples,
				)
			}
			if processWarning {
				logging.Warn("process file descriptor count is growing monotonically",
					"observer", name,
					"scope", "process",
					"fd", sample.FDCount,
					"samples", growth.fdSamples,
				)
			}
		}
	}
}

func (s *BackgroundServer) Stop(ctx context.Context) error {
	s.stopOnce.Do(s.cancel)
	return s.Wait(ctx)
}

func (s *BackgroundServer) Wait(ctx context.Context) error {
	select {
	case <-s.done:
		s.resultMu.Lock()
		defer s.resultMu.Unlock()
		return s.result
	case <-ctx.Done():
		return fmt.Errorf("HTTP server shutdown: %w", ctx.Err())
	}
}

func (s *BackgroundServer) setResult(err error) {
	s.resultMu.Lock()
	s.result = err
	s.resultMu.Unlock()
}

func (s *BackgroundServer) Metrics() xnethttp.ServerMetrics {
	return s.server.Metrics()
}

type resourceGrowthDetector struct {
	previousConnections int64
	previousFD          int
	connectionSamples   int
	fdSamples           int
}

func (d *resourceGrowthDetector) Observe(connections int64, fd int) (bool, bool) {
	if connections > d.previousConnections {
		d.connectionSamples++
	} else {
		d.connectionSamples = 0
	}
	if fd > 0 && fd > d.previousFD {
		d.fdSamples++
	} else {
		d.fdSamples = 0
	}
	d.previousConnections = connections
	d.previousFD = fd
	return connections > 0 && d.connectionSamples >= 3, fd > 0 && d.fdSamples >= 3
}

func logServerResources(message string, metrics xnethttp.ServerMetrics, collector xrayguard.Collector) xrayguard.Sample {
	attrs := []any{
		"connections_new", metrics.New,
		"connections_active", metrics.Active,
		"connections_idle", metrics.Idle,
		"connections_closed", metrics.Closed,
		"connections_current", metrics.Current,
		"connections_peak", metrics.Peak,
	}
	var sample xrayguard.Sample
	if collector != nil {
		if collected, err := collector.Sample(context.Background(), os.Getpid()); err == nil {
			sample = collected
			attrs = append(attrs,
				"fd", sample.FDCount,
				"socket_fd", sample.SocketFDCount,
				"established_tcp", sample.EstablishedTCPCount,
			)
		}
	}
	logging.Info(message, attrs...)
	return sample
}

func ignoreServeCloseError(err error) error {
	if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}
