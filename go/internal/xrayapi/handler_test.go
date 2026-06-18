package xrayapi

import (
	"context"
	"net"
	"reflect"
	"testing"
	"time"

	coreconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/coreconfig"
	handlercommand "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/handlercommand"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

func TestHandlerClientAddRemoveListInboundOutbound(t *testing.T) {
	server := newTestHandlerServer()
	dialer, cleanup := startHandlerBufconnServer(t, server)
	defer cleanup()

	client, err := DialWith(context.Background(), "bufnet", time.Second, dialer)
	if err != nil {
		t.Fatalf("DialWith: %v", err)
	}
	defer client.Close()

	if err := client.AddInbound(context.Background(), &coreconfig.InboundHandlerConfig{Tag: "in-a"}); err != nil {
		t.Fatalf("AddInbound: %v", err)
	}
	if err := client.AddOutbound(context.Background(), &coreconfig.OutboundHandlerConfig{Tag: "out-a"}); err != nil {
		t.Fatalf("AddOutbound: %v", err)
	}
	inTags, err := client.ListInboundTags(context.Background())
	if err != nil {
		t.Fatalf("ListInboundTags: %v", err)
	}
	if !reflect.DeepEqual(inTags, []string{"in-a"}) {
		t.Fatalf("inTags = %v", inTags)
	}
	outTags, err := client.ListOutboundTags(context.Background())
	if err != nil {
		t.Fatalf("ListOutboundTags: %v", err)
	}
	if !reflect.DeepEqual(outTags, []string{"out-a"}) {
		t.Fatalf("outTags = %v", outTags)
	}
	if err := client.RemoveInbound(context.Background(), "in-a"); err != nil {
		t.Fatalf("RemoveInbound: %v", err)
	}
	if err := client.RemoveOutbound(context.Background(), "out-a"); err != nil {
		t.Fatalf("RemoveOutbound: %v", err)
	}
	inTags, err = client.ListInboundTags(context.Background())
	if err != nil {
		t.Fatalf("ListInboundTags after remove: %v", err)
	}
	if len(inTags) != 0 {
		t.Fatalf("inTags after remove = %v", inTags)
	}
	outTags, err = client.ListOutboundTags(context.Background())
	if err != nil {
		t.Fatalf("ListOutboundTags after remove: %v", err)
	}
	if len(outTags) != 0 {
		t.Fatalf("outTags after remove = %v", outTags)
	}
}

func startHandlerBufconnServer(t *testing.T, handler handlercommand.HandlerServiceServer) (Dialer, func()) {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	handlercommand.RegisterHandlerServiceServer(grpcServer, handler)
	go func() {
		_ = grpcServer.Serve(listener)
	}()
	dialer := func(ctx context.Context, _ string, _ time.Duration) (*grpc.ClientConn, error) {
		return grpc.DialContext(ctx, "bufnet",
			grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
				return listener.Dial()
			}),
			grpc.WithInsecure(),
		)
	}
	return dialer, func() {
		grpcServer.Stop()
		_ = listener.Close()
	}
}

type testHandlerServer struct {
	handlercommand.UnimplementedHandlerServiceServer
	inbounds  map[string]*coreconfig.InboundHandlerConfig
	outbounds map[string]*coreconfig.OutboundHandlerConfig
}

func newTestHandlerServer() *testHandlerServer {
	return &testHandlerServer{
		inbounds:  make(map[string]*coreconfig.InboundHandlerConfig),
		outbounds: make(map[string]*coreconfig.OutboundHandlerConfig),
	}
}

func (s *testHandlerServer) AddInbound(_ context.Context, req *handlercommand.AddInboundRequest) (*handlercommand.AddInboundResponse, error) {
	s.inbounds[req.GetInbound().GetTag()] = req.GetInbound()
	return &handlercommand.AddInboundResponse{}, nil
}

func (s *testHandlerServer) RemoveInbound(_ context.Context, req *handlercommand.RemoveInboundRequest) (*handlercommand.RemoveInboundResponse, error) {
	delete(s.inbounds, req.GetTag())
	return &handlercommand.RemoveInboundResponse{}, nil
}

func (s *testHandlerServer) ListInbounds(context.Context, *handlercommand.ListInboundsRequest) (*handlercommand.ListInboundsResponse, error) {
	items := make([]*coreconfig.InboundHandlerConfig, 0, len(s.inbounds))
	for _, item := range s.inbounds {
		items = append(items, item)
	}
	return &handlercommand.ListInboundsResponse{Inbounds: items}, nil
}

func (s *testHandlerServer) AddOutbound(_ context.Context, req *handlercommand.AddOutboundRequest) (*handlercommand.AddOutboundResponse, error) {
	s.outbounds[req.GetOutbound().GetTag()] = req.GetOutbound()
	return &handlercommand.AddOutboundResponse{}, nil
}

func (s *testHandlerServer) RemoveOutbound(_ context.Context, req *handlercommand.RemoveOutboundRequest) (*handlercommand.RemoveOutboundResponse, error) {
	delete(s.outbounds, req.GetTag())
	return &handlercommand.RemoveOutboundResponse{}, nil
}

func (s *testHandlerServer) ListOutbounds(context.Context, *handlercommand.ListOutboundsRequest) (*handlercommand.ListOutboundsResponse, error) {
	items := make([]*coreconfig.OutboundHandlerConfig, 0, len(s.outbounds))
	for _, item := range s.outbounds {
		items = append(items, item)
	}
	return &handlercommand.ListOutboundsResponse{Outbounds: items}, nil
}
