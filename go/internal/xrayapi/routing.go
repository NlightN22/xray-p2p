package xrayapi

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	commonnet "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/commonnet"
	routerconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/routerconfig"
	routingcommand "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/routingcommand"
)

type RoutingRuleInfo struct {
	Tag     string
	RuleTag string
}

type RouteTest struct {
	InboundTag     string
	Network        string
	TargetIP       string
	TargetDomain   string
	TargetPort     uint32
	User           string
	FieldSelectors []string
	PublishResult  bool
}

type RouteTestResult struct {
	OutboundTag       string
	OutboundGroupTags []string
}

type BalancerInfo struct {
	OverrideTarget   string
	PrincipleTargets []string
}

type RoutingRuleOptions struct {
	Address string
	Timeout time.Duration
	Dialer  Dialer
}

func AddRoutingRule(ctx context.Context, opts RoutingRuleOptions, rule map[string]any) error {
	client, err := DialWith(ctx, opts.Address, opts.Timeout, opts.Dialer)
	if err != nil {
		return fmt.Errorf("connect xray API: %w", err)
	}
	defer client.Close()
	return client.AddRule(ctx, rule)
}

func RemoveRoutingRule(ctx context.Context, opts RoutingRuleOptions, ruleTag string) error {
	client, err := DialWith(ctx, opts.Address, opts.Timeout, opts.Dialer)
	if err != nil {
		return fmt.Errorf("connect xray API: %w", err)
	}
	defer client.Close()
	return client.RemoveRule(ctx, ruleTag)
}

func ListRoutingRules(ctx context.Context, opts RoutingRuleOptions) ([]RoutingRuleInfo, error) {
	client, err := DialWith(ctx, opts.Address, opts.Timeout, opts.Dialer)
	if err != nil {
		return nil, fmt.Errorf("connect xray API: %w", err)
	}
	defer client.Close()
	return client.ListRules(ctx)
}

func TestRoute(ctx context.Context, opts RoutingRuleOptions, route RouteTest) (RouteTestResult, error) {
	client, err := DialWith(ctx, opts.Address, opts.Timeout, opts.Dialer)
	if err != nil {
		return RouteTestResult{}, fmt.Errorf("connect xray API: %w", err)
	}
	defer client.Close()
	return client.TestRoute(ctx, route)
}

func GetBalancerInfo(ctx context.Context, opts RoutingRuleOptions, tag string) (BalancerInfo, error) {
	client, err := DialWith(ctx, opts.Address, opts.Timeout, opts.Dialer)
	if err != nil {
		return BalancerInfo{}, fmt.Errorf("connect xray API: %w", err)
	}
	defer client.Close()
	return client.GetBalancerInfo(ctx, tag)
}

func OverrideBalancerTarget(ctx context.Context, opts RoutingRuleOptions, balancerTag, target string) error {
	client, err := DialWith(ctx, opts.Address, opts.Timeout, opts.Dialer)
	if err != nil {
		return fmt.Errorf("connect xray API: %w", err)
	}
	defer client.Close()
	return client.OverrideBalancerTarget(ctx, balancerTag, target)
}

func (c *Client) AddRule(ctx context.Context, rule map[string]any) error {
	protoRule, err := routingRuleFromMap(cloneRuleMap(rule))
	if err != nil {
		return err
	}
	msg, err := typedMessage(&routerconfig.Config{Rule: []*routerconfig.RoutingRule{protoRule}})
	if err != nil {
		return err
	}
	_, err = c.routing.AddRule(ctx, &routingcommand.AddRuleRequest{
		Config:       msg,
		ShouldAppend: true,
	})
	if err != nil {
		return fmt.Errorf("add routing rule: %w", err)
	}
	return nil
}

func (c *Client) RemoveRule(ctx context.Context, ruleTag string) error {
	_, err := c.routing.RemoveRule(ctx, &routingcommand.RemoveRuleRequest{RuleTag: ruleTag})
	if err != nil {
		return fmt.Errorf("remove routing rule: %w", err)
	}
	return nil
}

