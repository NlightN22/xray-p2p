package netutil

import (
	"context"
	"errors"
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
