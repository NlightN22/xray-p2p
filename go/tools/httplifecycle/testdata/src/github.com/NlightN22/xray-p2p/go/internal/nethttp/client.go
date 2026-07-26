package nethttp

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
)

type OwnedClient interface {
	Do(*http.Request) (*http.Response, error)
	Shutdown(context.Context) error
}

type ClientOptions struct {
	TLSConfig   *tls.Config
	DialContext func(context.Context, string, string) (net.Conn, error)
}

type client struct{}

func (client) Do(*http.Request) (*http.Response, error) { return nil, nil }
func (client) Shutdown(context.Context) error           { return nil }

func NewClient(ClientOptions) OwnedClient {
	return client{}
}

func NewClientWithError(ClientOptions) (OwnedClient, error) {
	return client{}, nil
}