func (c *Client) ListRules(ctx context.Context) ([]RoutingRuleInfo, error) {
	response, err := c.routing.ListRule(ctx, &routingcommand.ListRuleRequest{})
	if err != nil {
		return nil, fmt.Errorf("list routing rules: %w", err)
	}
	result := make([]RoutingRuleInfo, 0, len(response.GetRules()))
	for _, rule := range response.GetRules() {
		if rule == nil {
			continue
		}
		result = append(result, RoutingRuleInfo{
			Tag:     rule.GetTag(),
			RuleTag: rule.GetRuleTag(),
		})
	}
	return result, nil
}

func (c *Client) ListRuleTags(ctx context.Context) ([]string, error) {
	rules, err := c.ListRules(ctx)
	if err != nil {
		return nil, err
	}
	tags := make([]string, 0, len(rules))
	for _, rule := range rules {
		if rule.RuleTag != "" {
			tags = append(tags, rule.RuleTag)
		}
	}
	return tags, nil
}

func (c *Client) TestRoute(ctx context.Context, route RouteTest) (RouteTestResult, error) {
	request, err := routeTestRequest(route)
	if err != nil {
		return RouteTestResult{}, err
	}
	response, err := c.routing.TestRoute(ctx, request)
	if err != nil {
		return RouteTestResult{}, fmt.Errorf("test route: %w", err)
	}
	return RouteTestResult{
		OutboundTag:       response.GetOutboundTag(),
		OutboundGroupTags: append([]string(nil), response.GetOutboundGroupTags()...),
	}, nil
}

func (c *Client) GetBalancerInfo(ctx context.Context, tag string) (BalancerInfo, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return BalancerInfo{}, fmt.Errorf("balancer tag is required")
	}
	response, err := c.routing.GetBalancerInfo(ctx, &routingcommand.GetBalancerInfoRequest{Tag: tag})
	if err != nil {
		return BalancerInfo{}, fmt.Errorf("get balancer info: %w", err)
	}
	balancer := response.GetBalancer()
	return BalancerInfo{
		OverrideTarget:   balancer.GetOverride().GetTarget(),
		PrincipleTargets: append([]string(nil), balancer.GetPrincipleTarget().GetTag()...),
	}, nil
}

func (c *Client) OverrideBalancerTarget(ctx context.Context, balancerTag, target string) error {
	balancerTag = strings.TrimSpace(balancerTag)
	target = strings.TrimSpace(target)
	if balancerTag == "" {
		return fmt.Errorf("balancer tag is required")
	}
	if target == "" {
		return fmt.Errorf("balancer target is required")
	}
	_, err := c.routing.OverrideBalancerTarget(ctx, &routingcommand.OverrideBalancerTargetRequest{
		BalancerTag: balancerTag,
		Target:      target,
	})
	if err != nil {
		return fmt.Errorf("override balancer target: %w", err)
	}
	return nil
}

func routeTestRequest(route RouteTest) (*routingcommand.TestRouteRequest, error) {
	ctx := &routingcommand.RoutingContext{
		InboundTag:   strings.TrimSpace(route.InboundTag),
		Network:      routeNetwork(route.Network),
		TargetDomain: strings.TrimSpace(route.TargetDomain),
		TargetPort:   route.TargetPort,
		User:         strings.TrimSpace(route.User),
	}
	if targetIP := strings.TrimSpace(route.TargetIP); targetIP != "" {
		parsed := net.ParseIP(targetIP)
		if parsed == nil {
			return nil, fmt.Errorf("invalid route target IP %q", targetIP)
		}
		ctx.TargetIPs = [][]byte{ipBytes(parsed)}
	}
	return &routingcommand.TestRouteRequest{
		RoutingContext: ctx,
		FieldSelectors: append([]string(nil), route.FieldSelectors...),
		PublishResult:  route.PublishResult,
	}, nil
}

func routeNetwork(value string) commonnet.Network {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "udp":
		return commonnet.Network_UDP
	case "unix":
		return commonnet.Network_UNIX
	case "", "tcp":
		return commonnet.Network_TCP
	default:
		return commonnet.Network_Unknown
	}
}
