package subscription

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var ErrNotModified = errors.New("subscription snapshot not modified")

type HTTPSource struct {
	SourceRef
	URL                    string
	Client                 *http.Client
	Timeout                time.Duration
	MaxBytes               int64
	MaxRedirects           int
	AllowHTTP              bool
	AllowCrossHostRedirect bool
}

func (s HTTPSource) Fetch(ctx context.Context, knownRevision string) (RawSnapshot, error) {
	endpoint, err := url.Parse(strings.TrimSpace(s.URL))
	if err != nil || endpoint.Hostname() == "" {
		return RawSnapshot{}, errors.New("fetch subscription: invalid URL")
	}
	if endpoint.User != nil {
		return RawSnapshot{}, errors.New("fetch subscription: URL userinfo is not allowed")
	}
	if !strings.EqualFold(endpoint.Scheme, "https") && !(s.AllowHTTP && strings.EqualFold(endpoint.Scheme, "http")) {
		return RawSnapshot{}, errors.New("fetch subscription: HTTPS is required")
	}
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	maxBytes := s.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 4 << 20
	}
	maxRedirects := s.MaxRedirects
	if maxRedirects <= 0 {
		maxRedirects = 3
	}
	base := s.Client
	if base == nil {
		base = http.DefaultClient
	}
	client := *base
	client.Timeout = timeout
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > maxRedirects {
			return errors.New("subscription redirect limit exceeded")
		}
		if !strings.EqualFold(req.URL.Scheme, "https") && !(s.AllowHTTP && strings.EqualFold(req.URL.Scheme, "http")) {
			return errors.New("subscription redirect target must use HTTPS")
		}
		if req.URL.User != nil {
			return errors.New("subscription redirect target userinfo is not allowed")
		}
		if !s.AllowCrossHostRedirect && !strings.EqualFold(req.URL.Hostname(), endpoint.Hostname()) {
			return errors.New("subscription cross-host redirect is not allowed")
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return RawSnapshot{}, errors.New("fetch subscription: create request")
	}
	if revision := strings.TrimSpace(knownRevision); revision != "" {
		req.Header.Set("If-None-Match", revision)
	}
	resp, err := client.Do(req)
	if err != nil {
		return RawSnapshot{}, errors.New("fetch subscription: request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		return RawSnapshot{}, ErrNotModified
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return RawSnapshot{}, fmt.Errorf("fetch subscription: server returned status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return RawSnapshot{}, errors.New("fetch subscription: read response")
	}
	if int64(len(data)) > maxBytes {
		return RawSnapshot{}, fmt.Errorf("fetch subscription: response exceeds %d bytes", maxBytes)
	}
	revision := strings.TrimSpace(resp.Header.Get("ETag"))
	if revision == "" {
		revision = strings.TrimSpace(resp.Header.Get("Last-Modified"))
	}
	return RawSnapshot{Source: s.SourceRef, Revision: revision, FetchedAt: time.Now().UTC(), ContentType: resp.Header.Get("Content-Type"), Data: data}, nil
}
