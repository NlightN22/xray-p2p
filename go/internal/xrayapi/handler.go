package xrayapi

import (
	"context"
	"fmt"
	"time"

	coreconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/coreconfig"
	handlercommand "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/handlercommand"
)

type HandlerOptions struct {
	Address string
	Timeout time.Duration
	Dialer  Dialer
}

func AddInbound(ctx context.Context, opts HandlerOptions, inbound *coreconfig.InboundHandlerConfig) error {
	client, err := DialWith(ctx, opts.Address, opts.Timeout, opts.Dialer)
	if err != nil {
		return fmt.Errorf("connect xray API: %w", err)
	}
	defer client.Close()
	return client.AddInbound(ctx, inbound)
}

func RemoveInbound(ctx context.Context, opts HandlerOptions, tag string) error {
	client, err := DialWith(ctx, opts.Address, opts.Timeout, opts.Dialer)
	if err != nil {
		return fmt.Errorf("connect xray API: %w", err)
	}
	defer client.Close()
	return client.RemoveInbound(ctx, tag)
}

func ListInboundTags(ctx context.Context, opts HandlerOptions) ([]string, error) {
	client, err := DialWith(ctx, opts.Address, opts.Timeout, opts.Dialer)
	if err != nil {
		return nil, fmt.Errorf("connect xray API: %w", err)
	}
	defer client.Close()
	return client.ListInboundTags(ctx)
}

func AddOutbound(ctx context.Context, opts HandlerOptions, outbound *coreconfig.OutboundHandlerConfig) error {
	client, err := DialWith(ctx, opts.Address, opts.Timeout, opts.Dialer)
	if err != nil {
		return fmt.Errorf("connect xray API: %w", err)
	}
	defer client.Close()
	return client.AddOutbound(ctx, outbound)
}

func RemoveOutbound(ctx context.Context, opts HandlerOptions, tag string) error {
	client, err := DialWith(ctx, opts.Address, opts.Timeout, opts.Dialer)
	if err != nil {
		return fmt.Errorf("connect xray API: %w", err)
	}
	defer client.Close()
	return client.RemoveOutbound(ctx, tag)
}

func ListOutboundTags(ctx context.Context, opts HandlerOptions) ([]string, error) {
	client, err := DialWith(ctx, opts.Address, opts.Timeout, opts.Dialer)
	if err != nil {
		return nil, fmt.Errorf("connect xray API: %w", err)
	}
	defer client.Close()
	return client.ListOutboundTags(ctx)
}

func (c *Client) AddInbound(ctx context.Context, inbound *coreconfig.InboundHandlerConfig) error {
	_, err := c.handler.AddInbound(ctx, &handlercommand.AddInboundRequest{Inbound: inbound})
	if err != nil {
		return fmt.Errorf("add inbound: %w", err)
	}
	return nil
}

func (c *Client) RemoveInbound(ctx context.Context, tag string) error {
	_, err := c.handler.RemoveInbound(ctx, &handlercommand.RemoveInboundRequest{Tag: tag})
	if err != nil {
		return fmt.Errorf("remove inbound: %w", err)
	}
	return nil
}

func (c *Client) ListInboundTags(ctx context.Context) ([]string, error) {
	response, err := c.handler.ListInbounds(ctx, &handlercommand.ListInboundsRequest{IsOnlyTags: true})
	if err != nil {
		return nil, fmt.Errorf("list inbounds: %w", err)
	}
	return inboundTags(response.GetInbounds()), nil
}

func (c *Client) AddOutbound(ctx context.Context, outbound *coreconfig.OutboundHandlerConfig) error {
	_, err := c.handler.AddOutbound(ctx, &handlercommand.AddOutboundRequest{Outbound: outbound})
	if err != nil {
		return fmt.Errorf("add outbound: %w", err)
	}
	return nil
}

func (c *Client) RemoveOutbound(ctx context.Context, tag string) error {
	_, err := c.handler.RemoveOutbound(ctx, &handlercommand.RemoveOutboundRequest{Tag: tag})
	if err != nil {
		return fmt.Errorf("remove outbound: %w", err)
	}
	return nil
}

func (c *Client) ListOutboundTags(ctx context.Context) ([]string, error) {
	response, err := c.handler.ListOutbounds(ctx, &handlercommand.ListOutboundsRequest{})
	if err != nil {
		return nil, fmt.Errorf("list outbounds: %w", err)
	}
	return outboundTags(response.GetOutbounds()), nil
}

func inboundTags(items []*coreconfig.InboundHandlerConfig) []string {
	tags := make([]string, 0, len(items))
	for _, item := range items {
		if item != nil && item.GetTag() != "" {
			tags = append(tags, item.GetTag())
		}
	}
	return tags
}

func outboundTags(items []*coreconfig.OutboundHandlerConfig) []string {
	tags := make([]string, 0, len(items))
	for _, item := range items {
		if item != nil && item.GetTag() != "" {
			tags = append(tags, item.GetTag())
		}
	}
	return tags
}
