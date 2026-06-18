package runtimeapply

import (
	"context"
	"errors"
	"fmt"
)

type OutboundApplier interface {
	AddOutbound(context.Context, map[string]any) error
	RemoveOutbound(context.Context, string) error
	ListOutboundTags(context.Context) ([]string, error)
}

func ApplyOutboundDiff(ctx context.Context, applier OutboundApplier, diff Diff) error {
	if diff.Kind != DiffOutboundOnly {
		return fmt.Errorf("outbound runtime apply requires outbound-only diff, got %s", diff.Kind)
	}
	if applier == nil {
		return errors.New("outbound applier is nil")
	}

	removed := make([]map[string]any, 0, len(diff.RemovedOutbounds))
	for _, change := range diff.RemovedOutbounds {
		if err := applier.RemoveOutbound(ctx, change.Tag); err != nil {
			rollbackErr := rollbackRemovedOutbounds(ctx, applier, removed)
			if rollbackErr != nil {
				return fmt.Errorf("remove outbound %s: %w; rollback: %v", change.Tag, err, rollbackErr)
			}
			return fmt.Errorf("remove outbound %s: %w", change.Tag, err)
		}
		removed = append(removed, change.Outbound)
	}

	added := make([]map[string]any, 0, len(diff.AddedOutbounds))
	for _, change := range diff.AddedOutbounds {
		if err := applier.AddOutbound(ctx, change.Outbound); err != nil {
			rollbackErr := errors.Join(rollbackAddedOutbounds(ctx, applier, added), rollbackRemovedOutbounds(ctx, applier, removed))
			if rollbackErr != nil {
				return fmt.Errorf("add outbound %s: %w; rollback: %v", change.Tag, err, rollbackErr)
			}
			return fmt.Errorf("add outbound %s: %w", change.Tag, err)
		}
		added = append(added, change.Outbound)
	}
	if err := verifyOutboundDiff(ctx, applier, diff); err != nil {
		rollbackErr := errors.Join(rollbackAddedOutbounds(ctx, applier, added), rollbackRemovedOutbounds(ctx, applier, removed))
		if rollbackErr != nil {
			return fmt.Errorf("verify outbound runtime apply: %w; rollback: %v", err, rollbackErr)
		}
		return fmt.Errorf("verify outbound runtime apply: %w", err)
	}
	return nil
}

func verifyOutboundDiff(ctx context.Context, applier OutboundApplier, diff Diff) error {
	tags, err := applier.ListOutboundTags(ctx)
	if err != nil {
		return fmt.Errorf("list outbounds: %w", err)
	}
	present := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		present[tag] = struct{}{}
	}
	for _, change := range diff.AddedOutbounds {
		if _, ok := present[change.Tag]; !ok {
			return fmt.Errorf("added outbound %s not found", change.Tag)
		}
	}
	for _, change := range diff.RemovedOutbounds {
		if _, ok := present[change.Tag]; ok {
			return fmt.Errorf("removed outbound %s still present", change.Tag)
		}
	}
	return nil
}

func rollbackAddedOutbounds(ctx context.Context, applier OutboundApplier, added []map[string]any) error {
	var result error
	for i := len(added) - 1; i >= 0; i-- {
		tag, _ := added[i]["tag"].(string)
		if tag == "" {
			continue
		}
		if err := applier.RemoveOutbound(ctx, tag); err != nil {
			result = errors.Join(result, err)
		}
	}
	return result
}

func rollbackRemovedOutbounds(ctx context.Context, applier OutboundApplier, removed []map[string]any) error {
	var result error
	for i := len(removed) - 1; i >= 0; i-- {
		if err := applier.AddOutbound(ctx, removed[i]); err != nil {
			result = errors.Join(result, err)
		}
	}
	return result
}
