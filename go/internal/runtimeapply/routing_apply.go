package runtimeapply

import (
	"context"
	"errors"
	"fmt"
)

type RoutingApplier interface {
	AddRule(context.Context, map[string]any) error
	RemoveRule(context.Context, string) error
}

func ApplyRoutingDiff(ctx context.Context, applier RoutingApplier, diff Diff) error {
	if diff.Kind != DiffRoutingOnly {
		return fmt.Errorf("routing runtime apply requires routing-only diff, got %s", diff.Kind)
	}
	if applier == nil {
		return errors.New("routing applier is nil")
	}

	removed := make([]map[string]any, 0, len(diff.RemovedRules))
	for _, change := range diff.RemovedRules {
		if err := applier.RemoveRule(ctx, change.RuleTag); err != nil {
			rollbackErr := rollbackRemoved(ctx, applier, removed)
			if rollbackErr != nil {
				return fmt.Errorf("remove routing rule %s: %w; rollback: %v", change.RuleTag, err, rollbackErr)
			}
			return fmt.Errorf("remove routing rule %s: %w", change.RuleTag, err)
		}
		removed = append(removed, change.Rule)
	}

	added := make([]map[string]any, 0, len(diff.AddedRules))
	for _, change := range diff.AddedRules {
		if err := applier.AddRule(ctx, change.Rule); err != nil {
			rollbackErr := errors.Join(rollbackAdded(ctx, applier, added), rollbackRemoved(ctx, applier, removed))
			if rollbackErr != nil {
				return fmt.Errorf("add routing rule %s: %w; rollback: %v", change.RuleTag, err, rollbackErr)
			}
			return fmt.Errorf("add routing rule %s: %w", change.RuleTag, err)
		}
		added = append(added, change.Rule)
	}
	return nil
}

func rollbackAdded(ctx context.Context, applier RoutingApplier, added []map[string]any) error {
	var result error
	for i := len(added) - 1; i >= 0; i-- {
		tag, _ := added[i]["ruleTag"].(string)
		if tag == "" {
			continue
		}
		if err := applier.RemoveRule(ctx, tag); err != nil {
			result = errors.Join(result, err)
		}
	}
	return result
}

func rollbackRemoved(ctx context.Context, applier RoutingApplier, removed []map[string]any) error {
	var result error
	for i := len(removed) - 1; i >= 0; i-- {
		if err := applier.AddRule(ctx, removed[i]); err != nil {
			result = errors.Join(result, err)
		}
	}
	return result
}
