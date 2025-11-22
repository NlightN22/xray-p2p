package netutil

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDetectPublicHostMajority(t *testing.T) {
	restore := stubProviders(
		func(context.Context) (string, error) { return "198.51.100.1", nil },
		func(context.Context) (string, error) { return "198.51.100.2", nil },
		func(context.Context) (string, error) { return "198.51.100.1", nil },
	)
	defer restore()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	value, err := DetectPublicHost(ctx)
	if err != nil {
		t.Fatalf("DetectPublicHost failed: %v", err)
	}
	if value != "198.51.100.1" {
		t.Fatalf("unexpected value %s", value)
	}
}

func TestDetectPublicHostSkipsInvalid(t *testing.T) {
	restore := stubProviders(
		func(context.Context) (string, error) { return "not-an-ip", nil },
		func(context.Context) (string, error) { return "2001:db8::1", nil },
		func(context.Context) (string, error) { return "198.51.100.3", nil },
	)
	defer restore()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	value, err := DetectPublicHost(ctx)
	if err != nil {
		t.Fatalf("DetectPublicHost failed: %v", err)
	}
	if value != "198.51.100.3" {
		t.Fatalf("unexpected value %s", value)
	}
}

func TestDetectPublicHostAllProvidersFail(t *testing.T) {
	restore := stubProviders(
		func(context.Context) (string, error) { return "", errors.New("fail") },
		func(context.Context) (string, error) { return "", nil },
	)
	defer restore()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if _, err := DetectPublicHost(ctx); err == nil {
		t.Fatalf("expected failure when no providers return IPv4")
	}
}

func stubProviders(providers ...hostProvider) func() {
	original := getProviders()
	setPublicHostProvidersForTest(providers)
	return func() {
		setPublicHostProvidersForTest(original)
	}
}

func TestHTTPPublicIPHandlesStatusAndBody(t *testing.T) {
	t.Parallel()

	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			http.Error(w, "fail", http.StatusInternalServerError)
			return
		}
		io.WriteString(w, "198.51.100.9\n")
	}))
	t.Cleanup(server.Close)

	provider := httpPublicIP(server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if _, err := provider(ctx); err == nil {
		t.Fatalf("expected error for 500 response")
	}
	value, err := provider(ctx)
	if err != nil {
		t.Fatalf("provider success: %v", err)
	}
	if value != "198.51.100.9" {
		t.Fatalf("unexpected value %s", value)
	}
}

func TestReadFirstLineEOF(t *testing.T) {
	t.Parallel()

	if _, err := readFirstLine(strings.NewReader("")); !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestParseIPv4(t *testing.T) {
	t.Parallel()

	if got := parseIPv4(" 198.51.100.7 "); got != "198.51.100.7" {
		t.Fatalf("unexpected parse result %q", got)
	}
	if got := parseIPv4("not-an-ip"); got != "" {
		t.Fatalf("expected empty result for invalid input, got %q", got)
	}
	if got := parseIPv4("2001:db8::1"); got != "" {
		t.Fatalf("expected IPv6 to be ignored, got %q", got)
	}
}

func TestMajorityValue(t *testing.T) {
	t.Parallel()

	values := []string{"a", "b", "a", "c", "a"}
	if got := majorityValue(values); got != "a" {
		t.Fatalf("expected majority 'a', got %s", got)
	}

	values = []string{"x", "y"}
	if got := majorityValue(values); got != "x" {
		t.Fatalf("expected first item when tied, got %s", got)
	}
}
