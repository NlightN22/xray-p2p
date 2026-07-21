package subscription

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
)

const AdapterURIList3XUIV2811 = "3x-ui-uri-list-v2.8.11"

type URIListDecoder struct {
	MaxOffers    int
	MaxLineBytes int
}

func (d URIListDecoder) Decode(raw RawSnapshot) (Snapshot, error) {
	data, err := decodeURIList(raw.Data)
	if err != nil {
		return Snapshot{}, err
	}
	maxOffers := d.MaxOffers
	if maxOffers <= 0 {
		maxOffers = 256
	}
	maxLineBytes := d.MaxLineBytes
	if maxLineBytes <= 0 {
		maxLineBytes = 16 * 1024
	}
	source := raw.Source
	if source.Adapter == "" {
		source.Adapter = AdapterURIList3XUIV2811
	}
	lines := bytes.Split(data, []byte{'\n'})
	offers := make([]ConnectionOffer, 0, len(lines))
	stableIDs := make(map[string]struct{})
	for _, rawLine := range lines {
		line := strings.TrimSpace(strings.TrimSuffix(string(rawLine), "\r"))
		if line == "" {
			continue
		}
		if len(line) > maxLineBytes {
			return Snapshot{}, fmt.Errorf("decode URI-list subscription: line exceeds %d bytes", maxLineBytes)
		}
		if len(offers) >= maxOffers {
			return Snapshot{}, fmt.Errorf("decode URI-list subscription: offer count exceeds %d", maxOffers)
		}
		link, err := tunnel.ParseLink(line)
		if err != nil {
			return Snapshot{}, fmt.Errorf("decode URI-list subscription offer %d: invalid or unsupported connection parameters", len(offers)+1)
		}
		if required := requiredUnknownParameters(link.Unknown); len(required) != 0 {
			return Snapshot{}, fmt.Errorf("decode URI-list subscription offer %d: unsupported parameters", len(offers)+1)
		}
		credential := strings.TrimSpace(tunnel.ActiveCredential(link.User))
		stableID := stableOfferID(source, link.Endpoint)
		if _, exists := stableIDs[stableID]; exists {
			return Snapshot{}, fmt.Errorf("decode URI-list subscription: duplicate offer identity")
		}
		stableIDs[stableID] = struct{}{}
		offers = append(offers, ConnectionOffer{StableID: stableID, Endpoint: link.Endpoint, UserLabel: link.User.UserLabel, Credential: credential})
	}
	if len(offers) == 0 {
		return Snapshot{}, fmt.Errorf("decode URI-list subscription: no offers")
	}
	return Snapshot{Source: source, Revision: raw.Revision, FetchedAt: raw.FetchedAt, Offers: offers}, nil
}

func requiredUnknownParameters(values map[string][]string) map[string][]string {
	required := make(map[string][]string)
	for key, value := range values {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(key)), "x-optional-") {
			continue
		}
		required[key] = value
	}
	return required
}

func decodeURIList(data []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(data)
	if bytes.Contains(trimmed, []byte("://")) {
		return trimmed, nil
	}
	compact := strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, string(trimmed))
	decoded, err := base64.StdEncoding.DecodeString(compact)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(compact)
	}
	if err != nil {
		return nil, fmt.Errorf("decode URI-list subscription encoding: %w", err)
	}
	return bytes.TrimSpace(decoded), nil
}
