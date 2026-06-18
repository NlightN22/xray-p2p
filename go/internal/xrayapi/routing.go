package xrayapi

import (
	"context"
	"fmt"
	"time"

	commonserial "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/commonserial"
	routerconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/routerconfig"
	routingcommand "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/routingcommand"
	"google.golang.org/protobuf/proto"
)

type RoutingRuleInfo struct {
	Tag     string
	RuleTag string
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

func (c *Client) AddRule(ctx context.Context, rule map[string]any) error {
	protoRule, err := routingRuleFromMap(cloneRuleMap(rule))
	if err != nil {
		return err
	}
	msg, err := typedMessage(protoRule)
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

func typedMessage(msg *routerconfig.RoutingRule) (*commonserial.TypedMessage, error) {
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal routing rule: %w", err)
	}
	return &commonserial.TypedMessage{
		Type:  string(msg.ProtoReflect().Descriptor().FullName()),
		Value: data,
	}, nil
}
