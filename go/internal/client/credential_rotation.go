package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
)

func fetchRotation(ctx context.Context, endpoint clientEndpointRecord, port int, credential string, timeout time.Duration) (controlplane.RotationResponse, error) {
	if port <= 0 {
		port = 62022
	}
	host := strings.TrimSpace(endpoint.Hostname)
	if host == "" {
		return controlplane.RotationResponse{}, fmt.Errorf("endpoint host is required")
	}
	url := "https://" + net.JoinHostPort(host, fmt.Sprintf("%d", port)) + controlplane.PathCredentialsRotate
	challenge, err := rotationRequest(ctx, endpoint, url, controlplane.RotationRequest{UserLabel: endpoint.User, Action: "challenge"}, timeout)
	if err != nil {
		return controlplane.RotationResponse{}, err
	}
	var c controlplane.RotationChallenge
	if err := json.Unmarshal(challenge, &c); err != nil {
		return controlplane.RotationResponse{}, err
	}
	proof := controlplane.RotationProof(credential, c.Nonce)
	body, err := rotationRequest(ctx, endpoint, url, controlplane.RotationRequest{UserLabel: endpoint.User, Nonce: c.Nonce, Proof: proof}, timeout)
	if err != nil {
		return controlplane.RotationResponse{}, err
	}
	var result controlplane.RotationResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return controlplane.RotationResponse{}, err
	}
	return result, nil
}

func rotationRequest(ctx context.Context, endpoint clientEndpointRecord, url string, payload controlplane.RotationRequest, timeout time.Duration) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := controlHTTPClient(endpoint, timeout).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out bytes.Buffer
	if _, err := out.ReadFrom(resp.Body); err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rotation request failed: %s", resp.Status)
	}
	return out.Bytes(), nil
}

func acknowledgeRotation(ctx context.Context, endpoint clientEndpointRecord, port int, credential string, timeout time.Duration) error {
	if port <= 0 {
		port = 62022
	}
	url := "https://" + net.JoinHostPort(endpoint.Hostname, fmt.Sprintf("%d", port)) + controlplane.PathCredentialsRotate
	challenge, err := rotationRequest(ctx, endpoint, url, controlplane.RotationRequest{UserLabel: endpoint.User, Action: "challenge"}, timeout)
	if err != nil {
		return err
	}
	var c controlplane.RotationChallenge
	if err := json.Unmarshal(challenge, &c); err != nil {
		return err
	}
	ackURL := strings.TrimSuffix(url, "rotate") + "ack"
	_, err = rotationRequest(ctx, endpoint, ackURL, controlplane.RotationRequest{UserLabel: endpoint.User, Nonce: c.Nonce, Proof: controlplane.RotationProof(credential, c.Nonce)}, timeout)
	return err
}
