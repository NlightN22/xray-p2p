package ha

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// SyncClient sends the coordinator's immutable candidate over HTTPS. The
// caller supplies the HTTP client so certificate policy remains explicit.
type SyncClient struct{ HTTPClient *http.Client }

func (c SyncClient) Prepare(ctx context.Context, peer Peer, generation Generation) (Acknowledgement, error) {
	if strings.TrimSpace(peer.Endpoint) == "" {
		return Acknowledgement{}, fmt.Errorf("HA peer %q endpoint is required", peer.ID)
	}
	signature, err := Sign(peer, generation)
	if err != nil {
		return Acknowledgement{}, err
	}
	request := PrepareRequest{PeerID: peer.ID, Generation: generation, Signature: signature}
	data, err := json.Marshal(request)
	if err != nil {
		return Acknowledgement{}, err
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	url := strings.TrimRight(peer.Endpoint, "/") + PathPrepare
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return Acknowledgement{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := httpClient.Do(req)
	if err != nil {
		return Acknowledgement{}, err
	}
	defer response.Body.Close()
	var ack Acknowledgement
	if err := json.NewDecoder(response.Body).Decode(&ack); err != nil {
		return Acknowledgement{}, err
	}
	if response.StatusCode != http.StatusOK || !ack.Ready {
		return ack, fmt.Errorf("HA peer %q rejected generation %d: %s", peer.ID, generation.Number, ack.Error)
	}
	return ack, nil
}

func (c SyncClient) Commit(ctx context.Context, peer Peer, generation Generation) (Generation, error) {
	signature, err := Sign(peer, generation)
	if err != nil {
		return Generation{}, err
	}
	data, err := json.Marshal(CommitRequest{PeerID: peer.ID, Generation: generation.Number, Signature: signature})
	if err != nil {
		return Generation{}, err
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	url := strings.TrimRight(peer.Endpoint, "/") + PathCommit
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return Generation{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := httpClient.Do(req)
	if err != nil {
		return Generation{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Generation{}, fmt.Errorf("HA peer %q commit failed: %s", peer.ID, response.Status)
	}
	var committed Generation
	if err := json.NewDecoder(response.Body).Decode(&committed); err != nil {
		return Generation{}, err
	}
	if committed.Number != generation.Number {
		return Generation{}, fmt.Errorf("HA peer %q committed unexpected generation %d", peer.ID, committed.Number)
	}
	return committed, nil
}
