package controlplane

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/ha"
	xnethttp "github.com/NlightN22/xray-p2p/go/internal/nethttp"
)

func TestControlOperationErrorsRemainObservableAfterTransportTimeout(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	runtime := Runtime{
		RotationUsers: []RotationUser{{
			UserLabel:                     "alice",
			ActiveCredential:              "new",
			PreviousCredentialForRotation: "old",
			RotationExpiresAt:             now.Add(time.Hour),
			CredentialGeneration:          2,
		}},
	}
	store, err := ha.NewStore(nil, ha.Generation{})
	if err != nil {
		t.Fatal(err)
	}
	ackErr := errors.New("persist acknowledged credential: disk unavailable")
	reloadErr := errors.New("reload HA state: invalid generation")
	ackStarted := make(chan struct{})
	ackRelease := make(chan struct{})
	reloadStarted := make(chan struct{})
	reloadRelease := make(chan struct{})
	reported := make(chan operationFailure, 2)
	handler := NewHandler(HandlerOptions{
		LoadRuntime: func() (Runtime, error) { return runtime, nil },
		Now:         func() time.Time { return now },
		Acknowledge: func(string, int) error {
			close(ackStarted)
			<-ackRelease
			return ackErr
		},
		HAStore: store,
		ReloadHA: func(*ha.Store) error {
			close(reloadStarted)
			<-reloadRelease
			return reloadErr
		},
		ReportError: func(operation string, err error) {
			reported <- operationFailure{operation: operation, err: err}
		},
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := xnethttp.NewServer(handler, xnethttp.ServerOptions{WriteTimeout: 40 * time.Millisecond})
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Close()
		<-serveDone
	})

	challenge := requestRotationChallenge(t, listener.Addr().String())
	body, err := json.Marshal(RotationRequest{
		UserLabel: "alice",
		Nonce:     challenge.Nonce,
		Proof:     RotationProof("new", challenge.Nonce),
	})
	if err != nil {
		t.Fatal(err)
	}
	ackResult := make(chan error, 1)
	go func() {
		response, requestErr := http.Post(
			"http://"+listener.Addr().String()+PathCredentialsAck,
			"application/json",
			bytes.NewReader(body),
		)
		if response != nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
		}
		ackResult <- requestErr
	}()
	<-ackStarted
	time.Sleep(80 * time.Millisecond)
	close(ackRelease)
	if err := <-ackResult; err == nil {
		t.Fatal("credential acknowledgement response unexpectedly survived WriteTimeout")
	}
	assertOperationFailure(t, reported, "acknowledge credential", ackErr)

	connection, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintf(connection, "GET /control/v1/ha/status HTTP/1.1\r\nHost: test\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	<-reloadStarted
	_ = connection.Close()
	close(reloadRelease)
	assertOperationFailure(t, reported, "reload HA state", reloadErr)
}

type operationFailure struct {
	operation string
	err       error
}

func requestRotationChallenge(t *testing.T, address string) RotationChallenge {
	t.Helper()
	body := bytes.NewBufferString(`{"user_label":"alice","action":"challenge"}`)
	response, err := http.Post("http://"+address+PathCredentialsRotate, "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("challenge status = %d", response.StatusCode)
	}
	var challenge RotationChallenge
	if err := json.NewDecoder(response.Body).Decode(&challenge); err != nil {
		t.Fatal(err)
	}
	return challenge
}

func assertOperationFailure(t *testing.T, failures <-chan operationFailure, operation string, want error) {
	t.Helper()
	select {
	case failure := <-failures:
		if failure.operation != operation || !errors.Is(failure.err, want) {
			t.Fatalf("reported failure = %#v, want operation %q and error %v", failure, operation, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("%s failure was not reported", operation)
	}
}
