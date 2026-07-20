package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/subscription"
)

type ExternalSubscriptionOptions struct {
	ID        string
	URL       string
	AllowHTTP bool
}

type ExternalSubscriptionStatus struct {
	ID              string
	Adapter         string
	Revision        string
	SelectedOfferID string
	Offers          []subscription.ConnectionOffer
	LastRefreshAt   time.Time
	LastApplyAt     time.Time
	LastError       string
}

var saveExternalSubscriptionState = func(store subscription.Store, state subscription.PersistedSource, expectedDigest string) error {
	return store.Save(state, expectedDigest)
}

func AddExternalSubscription(ctx context.Context, opts ExternalSubscriptionOptions) error {
	id := strings.TrimSpace(opts.ID)
	if !validExternalSubscriptionID(id) {
		return errors.New("subscription ID must use only letters, digits, dot, dash, or underscore")
	}
	if strings.TrimSpace(opts.URL) == "" {
		return errors.New("subscription URL is required")
	}
	state, err := loadClientInstallState(config.ConfigPath(layout.ClientConfigFileName))
	if err != nil {
		return err
	}
	for _, source := range state.Subscriptions {
		if strings.EqualFold(source.ID, id) {
			return fmt.Errorf("subscription %q already exists", id)
		}
	}
	source := externalSubscriptionSource{ID: id, Adapter: subscription.AdapterURIList3XUIV2811, CompatibilityVersion: subscription.AdapterURIList3XUIV2811, URL: strings.TrimSpace(opts.URL)}
	candidate := state
	candidate.Subscriptions = append(append([]externalSubscriptionSource(nil), state.Subscriptions...), source)
	return refreshExternalSubscription(ctx, opts, state, candidate, len(candidate.Subscriptions)-1, source, true)
}

func RefreshExternalSubscription(ctx context.Context, opts ExternalSubscriptionOptions) error {
	id := strings.TrimSpace(opts.ID)
	desired, err := loadClientInstallState(config.ConfigPath(layout.ClientConfigFileName))
	if err != nil {
		return err
	}
	source, index := findExternalSubscription(desired.Subscriptions, id)
	if index < 0 {
		return fmt.Errorf("subscription %q not found", id)
	}
	if strings.TrimSpace(opts.URL) != "" {
		source.URL = strings.TrimSpace(opts.URL)
	}
	candidate := desired
	candidate.Subscriptions = append([]externalSubscriptionSource(nil), desired.Subscriptions...)
	return refreshExternalSubscription(ctx, opts, desired, candidate, index, source, false)
}

func refreshExternalSubscription(ctx context.Context, opts ExternalSubscriptionOptions, previous, desired clientInstallState, index int, source externalSubscriptionSource, adding bool) error {
	store := externalSubscriptionStore(source.ID)
	persisted, digest, err := store.Load()
	if err != nil {
		return err
	}
	previousPersisted := persisted
	fetcher := subscription.HTTPSource{SourceRef: subscription.SourceRef{ID: source.ID, Adapter: source.Adapter}, URL: source.URL, AllowHTTP: opts.AllowHTTP}
	raw, err := fetcher.Fetch(ctx, persisted.Revision)
	if errors.Is(err, subscription.ErrNotModified) {
		if adding {
			return errors.New("initial subscription fetch returned no snapshot")
		}
		return nil
	}
	if err != nil {
		if adding {
			return err
		}
		return recordExternalSubscriptionError(store, persisted, digest, err)
	}
	snapshot, err := (subscription.URIListDecoder{}).Decode(raw)
	if err != nil {
		if adding {
			return err
		}
		return recordExternalSubscriptionError(store, persisted, digest, err)
	}
	now := time.Now().UTC()
	persisted.SourceRef = snapshot.Source
	persisted.URL = source.URL
	persisted.Revision = snapshot.Revision
	persisted.LastGood = &snapshot
	persisted.LastRefreshAt = now
	persisted.LastApplyAt = now
	persisted.LastError = ""
	desired.Subscriptions[index] = source
	desired.Endpoints = replaceSubscriptionEndpoints(desired.Endpoints, source.ID, snapshot.Offers)
	commit := func() error {
		if err := desired.save(config.ConfigPath(layout.ClientConfigFileName)); err != nil {
			return err
		}
		if err := saveExternalSubscriptionState(store, persisted, digest); err != nil {
			if restoreErr := previous.save(config.ConfigPath(layout.ClientConfigFileName)); restoreErr != nil {
				return fmt.Errorf("persist subscription LKG: %w; restore Desired: %v", err, restoreErr)
			}
			return fmt.Errorf("persist subscription LKG: %w", err)
		}
		return nil
	}
	if _, err := commitClientSubscriptionStateTransaction(ctx, desired, commit); err != nil {
		if adding {
			return err
		}
		return recordExternalSubscriptionError(store, previousPersisted, digest, err)
	}
	return nil
}

