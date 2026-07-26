package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
)

func TestControlHTTPRotationPendingAndSubscriptionOK(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case controlplane.PathCredentialsRotate:
			body, _ := io.ReadAll(request.Body)
			if strings.Contains(string(body), `"action":"challenge"`) {
				_, _ = io.WriteString(w, `{"nonce":"pending-nonce"}`)
				return
			}
			_, _ = io.WriteString(w, `{"rotation_pending":true,"active_credential":"rotated"}`)
		case controlplane.PathSubscription:
			_, _ = io.WriteString(w, `{"generation":"generation-2","profile":"trojan-tls","protocol":"trojan","transport":"tcp","security":"tls","host":"edge.example","port":443}`)
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()

	endpoint, port := lifecycleEndpoint(t, server.URL)
	client := controlHTTPClient(endpoint, time.Second)
	defer shutdownControlClient(t, client)
	rotation, err := fetchRotation(t.Context(), client, endpoint, port, "credential")
	if err != nil {
		t.Fatal(err)
	}
	if !rotation.RotationPending || rotation.ActiveCredential != "rotated" {
		t.Fatalf("unexpected rotation response: %+v", rotation)
	}
	subscription, err := fetchSubscriptionConditional(t.Context(), client, endpoint, port, rotation.ActiveCredential, "generation-1")
	if err != nil {
		t.Fatal(err)
	}
	if subscription.Generation != "generation-2" || subscription.Host != "edge.example" {
		t.Fatalf("unexpected subscription: %+v", subscription)
	}
}

func TestControlHTTPStatusErrors(t *testing.T) {
	for _, path := range []string{controlplane.PathCredentialsRotate, controlplane.PathSubscription} {
		for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusInternalServerError} {
			t.Run(path+"/"+http.StatusText(status), func(t *testing.T) {
				server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					http.Error(w, http.StatusText(status), status)
				}))
				defer server.Close()
				endpoint, port := lifecycleEndpoint(t, server.URL)
				client := controlHTTPClient(endpoint, time.Second)
				defer shutdownControlClient(t, client)
				var err error
				if path == controlplane.PathCredentialsRotate {
					_, err = fetchRotation(t.Context(), client, endpoint, port, "credential")
				} else {
					_, err = fetchSubscription(t.Context(), client, endpoint, port, "credential")
				}
				if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("%d", status)) {
					t.Fatalf("status %d error = %v", status, err)
				}
			})
		}
	}
}

func TestControlHTTPRejectsMalformedJSON(t *testing.T) {
	for _, path := range []string{controlplane.PathCredentialsRotate, controlplane.PathSubscription} {
		t.Run(path, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, `{broken`)
			}))
			defer server.Close()
			endpoint, port := lifecycleEndpoint(t, server.URL)
			client := controlHTTPClient(endpoint, time.Second)
			defer shutdownControlClient(t, client)
			var err error
			if path == controlplane.PathCredentialsRotate {
				_, err = fetchRotation(t.Context(), client, endpoint, port, "credential")
			} else {
				_, err = fetchSubscription(t.Context(), client, endpoint, port, "credential")
			}
			if err == nil {
				t.Fatal("malformed JSON was accepted")
			}
		})
	}
}

func TestControlHTTPTimeoutAndCancellation(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		<-request.Context().Done()
	}))
	defer server.Close()
	endpoint, port := lifecycleEndpoint(t, server.URL)

	timeoutClient := controlHTTPClient(endpoint, 30*time.Millisecond)
	if _, err := fetchSubscription(t.Context(), timeoutClient, endpoint, port, "credential"); err == nil {
		t.Fatal("timed out subscription request succeeded")
	}
	shutdownControlClient(t, timeoutClient)

	client := controlHTTPClient(endpoint, time.Second)
	defer shutdownControlClient(t, client)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := fetchSubscription(ctx, client, endpoint, port, "credential"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled request error = %v, want %v", err, context.Canceled)
	}
}

func TestControlHTTPRecoversAfterNetworkFailure(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	endpoint, port := lifecycleEndpoint(t, server.URL)
	dialer := &failOnceDialer{}
	client := newControlHTTPClient(endpoint, time.Second, dialer)
	defer shutdownControlClient(t, client)

	if _, err := fetchSubscription(t.Context(), client, endpoint, port, "credential"); err == nil {
		t.Fatal("injected network failure succeeded")
	}
	if _, err := fetchSubscription(t.Context(), client, endpoint, port, "credential"); err != nil {
		t.Fatalf("request did not recover: %v", err)
	}
}

type failOnceDialer struct {
	failed atomic.Bool
	dialer net.Dialer
}

func (d *failOnceDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if d.failed.CompareAndSwap(false, true) {
		return nil, errors.New("injected dial failure")
	}
	return d.dialer.DialContext(ctx, network, address)
}

func (*failOnceDialer) Shutdown(context.Context) error {
	return nil
}

func shutdownControlClient(t *testing.T, client interface {
	Shutdown(context.Context) error
}) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := client.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
}
