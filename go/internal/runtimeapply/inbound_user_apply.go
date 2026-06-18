package runtimeapply

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type InboundUserApplier interface {
	AddInboundUser(ctx context.Context, inboundTag, email, password string) error
	RemoveInboundUser(ctx context.Context, inboundTag, email string) error
	ListInboundUserEmails(ctx context.Context, inboundTag string) ([]string, error)
}

func ApplyInboundUserDiff(ctx context.Context, applier InboundUserApplier, diff Diff) error {
	if diff.Kind != DiffInboundUsers {
		return fmt.Errorf("inbound user runtime apply requires inbound-user diff, got %s", diff.Kind)
	}
	if applier == nil {
		return errors.New("inbound user applier is nil")
	}

	removed := make([]InboundUserChange, 0, len(diff.RemovedInboundUsers))
	for _, change := range diff.RemovedInboundUsers {
		if err := applier.RemoveInboundUser(ctx, change.InboundTag, change.Email); err != nil {
			rollbackErr := rollbackRemovedInboundUsers(ctx, applier, removed)
			if rollbackErr != nil {
				return fmt.Errorf("remove inbound user %s/%s: %w; rollback: %v", change.InboundTag, change.Email, err, rollbackErr)
			}
			return fmt.Errorf("remove inbound user %s/%s: %w", change.InboundTag, change.Email, err)
		}
		removed = append(removed, change)
	}

	added := make([]InboundUserChange, 0, len(diff.AddedInboundUsers))
	for _, change := range diff.AddedInboundUsers {
		if err := applier.AddInboundUser(ctx, change.InboundTag, change.Email, change.Password); err != nil {
			rollbackErr := errors.Join(rollbackAddedInboundUsers(ctx, applier, added), rollbackRemovedInboundUsers(ctx, applier, removed))
			if rollbackErr != nil {
				return fmt.Errorf("add inbound user %s/%s: %w; rollback: %v", change.InboundTag, change.Email, err, rollbackErr)
			}
			return fmt.Errorf("add inbound user %s/%s: %w", change.InboundTag, change.Email, err)
		}
		added = append(added, change)
	}
	if err := verifyInboundUserDiff(ctx, applier, diff); err != nil {
		rollbackErr := errors.Join(rollbackAddedInboundUsers(ctx, applier, added), rollbackRemovedInboundUsers(ctx, applier, removed))
		if rollbackErr != nil {
			return fmt.Errorf("verify inbound user runtime apply: %w; rollback: %v", err, rollbackErr)
		}
		return fmt.Errorf("verify inbound user runtime apply: %w", err)
	}
	return nil
}

func verifyInboundUserDiff(ctx context.Context, applier InboundUserApplier, diff Diff) error {
	tags := make(map[string]struct{})
	for _, change := range diff.AddedInboundUsers {
		tags[change.InboundTag] = struct{}{}
	}
	for _, change := range diff.RemovedInboundUsers {
		tags[change.InboundTag] = struct{}{}
	}
	for tag := range tags {
		emails, err := applier.ListInboundUserEmails(ctx, tag)
		if err != nil {
			return fmt.Errorf("list inbound users %s: %w", tag, err)
		}
		present := emailSet(emails)
		for _, change := range diff.AddedInboundUsers {
			if change.InboundTag == tag {
				if _, ok := present[strings.ToLower(change.Email)]; !ok {
					return fmt.Errorf("added inbound user %s/%s not found", tag, change.Email)
				}
			}
		}
		for _, change := range diff.RemovedInboundUsers {
			if change.InboundTag == tag {
				if _, ok := present[strings.ToLower(change.Email)]; ok {
					return fmt.Errorf("removed inbound user %s/%s still present", tag, change.Email)
				}
			}
		}
	}
	return nil
}

func rollbackAddedInboundUsers(ctx context.Context, applier InboundUserApplier, added []InboundUserChange) error {
	var result error
	for i := len(added) - 1; i >= 0; i-- {
		if err := applier.RemoveInboundUser(ctx, added[i].InboundTag, added[i].Email); err != nil {
			result = errors.Join(result, err)
		}
	}
	return result
}

func rollbackRemovedInboundUsers(ctx context.Context, applier InboundUserApplier, removed []InboundUserChange) error {
	var result error
	for i := len(removed) - 1; i >= 0; i-- {
		if err := applier.AddInboundUser(ctx, removed[i].InboundTag, removed[i].Email, removed[i].Password); err != nil {
			result = errors.Join(result, err)
		}
	}
	return result
}

func emailSet(emails []string) map[string]struct{} {
	result := make(map[string]struct{}, len(emails))
	for _, email := range emails {
		email = strings.ToLower(strings.TrimSpace(email))
		if email != "" {
			result[email] = struct{}{}
		}
	}
	return result
}
