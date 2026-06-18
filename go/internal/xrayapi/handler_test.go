package xrayapi

import (
	"context"
	"net"
	"reflect"
	"testing"
	"time"

	commonprotocol "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/commonprotocol"
	coreconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/coreconfig"
	handlercommand "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/handlercommand"
	trojanconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/trojanconfig"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
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
	if err := client.AddInboundUser(context.Background(), "in-a", "alpha@example.com", "secret"); err != nil {
		t.Fatalf("AddInboundUser: %v", err)
	}
	users, err := client.ListInboundUserEmails(context.Background(), "in-a")
	if err != nil {
		t.Fatalf("ListInboundUserEmails: %v", err)
	}
	if !reflect.DeepEqual(users, []string{"alpha@example.com"}) {
		t.Fatalf("users = %v", users)
	}
	if err := client.RemoveInboundUser(context.Background(), "in-a", "alpha@example.com"); err != nil {
		t.Fatalf("RemoveInboundUser: %v", err)
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
	users     map[string]map[string]*commonprotocol.User
}

func newTestHandlerServer() *testHandlerServer {
	return &testHandlerServer{
		inbounds:  make(map[string]*coreconfig.InboundHandlerConfig),
		outbounds: make(map[string]*coreconfig.OutboundHandlerConfig),
		users:     make(map[string]map[string]*commonprotocol.User),
	}
}

func (s *testHandlerServer) AddInbound(_ context.Context, req *handlercommand.AddInboundRequest) (*handlercommand.AddInboundResponse, error) {
	s.inbounds[req.GetInbound().GetTag()] = req.GetInbound()
	return &handlercommand.AddInboundResponse{}, nil
}

func (s *testHandlerServer) RemoveInbound(_ context.Context, req *handlercommand.RemoveInboundRequest) (*handlercommand.RemoveInboundResponse, error) {
	delete(s.inbounds, req.GetTag())
	delete(s.users, req.GetTag())
	return &handlercommand.RemoveInboundResponse{}, nil
}

func (s *testHandlerServer) AlterInbound(_ context.Context, req *handlercommand.AlterInboundRequest) (*handlercommand.AlterInboundResponse, error) {
	switch req.GetOperation().GetType() {
	case "xray.app.proxyman.command.AddUserOperation":
		op := &handlercommand.AddUserOperation{}
		if err := proto.Unmarshal(req.GetOperation().GetValue(), op); err != nil {
			return nil, err
		}
		account := &trojanconfig.Account{}
		if err := proto.Unmarshal(op.GetUser().GetAccount().GetValue(), account); err != nil {
			return nil, err
		}
		if account.GetPassword() == "" {
			return nil, errUnexpectedTypedMessage
		}
		if s.users[req.GetTag()] == nil {
			s.users[req.GetTag()] = make(map[string]*commonprotocol.User)
		}
		s.users[req.GetTag()][op.GetUser().GetEmail()] = op.GetUser()
	case "xray.app.proxyman.command.RemoveUserOperation":
		op := &handlercommand.RemoveUserOperation{}
		if err := proto.Unmarshal(req.GetOperation().GetValue(), op); err != nil {
			return nil, err
		}
		delete(s.users[req.GetTag()], op.GetEmail())
	default:
		return nil, errUnexpectedTypedMessage
	}
	return &handlercommand.AlterInboundResponse{}, nil
}

func (s *testHandlerServer) ListInbounds(context.Context, *handlercommand.ListInboundsRequest) (*handlercommand.ListInboundsResponse, error) {
	items := make([]*coreconfig.InboundHandlerConfig, 0, len(s.inbounds))
	for _, item := range s.inbounds {
		items = append(items, item)
	}
	return &handlercommand.ListInboundsResponse{Inbounds: items}, nil
}

func (s *testHandlerServer) GetInboundUsers(_ context.Context, req *handlercommand.GetInboundUserRequest) (*handlercommand.GetInboundUserResponse, error) {
	users := s.users[req.GetTag()]
	items := make([]*commonprotocol.User, 0, len(users))
	for _, item := range users {
		if req.GetEmail() == "" || item.GetEmail() == req.GetEmail() {
			items = append(items, item)
		}
	}
	return &handlercommand.GetInboundUserResponse{Users: items}, nil
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
