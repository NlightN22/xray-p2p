package server

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/identitysync"
)

func applyHAIdentityState(identityPayload, provisionedPayload []byte) error {
	if len(identityPayload) == 0 {
		return nil
	}
	var generation identitysync.Generation
	if err := json.Unmarshal(identityPayload, &generation); err != nil {
		return fmt.Errorf("parse HA identity generation: %w", err)
	}
	if generation.ID == "" {
		return fmt.Errorf("HA identity generation ID is required")
	}
	if generation.Subjects == nil {
		generation.Subjects = make(map[string]identitysync.Subject)
	}
	if generation.Groups == nil {
		generation.Groups = make(map[string]identitysync.Group)
	}
	if err := applyHAProvisionedOverlay(&generation, provisionedPayload); err != nil {
		return err
	}
	store := identitysync.DefaultStore()
	state, err := store.Load()
	if err != nil {
		return err
	}
	state.Current = &generation
	state.Pending = nil
	state.Transaction = nil
	state.Status.State = identitysync.SyncStatusSuccess
	state.Status.Error = ""
	return store.Save(state)
}

func applyHAProvisionedOverlay(generation *identitysync.Generation, payload []byte) error {
	if len(payload) == 0 {
		return nil
	}
	var labels []string
	if err := json.Unmarshal(payload, &labels); err == nil {
		markHAProvisionedLabels(generation, labels)
		return nil
	}
	var byLabel map[string]bool
	if err := json.Unmarshal(payload, &byLabel); err != nil {
		return fmt.Errorf("parse HA provisioned resources: %w", err)
	}
	for id, subject := range generation.Subjects {
		if provisioned, ok := byLabel[subject.UserLabel]; ok {
			subject.Provisioned = provisioned
			generation.Subjects[id] = subject
		}
	}
	return nil
}

func markHAProvisionedLabels(generation *identitysync.Generation, labels []string) {
	allowed := make(map[string]struct{}, len(labels))
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label != "" {
			allowed[strings.ToLower(label)] = struct{}{}
		}
	}
	for id, subject := range generation.Subjects {
		_, subject.Provisioned = allowed[strings.ToLower(subject.UserLabel)]
		generation.Subjects[id] = subject
	}
}
