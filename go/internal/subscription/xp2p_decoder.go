package subscription

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
)

const AdapterXP2PControl = "xp2p-control-v1"

type XP2PControlDecoder struct {
	Credential string
	UserLabel  string
}

func (d XP2PControlDecoder) Decode(raw RawSnapshot) (Snapshot, error) {
	var value controlplane.Subscription
	if err := json.Unmarshal(raw.Data, &value); err != nil {
		return Snapshot{}, fmt.Errorf("decode XP2P subscription: %w", err)
	}
	credential := strings.TrimSpace(d.Credential)
	if credential == "" {
		return Snapshot{}, fmt.Errorf("decode XP2P subscription: credential is required")
	}
	profileName := tunnel.Profile(strings.TrimSpace(value.Profile))
	if profileName == "" {
		profileName = tunnel.ProfileTrojanTLS
	}
	profile, err := tunnel.DefaultProfile(profileName)
	if err != nil {
		return Snapshot{}, err
	}
	if protocol := strings.TrimSpace(value.Protocol); protocol != "" && !strings.EqualFold(protocol, profile.Protocol) {
		return Snapshot{}, fmt.Errorf("decode XP2P subscription: protocol does not match profile %q", profile.Profile)
	}
	if transport := strings.TrimSpace(value.Transport); transport != "" && !strings.EqualFold(transport, profile.Transport) {
		return Snapshot{}, fmt.Errorf("decode XP2P subscription: transport does not match profile %q", profile.Profile)
	}
	if security := strings.TrimSpace(value.Security); security != "" && !strings.EqualFold(security, profile.Security) {
		return Snapshot{}, fmt.Errorf("decode XP2P subscription: security does not match profile %q", profile.Profile)
	}
	profile.Host = strings.TrimSpace(value.Host)
	profile.Port = value.Port
	profile.ServerName = strings.TrimSpace(value.ServerName)
	profile.TLS.PinnedPeerCertSHA256 = strings.TrimSpace(value.TLS.PinnedPeerCertSHA256)
	profile.TLS.VerifyPeerCertByName = strings.TrimSpace(value.TLS.VerifyPeerCertByName)
	profile.TLS.AllowInsecure = value.TLS.ClientMayAllowInsecure
	if flow := strings.TrimSpace(value.Parameters["flow"]); flow != "" {
		profile.Metadata["flow"] = flow
	}
	profile, err = tunnel.Normalize(profile)
	if err != nil {
		return Snapshot{}, fmt.Errorf("decode XP2P subscription profile: %w", err)
	}
	if profile.Host == "" || profile.Port < 1 || profile.Port > 65535 {
		return Snapshot{}, fmt.Errorf("decode XP2P subscription: invalid endpoint")
	}
	if profile.Protocol == "vless" {
		if err := tunnel.ValidateVLESSCredential(credential); err != nil {
			return Snapshot{}, err
		}
	}
	source := raw.Source
	if source.Adapter == "" {
		source.Adapter = AdapterXP2PControl
	}
	offer := ConnectionOffer{StableID: stableOfferID(source, profile), Endpoint: profile, UserLabel: strings.TrimSpace(d.UserLabel), Credential: credential, Metadata: map[string]string{"generation": value.Generation}}
	return Snapshot{Source: source, Revision: value.Generation, FetchedAt: raw.FetchedAt, Offers: []ConnectionOffer{offer}}, nil
}
