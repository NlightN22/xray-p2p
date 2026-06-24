package ha

import (
	"context"
	"fmt"
	"strings"
)

// Coordinator applies one ordered generation. A failure leaves its local
// committed generation unchanged; peers retain their previous committed state
// until the authenticated commit request arrives.
type Coordinator struct{ Client SyncClient }

func (c Coordinator) Sync(ctx context.Context, store *Store, generation Generation) error {
	if store == nil {
		return fmt.Errorf("HA store is required")
	}
	if err := store.Stage(generation); err != nil {
		return err
	}
	defer store.Abort(generation.Number)
	client := c.Client
	if client.LocalPeerID == "" {
		client.LocalPeerID = store.LocalPeerID()
	}
	peers := store.Peers()
	if len(peers) > 0 && client.LocalPeerID == "" {
		return fmt.Errorf("HA local peer identity is required")
	}
	var prepareErrors []string
	for _, peer := range peers {
		ack, err := client.Prepare(ctx, peer, generation)
		if err != nil {
			prepareErrors = append(prepareErrors, err.Error())
			continue
		}
		if err := store.Acknowledge(ack); err != nil {
			return err
		}
	}
	if !store.RecoveryState().Quorum.Available {
		if len(prepareErrors) > 0 {
			return fmt.Errorf("%w for generation %d: %s", ErrQuorumUnavailable, generation.Number, strings.Join(prepareErrors, "; "))
		}
		return fmt.Errorf("%w for generation %d", ErrQuorumUnavailable, generation.Number)
	}
	for _, peer := range peers {
		_, _ = client.Commit(ctx, peer, generation)
	}
	if _, err := store.Commit(); err != nil {
		return err
	}
	return nil
}
