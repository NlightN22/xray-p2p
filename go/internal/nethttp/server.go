package nethttp

import (
	"context"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

const (
	DefaultServerReadHeaderTimeout = 5 * time.Second
	DefaultServerReadTimeout       = 15 * time.Second
	DefaultServerWriteTimeout      = 30 * time.Second
	DefaultServerIdleTimeout       = 30 * time.Second
	DefaultServerShutdownTimeout   = 5 * time.Second
)

type ServerOptions struct {
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

type ServerMetrics struct {
	New     int64
	Active  int64
	Idle    int64
	Closed  int64
	Current int64
	Peak    int64
}

type Server struct {
	*http.Server
	states  sync.Map
	new     atomic.Int64
	active  atomic.Int64
	idle    atomic.Int64
	closed  atomic.Int64
	current atomic.Int64
	peak    atomic.Int64
}

func NewServer(handler http.Handler, options ServerOptions) *Server {
	options = withServerDefaults(options)
	server := &Server{}
	server.Server = &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: options.ReadHeaderTimeout,
		ReadTimeout:       options.ReadTimeout,
		WriteTimeout:      options.WriteTimeout,
		IdleTimeout:       options.IdleTimeout,
		ConnState:         server.observeConnState,
	}
	return server
}

func (s *Server) ShutdownOwned(listener net.Listener) error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultServerShutdownTimeout)
	defer cancel()
	err := s.Shutdown(ctx)
	if err != nil {
		_ = s.Close()
	}
	if listener != nil {
		_ = listener.Close()
	}
	return err
}

func (s *Server) Metrics() ServerMetrics {
	return ServerMetrics{
		New: s.new.Load(), Active: s.active.Load(), Idle: s.idle.Load(),
		Closed: s.closed.Load(), Current: s.current.Load(), Peak: s.peak.Load(),
	}
}

func (s *Server) observeConnState(conn net.Conn, state http.ConnState) {
	previous, loaded := s.states.LoadOrStore(conn, http.StateNew)
	if loaded {
		s.decrement(previous.(http.ConnState))
	}
	s.increment(state)
	if state == http.StateClosed || state == http.StateHijacked {
		s.states.Delete(conn)
	} else {
		s.states.Store(conn, state)
	}
}

func (s *Server) increment(state http.ConnState) {
	switch state {
	case http.StateNew:
		s.new.Add(1)
		current := s.current.Add(1)
		for peak := s.peak.Load(); current > peak && !s.peak.CompareAndSwap(peak, current); peak = s.peak.Load() {
		}
	case http.StateActive:
		s.active.Add(1)
	case http.StateIdle:
		s.idle.Add(1)
	case http.StateClosed, http.StateHijacked:
		s.closed.Add(1)
		s.current.Add(-1)
	}
}

func (s *Server) decrement(state http.ConnState) {
	switch state {
	case http.StateActive:
		s.active.Add(-1)
	case http.StateIdle:
		s.idle.Add(-1)
	}
}

func withServerDefaults(options ServerOptions) ServerOptions {
	if options.ReadHeaderTimeout <= 0 {
		options.ReadHeaderTimeout = DefaultServerReadHeaderTimeout
	}
	if options.ReadTimeout <= 0 {
		options.ReadTimeout = DefaultServerReadTimeout
	}
	if options.WriteTimeout <= 0 {
		options.WriteTimeout = DefaultServerWriteTimeout
	}
	if options.IdleTimeout <= 0 {
		options.IdleTimeout = DefaultServerIdleTimeout
	}
	return options
}