func ListExternalSubscriptions() ([]ExternalSubscriptionStatus, error) {
	desired, err := loadClientInstallState(config.ConfigPath(layout.ClientConfigFileName))
	if err != nil {
		return nil, err
	}
	result := make([]ExternalSubscriptionStatus, 0, len(desired.Subscriptions))
	for _, source := range desired.Subscriptions {
		persisted, _, err := externalSubscriptionStore(source.ID).Load()
		if err != nil {
			return nil, err
		}
		status := ExternalSubscriptionStatus{ID: source.ID, Adapter: source.Adapter, Revision: persisted.Revision, SelectedOfferID: source.SelectedOfferID, LastRefreshAt: persisted.LastRefreshAt, LastApplyAt: persisted.LastApplyAt, LastError: persisted.LastError}
		if persisted.LastGood != nil {
			status.Offers = append([]subscription.ConnectionOffer(nil), persisted.LastGood.Offers...)
		}
		result = append(result, status)
	}
	return result, nil
}

func RemoveExternalSubscription(ctx context.Context, id string) error {
	desired, err := loadClientInstallState(config.ConfigPath(layout.ClientConfigFileName))
	if err != nil {
		return err
	}
	_, index := findExternalSubscription(desired.Subscriptions, id)
	if index < 0 {
		return fmt.Errorf("subscription %q not found", strings.TrimSpace(id))
	}
	sourceID := desired.Subscriptions[index].ID
	desired.Subscriptions = append(desired.Subscriptions[:index], desired.Subscriptions[index+1:]...)
	desired.Endpoints = replaceSubscriptionEndpoints(desired.Endpoints, sourceID, nil)
	if err := commitClientRuntimeState(ctx, desired); err != nil {
		return err
	}
	if err := os.Remove(externalSubscriptionStore(sourceID).Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove subscription state: %w", err)
	}
	return nil
}

func findExternalSubscription(sources []externalSubscriptionSource, id string) (externalSubscriptionSource, int) {
	for index, source := range sources {
		if strings.EqualFold(strings.TrimSpace(source.ID), strings.TrimSpace(id)) {
			return source, index
		}
	}
	return externalSubscriptionSource{}, -1
}

func externalSubscriptionStore(id string) subscription.Store {
	name := strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(strings.TrimSpace(id))
	return subscription.Store{Path: filepath.Join(config.StateRoot(), "subscriptions", name+".json")}
}

func recordExternalSubscriptionError(store subscription.Store, state subscription.PersistedSource, digest string, cause error) error {
	state.LastError = cause.Error()
	if saveErr := saveExternalSubscriptionState(store, state, digest); saveErr != nil {
		return fmt.Errorf("%w; persist refresh diagnostic: %v", cause, saveErr)
	}
	return cause
}

func replaceSubscriptionEndpoints(current []clientEndpointRecord, sourceID string, offers []subscription.ConnectionOffer) []clientEndpointRecord {
	result := make([]clientEndpointRecord, 0, len(current)+len(offers))
	for _, endpoint := range current {
		if endpoint.SubscriptionSourceID != sourceID {
			result = append(result, endpoint)
		}
	}
	for _, offer := range offers {
		endpoint := offer.Endpoint
		result = append(result, clientEndpointRecord{
			SubscriptionSourceID: sourceID, SubscriptionOfferID: offer.StableID,
			Profile: string(endpoint.Profile), Protocol: endpoint.Protocol, Transport: endpoint.Transport, Security: endpoint.Security,
			Flow: endpoint.Metadata["flow"], Hostname: endpoint.Host, Address: endpoint.Host, Port: endpoint.Port,
			User: offer.UserLabel, Password: offer.Credential, ServerName: endpoint.ServerName, ALPN: endpoint.TLS.ALPN,
			AllowInsecure: endpoint.TLS.AllowInsecure, PinnedPeerCertSHA256: endpoint.TLS.PinnedPeerCertSHA256,
			VerifyPeerCertByName: endpoint.TLS.VerifyPeerCertByName, Tag: externalOfferTag(offer.StableID),
		})
	}
	return result
}

func externalOfferTag(stableID string) string {
	value := strings.TrimPrefix(strings.TrimSpace(stableID), "offer-")
	if len(value) > 16 {
		value = value[:16]
	}
	return "subscription-" + value
}

func validExternalSubscriptionID(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '.' || char == '-' || char == '_' {
			continue
		}
		return false
	}
	return true
}
