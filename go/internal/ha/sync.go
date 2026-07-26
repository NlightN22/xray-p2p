package ha

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	ownedhttp "github.com/NlightN22/xray-p2p/go/internal/nethttp"
)

const maxSyncResponseBytes = 1 << 20

// SyncClient sends the coordinator's immutable candidate over HTTPS. The
// caller supplies the HTTP client so certificate policy remains explicit.
type SyncClient struct {
	HTTPClient        ownedhttp.Doer
	HTTPClientForPeer func(Peer) ownedhttp.Doer
	LocalPeerID       string
}

func (c SyncClient) httpClient(peer Peer) ownedhttp.Doer {
	if c.HTTPClientForPeer != nil {
		if client := c.HTTPClientForPeer(peer); client != nil {
			return client
		}
	}
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return nil
}

func (c SyncClient) Prepare(ctx context.Context, peer Peer, generation Generation) (Acknowledgement, error) {
	if strings.TrimSpace(peer.Endpoint) == "" {
		return Acknowledgement{}, fmt.Errorf("HA peer %q endpoint is required", peer.ID)
	}
	senderID := c.senderID(peer)
	signature, err := SignAs(senderID, peer.Secret, generation)
	if err != nil {
		return Acknowledgement{}, err
	}
	request := PrepareRequest{PeerID: senderID, Generation: generation, Signature: signature}
	data, err := json.Marshal(request)
	if err != nil {
		return Acknowledgement{}, err
	}
	httpClient := c.httpClient(peer)
	if httpClient == nil {
		return Acknowledgement{}, errors.New("HA HTTP client is required")
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
	defer ownedhttp.DrainAndClose(response, maxSyncResponseBytes)
	var ack Acknowledgement
	if err := decodeSyncResponse(response.Body, &ack); err != nil {
		return Acknowledgement{}, err
	}
	if response.StatusCode != http.StatusOK || !ack.Ready {
		return ack, fmt.Errorf("HA peer %q rejected generation %d: %s", peer.ID, generation.Number, ack.Error)
	}
	return ack, nil
}

func (c SyncClient) Commit(ctx context.Context, peer Peer, generation Generation) (Generation, error) {
	senderID := c.senderID(peer)
	signature, err := SignAs(senderID, peer.Secret, generation)
	if err != nil {
		return Generation{}, err
	}
	data, err := json.Marshal(CommitRequest{PeerID: senderID, Generation: generation.Number, Signature: signature})
	if err != nil {
		return Generation{}, err
	}
	httpClient := c.httpClient(peer)
	if httpClient == nil {
		return Generation{}, errors.New("HA HTTP client is required")
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
	defer ownedhttp.DrainAndClose(response, maxSyncResponseBytes)
	if response.StatusCode != http.StatusOK {
		return Generation{}, fmt.Errorf("HA peer %q commit failed: %s", peer.ID, response.Status)
	}
	var committed Generation
	if err := decodeSyncResponse(response.Body, &committed); err != nil {
		return Generation{}, err
	}
	if committed.Number != generation.Number {
		return Generation{}, fmt.Errorf("HA peer %q committed unexpected generation %d", peer.ID, committed.Number)
	}
	return committed, nil
}

func decodeSyncResponse(body io.Reader, target any) error {
	data, err := io.ReadAll(io.LimitReader(body, maxSyncResponseBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxSyncResponseBytes {
		return fmt.Errorf("HA response exceeds %d bytes", maxSyncResponseBytes)
	}
	return json.Unmarshal(data, target)
}

func (c SyncClient) senderID(peer Peer) string {
	if id := strings.TrimSpace(c.LocalPeerID); id != "" {
		return id
	}
	return peer.ID
}
