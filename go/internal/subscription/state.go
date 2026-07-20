package subscription

import "time"

type PersistedSource struct {
	SourceRef
	URL             string    `json:"url" toml:"url"`
	Revision        string    `json:"revision,omitempty" toml:"revision,omitempty"`
	SelectedOfferID string    `json:"selected_offer_id,omitempty" toml:"selected_offer_id,omitempty"`
	LastGood        *Snapshot `json:"last_good,omitempty" toml:"last_good,omitempty"`
	LastRefreshAt   time.Time `json:"last_refresh_at,omitempty" toml:"last_refresh_at,omitempty"`
	LastApplyAt     time.Time `json:"last_apply_at,omitempty" toml:"last_apply_at,omitempty"`
	LastError       string    `json:"last_error,omitempty" toml:"last_error,omitempty"`
}

type ReconcileResult struct {
	Added     []ConnectionOffer
	Updated   []ConnectionOffer
	Removed   []ConnectionOffer
	Unchanged []ConnectionOffer
}

func Reconcile(previous *Snapshot, next Snapshot) ReconcileResult {
	result := ReconcileResult{}
	old := make(map[string]ConnectionOffer)
	if previous != nil && previous.Source.ID == next.Source.ID {
		for _, offer := range previous.Offers {
			old[offer.StableID] = offer
		}
	}
	for _, offer := range next.Offers {
		prior, ok := old[offer.StableID]
		if !ok {
			result.Added = append(result.Added, offer)
		} else if offersEqual(prior, offer) {
			result.Unchanged = append(result.Unchanged, offer)
		} else {
			result.Updated = append(result.Updated, offer)
		}
		delete(old, offer.StableID)
	}
	for _, offer := range old {
		result.Removed = append(result.Removed, offer)
	}
	return result
}
