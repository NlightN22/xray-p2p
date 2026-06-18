package xrayapi

import (
	"context"
	"errors"
	"strings"
	"time"

	routingcommand "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/routingcommand"
	statscommand "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/statscommand"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const DefaultTimeout = 3 * time.Second

type Dialer func(context.Context, string, time.Duration) (*grpc.ClientConn, error)

type Client struct {
	conn    *grpc.ClientConn
	stats   statscommand.StatsServiceClient
	routing routingcommand.RoutingServiceClient
}

func Dial(ctx context.Context, address string, timeout time.Duration) (*Client, error) {
	return DialWith(ctx, address, timeout, defaultDialer)
}

func DialWith(ctx context.Context, address string, timeout time.Duration, dialer Dialer) (*Client, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, errors.New("xray API address is empty")
	}
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if dialer == nil {
		dialer = defaultDialer
	}
	conn, err := dialer(ctx, address, timeout)
	if err != nil {
		return nil, err
	}
	return &Client{
		conn:    conn,
		stats:   statscommand.NewStatsServiceClient(conn),
		routing: routingcommand.NewRoutingServiceClient(conn),
	}, nil
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func defaultDialer(ctx context.Context, address string, timeout time.Duration) (*grpc.ClientConn, error) {
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return grpc.DialContext(
		dialCtx,
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
}
