package xrayapi

import (
	"context"
	"net"
	"testing"
	"time"

	observatorycommand "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/observatorycommand"
	observatoryconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/observatoryconfig"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestGetOutboundStatusesCallsObservatoryService(t *testing.T) {
	service := &observatoryServer{
		statuses: []*observatoryconfig.OutboundStatus{
			{
				OutboundTag:     "proxy-alpha",
				Alive:           true,
				Delay:           42,
				LastSeenTime:    100,
				LastTryTime:     120,
				HealthPing:      &observatoryconfig.HealthPingMeasurementResult{All: 4, Fail: 1, Average: 45},
				LastErrorReason: "",
			},
			{
				OutboundTag:     "proxy-beta",
				Alive:           false,
				LastErrorReason: "timeout",
			},
		},
	}
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	observatorycommand.RegisterObservatoryServiceServer(server, service)
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})
	go func() {
		_ = server.Serve(listener)
	}()

	dialer := func(ctx context.Context, address string, timeout time.Duration) (*grpc.ClientConn, error) {
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
	}

	statuses, err := GetOutboundStatuses(context.Background(), ObservatoryOptions{
		Address: "bufnet",
		Timeout: time.Second,
		Dialer:  dialer,
	})
	if err != nil {
		t.Fatalf("GetOutboundStatuses: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("statuses = %+v, want 2 items", statuses)
	}
	if statuses[0].Tag != "proxy-alpha" || !statuses[0].Alive || statuses[0].DelayMillis != 42 || statuses[0].HealthAverageMs != 45 {
		t.Fatalf("unexpected first status: %+v", statuses[0])
	}
	if statuses[1].Tag != "proxy-beta" || statuses[1].Alive || statuses[1].LastError != "timeout" {
		t.Fatalf("unexpected second status: %+v", statuses[1])
	}
}

type observatoryServer struct {
	observatorycommand.UnimplementedObservatoryServiceServer
	statuses []*observatoryconfig.OutboundStatus
}

func (s *observatoryServer) GetOutboundStatus(context.Context, *observatorycommand.GetOutboundStatusRequest) (*observatorycommand.GetOutboundStatusResponse, error) {
	return &observatorycommand.GetOutboundStatusResponse{
		Status: &observatoryconfig.ObservationResult{Status: s.statuses},
	}, nil
}
