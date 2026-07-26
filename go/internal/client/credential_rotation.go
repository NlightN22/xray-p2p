package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
	ownedhttp "github.com/NlightN22/xray-p2p/go/internal/nethttp"
)

func fetchRotation(ctx context.Context, client ownedhttp.Doer, endpoint clientEndpointRecord, port int, credential string) (controlplane.RotationResponse, error) {
	if port <= 0 {
		port = 62022
	}
	host := strings.TrimSpace(endpoint.Hostname)
	if host == "" {
		return controlplane.RotationResponse{}, fmt.Errorf("endpoint host is required")
	}
	url := "https://" + net.JoinHostPort(host, fmt.Sprintf("%d", port)) + controlplane.PathCredentialsRotate
	challenge, err := rotationRequest(ctx, client, url, controlplane.RotationRequest{UserLabel: endpoint.User, Action: "challenge"})
	if err != nil {
		return controlplane.RotationResponse{}, err
	}
	var c controlplane.RotationChallenge
	if err := json.Unmarshal(challenge, &c); err != nil {
		return controlplane.RotationResponse{}, err
	}
	proof := controlplane.RotationProof(credential, c.Nonce)
	body, err := rotationRequest(ctx, client, url, controlplane.RotationRequest{UserLabel: endpoint.User, Nonce: c.Nonce, Proof: proof})
	if err != nil {
		return controlplane.RotationResponse{}, err
	}
	var result controlplane.RotationResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return controlplane.RotationResponse{}, err
	}
	return result, nil
}

func rotationRequest(ctx context.Context, client ownedhttp.Doer, url string, payload controlplane.RotationRequest) ([]byte, error) {
	if client == nil {
		return nil, fmt.Errorf("control HTTP client is required")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	const responseLimit = 1 << 20
	var out bytes.Buffer
	if _, err := out.ReadFrom(io.LimitReader(resp.Body, responseLimit+1)); err != nil {
		return nil, err
	}
	if out.Len() > responseLimit {
		return nil, fmt.Errorf("rotation response exceeds %d bytes", responseLimit)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rotation request failed: %s", resp.Status)
	}
	return out.Bytes(), nil
}

func acknowledgeRotation(ctx context.Context, client ownedhttp.Doer, endpoint clientEndpointRecord, port int, credential string) error {
	if port <= 0 {
		port = 62022
	}
	url := "https://" + net.JoinHostPort(endpoint.Hostname, fmt.Sprintf("%d", port)) + controlplane.PathCredentialsRotate
	challenge, err := rotationRequest(ctx, client, url, controlplane.RotationRequest{UserLabel: endpoint.User, Action: "challenge"})
	if err != nil {
		return err
	}
	var c controlplane.RotationChallenge
	if err := json.Unmarshal(challenge, &c); err != nil {
		return err
	}
	ackURL := strings.TrimSuffix(url, "rotate") + "ack"
	_, err = rotationRequest(ctx, client, ackURL, controlplane.RotationRequest{UserLabel: endpoint.User, Nonce: c.Nonce, Proof: controlplane.RotationProof(credential, c.Nonce)})
	return err
}
