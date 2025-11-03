package netutil

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	resolverTimeout = 3 * time.Second
	httpTimeout     = 5 * time.Second
)

type hostProvider func(context.Context) (string, error)

var (
	publicHostProviders = []hostProvider{
		resolvePublicIP("resolver1.opendns.com:53", "myip.opendns.com"),
		resolvePublicIP("1.1.1.1:53", "whoami.cloudflare"),
		httpPublicIP("https://ifconfig.me"),
		httpPublicIP("https://checkip.amazonaws.com"),
	}
	providersMu sync.RWMutex
)

// DetectPublicHost returns the best-effort public IPv4 address observed for the current machine.
// It queries multiple providers and returns the most frequent IPv4 result.
func DetectPublicHost(ctx context.Context) (string, error) {
	providers := getProviders()

	var values []string
	for _, provider := range providers {
		if ctx.Err() != nil {
			break
		}
		value, err := provider(ctx)
		if err != nil {
			continue
		}
		ip := parseIPv4(value)
		if ip == "" {
			continue
		}
		values = append(values, ip)
	}

	if len(values) == 0 {
		return "", errors.New("netutil: unable to detect public host")
	}

	best := majorityValue(values)
	return best, nil
}

// Resolve public IP using a specific DNS server.
func resolvePublicIP(serverAddr, hostname string) hostProvider {
	return func(ctx context.Context) (string, error) {
		dialer := &net.Dialer{
			Timeout: resolverTimeout,
		}
		resolver := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, serverAddr)
			},
		}
		ips, err := resolver.LookupIPAddr(ctx, hostname)
		if err != nil {
			return "", err
		}
		for _, ip := range ips {
			if v4 := ip.IP.To4(); v4 != nil {
				return v4.String(), nil
			}
		}
		return "", errors.New("netutil: no IPv4 address returned")
	}
}

// Query public IP using HTTP GET.
func httpPublicIP(url string) hostProvider {
	client := &http.Client{
		Timeout: httpTimeout,
	}

	return func(ctx context.Context) (string, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}

		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", errors.New("netutil: unexpected status code")
		}

		body, err := readFirstLine(resp.Body)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(body), nil
	}
}

func readFirstLine(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	if scanner.Scan() {
		return scanner.Text(), nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", io.EOF
}

func parseIPv4(value string) string {
	ip := net.ParseIP(strings.TrimSpace(value))
	if ip == nil {
		return ""
	}
	v4 := ip.To4()
	if v4 == nil {
		return ""
	}
	return v4.String()
}

func majorityValue(values []string) string {
	counts := make(map[string]int, len(values))
	best := values[0]
	bestCount := 0
	for _, value := range values {
		counts[value]++
		if counts[value] > bestCount {
			best = value
			bestCount = counts[value]
		}
	}
	return best
}

func getProviders() []hostProvider {
	providersMu.RLock()
	defer providersMu.RUnlock()
	result := make([]hostProvider, len(publicHostProviders))
	copy(result, publicHostProviders)
	return result
}

// setPublicHostProvidersForTest overrides the detection providers. It is intended for tests.
func setPublicHostProvidersForTest(providers []hostProvider) {
	providersMu.Lock()
	defer providersMu.Unlock()
	publicHostProviders = providers
}
