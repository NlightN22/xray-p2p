// Package subscription defines application contracts for subscription sources.
package subscription

import (
	"context"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
)

type SourceRef struct {
	ID      string `json:"id" toml:"id"`
	Adapter string `json:"adapter" toml:"adapter"`
}

type RawSnapshot struct {
	Source      SourceRef
	Revision    string
	FetchedAt   time.Time
	ContentType string
	Data        []byte
}

type Snapshot struct {
	Source    SourceRef         `json:"source" toml:"source"`
	Revision  string            `json:"revision,omitempty" toml:"revision,omitempty"`
	FetchedAt time.Time         `json:"fetched_at" toml:"fetched_at"`
	Offers    []ConnectionOffer `json:"offers" toml:"offers"`
}

type ConnectionOffer struct {
	StableID   string            `json:"stable_id" toml:"stable_id"`
	Endpoint   tunnel.Endpoint   `json:"endpoint" toml:"endpoint"`
	UserLabel  string            `json:"user_label,omitempty" toml:"user_label,omitempty"`
	Credential string            `json:"credential" toml:"credential"`
	Metadata   map[string]string `json:"metadata,omitempty" toml:"metadata,omitempty"`
}

type Source interface {
	Fetch(context.Context, string) (RawSnapshot, error)
}

type Decoder interface {
	Decode(RawSnapshot) (Snapshot, error)
}
