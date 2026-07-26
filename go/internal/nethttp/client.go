package nethttp

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	defaultTimeout               = 3 * time.Second
	defaultIdleConnTimeout       = 30 * time.Second
	defaultMaxIdleConns          = 32
	defaultMaxIdleConnsPerHost   = 4
	defaultResponseHeaderTimeout = 3 * time.Second
	defaultTLSHandshakeTimeout   = 3 * time.Second
)

var ErrClientClosed = errors.New("HTTP client is closed")

type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

type OwnedClient interface {
	Doer
	Shutdown(context.Context) error
}

type OwnedDialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
	Shutdown(context.Context) error
}

type ClientOptions struct {
	Timeout               time.Duration
	DialTimeout           time.Duration
	TLSHandshakeTimeout   time.Duration
	ResponseHeaderTimeout time.Duration
	IdleConnTimeout       time.Duration
	MaxIdleConns          int
	MaxIdleConnsPerHost   int
	ForceHTTP2            bool
	TLSConfig             *tls.Config
	Dialer                OwnedDialer
}

type ownedClient struct {
	client    *http.Client
	transport *http.Transport
	dialer    OwnedDialer
	ctx       context.Context
	cancel    context.CancelFunc

	mu      sync.Mutex
	closed  bool
	active  int
	drained chan struct{}
	bodies  map[*trackedBody]struct{}
}

func NewClient(options ClientOptions) OwnedClient {
	options = withClientDefaults(options)
	dialer := options.Dialer
	if dialer == nil {
		dialer = directDialer{dialer: &net.Dialer{Timeout: options.DialTimeout}}
	}
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		TLSClientConfig:       cloneTLSConfig(options.TLSConfig),
		TLSHandshakeTimeout:   options.TLSHandshakeTimeout,
		ResponseHeaderTimeout: options.ResponseHeaderTimeout,
		IdleConnTimeout:       options.IdleConnTimeout,
		MaxIdleConns:          options.MaxIdleConns,
		MaxIdleConnsPerHost:   options.MaxIdleConnsPerHost,
		ForceAttemptHTTP2:     options.ForceHTTP2,
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &ownedClient{
		client:    &http.Client{Timeout: options.Timeout, Transport: transport},
		transport: transport,
		dialer:    dialer,
		ctx:       ctx,
		cancel:    cancel,
		drained:   closedChannel(),
		bodies:    make(map[*trackedBody]struct{}),
	}
}

func (c *ownedClient) Do(req *http.Request) (*http.Response, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, ErrClientClosed
	}
	if c.active == 0 {
		c.drained = make(chan struct{})
	}
	c.active++
	c.mu.Unlock()

	ctx, cancel := context.WithCancel(req.Context())
	stop := context.AfterFunc(c.ctx, cancel)
	resp, err := c.client.Do(req.Clone(ctx))
	if err != nil {
		stop()
		cancel()
		c.finishRequest()
		return nil, err
	}
	body := &trackedBody{
		ReadCloser: resp.Body,
	}
	body.done = func() {
		c.mu.Lock()
		delete(c.bodies, body)
		c.finishRequestLocked()
		c.mu.Unlock()
		stop()
		cancel()
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		_ = body.Close()
		return nil, ErrClientClosed
	}
	c.bodies[body] = struct{}{}
	resp.Body = body
	c.mu.Unlock()
	return resp, nil
}

func (c *ownedClient) finishRequest() {
	c.mu.Lock()
	c.finishRequestLocked()
	c.mu.Unlock()
}

func (c *ownedClient) finishRequestLocked() {
	c.active--
	if c.active == 0 {
		close(c.drained)
	}
}

func (c *ownedClient) Shutdown(ctx context.Context) error {
	c.mu.Lock()
	if !c.closed {
		c.closed = true
		c.cancel()
	}
	bodies := make([]*trackedBody, 0, len(c.bodies))
	for body := range c.bodies {
		bodies = append(bodies, body)
	}
	drained := c.drained
	c.mu.Unlock()

	for _, body := range bodies {
		_ = body.Close()
	}
	var waitErr error
	select {
	case <-drained:
	case <-ctx.Done():
		waitErr = ctx.Err()
	}
	c.transport.CloseIdleConnections()
	if err := c.dialer.Shutdown(ctx); waitErr == nil {
		waitErr = err
	}
	return waitErr
}

func closedChannel() chan struct{} {
	done := make(chan struct{})
	close(done)
	return done
}

type trackedBody struct {
	io.ReadCloser
	once sync.Once
	done func()
}

func (b *trackedBody) Close() error {
	err := b.ReadCloser.Close()
	b.once.Do(b.done)
	return err
}

type directDialer struct {
	dialer *net.Dialer
}

func (d directDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return d.dialer.DialContext(ctx, network, address)
}

func (directDialer) Shutdown(context.Context) error {
	return nil
}

func withClientDefaults(options ClientOptions) ClientOptions {
	if options.Timeout <= 0 {
		options.Timeout = defaultTimeout
	}
	if options.DialTimeout <= 0 {
		options.DialTimeout = options.Timeout
	}
	if options.TLSHandshakeTimeout <= 0 {
		options.TLSHandshakeTimeout = defaultTLSHandshakeTimeout
	}
	if options.ResponseHeaderTimeout <= 0 {
		options.ResponseHeaderTimeout = defaultResponseHeaderTimeout
	}
	if options.IdleConnTimeout <= 0 {
		options.IdleConnTimeout = defaultIdleConnTimeout
	}
	if options.MaxIdleConns <= 0 {
		options.MaxIdleConns = defaultMaxIdleConns
	}
	if options.MaxIdleConnsPerHost <= 0 {
		options.MaxIdleConnsPerHost = defaultMaxIdleConnsPerHost
	}
	options.ForceHTTP2 = true
	return options
}

func cloneTLSConfig(config *tls.Config) *tls.Config {
	if config == nil {
		return nil
	}
	return config.Clone()
}
