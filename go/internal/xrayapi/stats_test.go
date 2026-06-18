package xrayapi

import (
	"context"
	"net"
	"testing"
	"time"

	statscommand "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/statscommand"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestQueryStatsUsesStatsService(t *testing.T) {
	server := grpc.NewServer()
	statscommand.RegisterStatsServiceServer(server, &statsServer{
		stats: []*statscommand.Stat{
			{Name: "user>>>alice>>>traffic>>>uplink", Value: 10},
			{Name: "user>>>alice>>>traffic>>>downlink", Value: 20},
		},
	})
	listener := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})
	go func() {
		_ = server.Serve(listener)
	}()

	stats, err := QueryStats(context.Background(), StatsQueryOptions{
		Address: "bufnet",
		Pattern: "user>>>",
		Timeout: time.Second,
		Dialer: func(ctx context.Context, address string, timeout time.Duration) (*grpc.ClientConn, error) {
			dialCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			return grpc.DialContext(
				dialCtx,
				address,
				grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
					return listener.Dial()
				}),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithBlock(),
			)
		},
	})
	if err != nil {
		t.Fatalf("QueryStats: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("got %d stats, want 2: %+v", len(stats), stats)
	}
	if stats[0].Name != "user>>>alice>>>traffic>>>uplink" || stats[0].Value != 10 {
		t.Fatalf("unexpected first stat: %+v", stats[0])
	}
	if stats[1].Name != "user>>>alice>>>traffic>>>downlink" || stats[1].Value != 20 {
		t.Fatalf("unexpected second stat: %+v", stats[1])
	}
}

type statsServer struct {
	statscommand.UnimplementedStatsServiceServer
	stats []*statscommand.Stat
}

func (s *statsServer) QueryStats(context.Context, *statscommand.QueryStatsRequest) (*statscommand.QueryStatsResponse, error) {
	return &statscommand.QueryStatsResponse{Stat: s.stats}, nil
}
