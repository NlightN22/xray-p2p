package xrayapi

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	commonnet "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/commonnet"
	routerconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/routerconfig"
	routingcommand "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/routingcommand"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
)

var errUnexpectedTypedMessage = errors.New("unexpected typed message")

func TestRoutingClientCallsRoutingService(t *testing.T) {
	service := &routingServer{
		list: []*routingcommand.ListRuleItem{
			{Tag: "proxy-alpha", RuleTag: "existing"},
		},
		testRouteResponse: &routingcommand.RoutingContext{
			OutboundTag:       "proxy-alpha",
			OutboundGroupTags: []string{"group-a"},
		},
		balancerInfo: &routingcommand.BalancerMsg{
			Override:        &routingcommand.OverrideInfo{Target: "proxy-beta"},
			PrincipleTarget: &routingcommand.PrincipleTargetInfo{Tag: []string{"proxy-alpha", "proxy-beta"}},
		},
	}
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	routingcommand.RegisterRoutingServiceServer(server, service)
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
	opts := RoutingRuleOptions{Address: "bufnet", Timeout: time.Second, Dialer: dialer}

	if err := AddRoutingRule(context.Background(), opts, map[string]any{
		"type":        "field",
		"ruleTag":     "xp2p-new",
		"outboundTag": "proxy-alpha",
		"domains":     []any{"app.example"},
	}); err != nil {
		t.Fatalf("AddRoutingRule: %v", err)
	}
	if service.added == nil || service.added.GetRuleTag() != "xp2p-new" || service.added.GetTag() != "proxy-alpha" {
		t.Fatalf("unexpected added rule: %+v", service.added)
	}

	if err := RemoveRoutingRule(context.Background(), opts, "old-rule"); err != nil {
		t.Fatalf("RemoveRoutingRule: %v", err)
	}
	if service.removed != "old-rule" {
		t.Fatalf("removed = %q, want old-rule", service.removed)
	}

	rules, err := ListRoutingRules(context.Background(), opts)
	if err != nil {
		t.Fatalf("ListRoutingRules: %v", err)
	}
	if len(rules) != 1 || rules[0].RuleTag != "existing" || rules[0].Tag != "proxy-alpha" {
		t.Fatalf("unexpected rules: %+v", rules)
	}

	route, err := TestRoute(context.Background(), opts, RouteTest{
		InboundTag:     "socks-in",
		Network:        "tcp",
		TargetDomain:   "app.example",
		TargetPort:     443,
		FieldSelectors: []string{"outbound"},
	})
	if err != nil {
		t.Fatalf("TestRoute: %v", err)
	}
	if route.OutboundTag != "proxy-alpha" || len(route.OutboundGroupTags) != 1 || route.OutboundGroupTags[0] != "group-a" {
		t.Fatalf("unexpected route result: %+v", route)
	}
	if service.testRouteRequest.GetRoutingContext().GetInboundTag() != "socks-in" ||
		service.testRouteRequest.GetRoutingContext().GetNetwork() != commonnet.Network_TCP ||
		service.testRouteRequest.GetRoutingContext().GetTargetDomain() != "app.example" ||
		service.testRouteRequest.GetRoutingContext().GetTargetPort() != 443 {
		t.Fatalf("unexpected route request: %+v", service.testRouteRequest.GetRoutingContext())
	}

	info, err := GetBalancerInfo(context.Background(), opts, "balancer-a")
	if err != nil {
		t.Fatalf("GetBalancerInfo: %v", err)
	}
	if info.OverrideTarget != "proxy-beta" || len(info.PrincipleTargets) != 2 {
		t.Fatalf("unexpected balancer info: %+v", info)
	}
	if service.balancerInfoTag != "balancer-a" {
		t.Fatalf("balancer info tag = %q, want balancer-a", service.balancerInfoTag)
	}

	if err := OverrideBalancerTarget(context.Background(), opts, "balancer-a", "proxy-alpha"); err != nil {
		t.Fatalf("OverrideBalancerTarget: %v", err)
	}
	if service.overrideBalancerTag != "balancer-a" || service.overrideTarget != "proxy-alpha" {
		t.Fatalf("unexpected balancer override: tag=%q target=%q", service.overrideBalancerTag, service.overrideTarget)
	}
}

type routingServer struct {
	routingcommand.UnimplementedRoutingServiceServer
	added               *routerconfig.RoutingRule
	removed             string
	list                []*routingcommand.ListRuleItem
	testRouteRequest    *routingcommand.TestRouteRequest
	testRouteResponse   *routingcommand.RoutingContext
	balancerInfoTag     string
	balancerInfo        *routingcommand.BalancerMsg
	overrideBalancerTag string
	overrideTarget      string
}

func (s *routingServer) AddRule(_ context.Context, request *routingcommand.AddRuleRequest) (*routingcommand.AddRuleResponse, error) {
	if request.GetConfig().GetType() != "xray.app.router.Config" {
		return nil, errUnexpectedTypedMessage
	}
	var config routerconfig.Config
	if err := proto.Unmarshal(request.GetConfig().GetValue(), &config); err != nil {
		return nil, err
	}
	if len(config.GetRule()) != 1 {
		return nil, errUnexpectedTypedMessage
	}
	s.added = config.GetRule()[0]
	return &routingcommand.AddRuleResponse{}, nil
}

func (s *routingServer) RemoveRule(_ context.Context, request *routingcommand.RemoveRuleRequest) (*routingcommand.RemoveRuleResponse, error) {
	s.removed = request.GetRuleTag()
	return &routingcommand.RemoveRuleResponse{}, nil
}

func (s *routingServer) ListRule(context.Context, *routingcommand.ListRuleRequest) (*routingcommand.ListRuleResponse, error) {
	return &routingcommand.ListRuleResponse{Rules: s.list}, nil
}

func (s *routingServer) TestRoute(_ context.Context, request *routingcommand.TestRouteRequest) (*routingcommand.RoutingContext, error) {
	s.testRouteRequest = request
	return s.testRouteResponse, nil
}

func (s *routingServer) GetBalancerInfo(_ context.Context, request *routingcommand.GetBalancerInfoRequest) (*routingcommand.GetBalancerInfoResponse, error) {
	s.balancerInfoTag = request.GetTag()
	return &routingcommand.GetBalancerInfoResponse{Balancer: s.balancerInfo}, nil
}

func (s *routingServer) OverrideBalancerTarget(_ context.Context, request *routingcommand.OverrideBalancerTargetRequest) (*routingcommand.OverrideBalancerTargetResponse, error) {
	s.overrideBalancerTag = request.GetBalancerTag()
	s.overrideTarget = request.GetTarget()
	return &routingcommand.OverrideBalancerTargetResponse{}, nil
}
