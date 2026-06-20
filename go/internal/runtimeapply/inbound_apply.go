package runtimeapply

import (
	"context"
	"errors"
	"fmt"
)

type InboundApplier interface {
	AddInbound(context.Context, map[string]any) error
	RemoveInbound(context.Context, string) error
	ListInboundTags(context.Context) ([]string, error)
}

func ApplyInboundDiff(ctx context.Context, applier InboundApplier, diff Diff) error {
	if diff.Kind != DiffInboundOnly {
		return fmt.Errorf("inbound runtime apply requires inbound-only diff, got %s", diff.Kind)
	}
	if applier == nil {
		return errors.New("inbound applier is nil")
	}

	removed := make([]map[string]any, 0, len(diff.RemovedInbounds))
	for _, change := range diff.RemovedInbounds {
		if err := applier.RemoveInbound(ctx, change.Tag); err != nil {
			rollbackErr := rollbackRemovedInbounds(ctx, applier, removed)
			if rollbackErr != nil {
				return fmt.Errorf("remove inbound %s: %w; rollback: %v", change.Tag, err, rollbackErr)
			}
			return fmt.Errorf("remove inbound %s: %w", change.Tag, err)
		}
		removed = append(removed, change.Inbound)
	}

	added := make([]map[string]any, 0, len(diff.AddedInbounds))
	for _, change := range diff.AddedInbounds {
		if err := applier.AddInbound(ctx, change.Inbound); err != nil {
			rollbackErr := errors.Join(rollbackAddedInbounds(ctx, applier, added), rollbackRemovedInbounds(ctx, applier, removed))
			if rollbackErr != nil {
				return fmt.Errorf("add inbound %s: %w; rollback: %v", change.Tag, err, rollbackErr)
			}
			return fmt.Errorf("add inbound %s: %w", change.Tag, err)
		}
		added = append(added, change.Inbound)
	}
	if err := verifyInboundDiff(ctx, applier, diff); err != nil {
		rollbackErr := errors.Join(rollbackAddedInbounds(ctx, applier, added), rollbackRemovedInbounds(ctx, applier, removed))
		if rollbackErr != nil {
			return fmt.Errorf("verify inbound runtime apply: %w; rollback: %v", err, rollbackErr)
		}
		return fmt.Errorf("verify inbound runtime apply: %w", err)
	}
	return nil
}

func verifyInboundDiff(ctx context.Context, applier InboundApplier, diff Diff) error {
	tags, err := applier.ListInboundTags(ctx)
	if err != nil {
		return fmt.Errorf("list inbounds: %w", err)
	}
	present := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		present[tag] = struct{}{}
	}
	for _, change := range diff.AddedInbounds {
		if _, ok := present[change.Tag]; !ok {
			return fmt.Errorf("added inbound %s not found", change.Tag)
		}
	}
	replaced := make(map[string]struct{}, len(diff.AddedInbounds))
	for _, change := range diff.AddedInbounds {
		replaced[change.Tag] = struct{}{}
	}
	for _, change := range diff.RemovedInbounds {
		if _, ok := replaced[change.Tag]; ok {
			continue
		}
		if _, ok := present[change.Tag]; ok {
			return fmt.Errorf("removed inbound %s still present", change.Tag)
		}
	}
	return nil
}

func rollbackAddedInbounds(ctx context.Context, applier InboundApplier, added []map[string]any) error {
	var result error
	for i := len(added) - 1; i >= 0; i-- {
		tag, _ := added[i]["tag"].(string)
		if tag == "" {
			continue
		}
		if err := applier.RemoveInbound(ctx, tag); err != nil {
			result = errors.Join(result, err)
		}
	}
	return result
}

func rollbackRemovedInbounds(ctx context.Context, applier InboundApplier, removed []map[string]any) error {
	var result error
	for i := len(removed) - 1; i >= 0; i-- {
		if err := applier.AddInbound(ctx, removed[i]); err != nil {
			result = errors.Join(result, err)
		}
	}
	return result
}
